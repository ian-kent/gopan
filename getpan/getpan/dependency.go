package getpan

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/companieshouse/gopan/gopan"
	"github.com/ian-kent/go-log/log"
	"gopkg.in/yaml.v1"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"
	"path/filepath"
)

type DependencyList struct {
	Dependencies []*Dependency
	Parent       *Module
}

var global_modules = make(map[string]*Module)
var global_installed = make(map[string]*Module)
var global_unique = make(map[string]int)
var global_lock = new(sync.Mutex)
var versionRe = regexp.MustCompile("^([=><!]+)?\\s*([v\\d\\._-]+)$")
var file_lock = new(sync.Mutex)
var file_get = make(map[string]*sync.Mutex)
var install_lock = new(sync.Mutex)
var install_mutex = make(map[string]*sync.Mutex)
var install_semaphore chan int

type Dependency struct {
	Name       string
	Version    string
	Modifier   string
	Module     *Module
	Additional []*Dependency
}

func (d *Dependency) String() string {
	return fmt.Sprintf("%s (%s %s)", d.Name, d.Modifier, d.Version)
}

type Module struct {
	Name      string
	Version   string
	Url       string
	Source    *Source
	Cached    string
	Extracted string
	Dir       string
	Deps      *DependencyList
	Formod    *Module
}

func (m *Module) String() string {
	return fmt.Sprintf("%s (%s) from %s", m.Name, m.Version, m.Source)
}

func (m1 *Module) IsCircular(m2 *Module) bool {
	if m1.Cached == m2.Cached {
		return true
	}
	if m1.Formod != nil {
		return m1.Formod.IsCircular(m2)
	}
	return false
}

func (m *Module) Path() string {
	path := ""
	if m.Formod != nil {
		path = m.Formod.Path() + "->"
	}
	path = path + m.Name + "-" + m.Version
	return path
}

func (d *DependencyList) AddDependency(dep *Dependency) {
	if _, ok := perl_core[dep.Name]; ok {
		log.Trace("Dependency " + dep.Name + " is from perl core")
		return
	}
	if d.Dependencies == nil {
		d.Dependencies = make([]*Dependency, 0)
	}
	d.Dependencies = append(d.Dependencies, dep)
}

func (d *DependencyList) UniqueInstalled() int {
	return len(global_unique)
}

func (d *DependencyList) Install() (int, error) {
	if d == nil {
		log.Debug("No dependencies to install")
		return 0, nil
	}

	n := 0

	if install_semaphore == nil {
		install_semaphore = make(chan int, config.CPUs)
	}

	var wg sync.WaitGroup
	var errorLock sync.Mutex

	errs := make([]string, 0)

	for _, dep := range d.Dependencies {
		log.Debug("Installing dependency: %s", dep)
		wg.Add(1)
		go func(dep *Dependency) {
			defer wg.Done()
			defer func(mod *Module) {
				if mod != nil {
					log.Debug("Resuming installation of %s", mod)
				}
			}(d.Parent)

			_, ok1 := global_installed[dep.Module.Cached]
			_, ok2 := global_installed[dep.Module.Name+"-"+dep.Module.Version]
			if ok1 || ok2 {
				log.Trace("Module is already installed: %s", dep.Module)
				return
			}

			log.Trace("Aquiring install lock for module %s", dep.Module)
			install_lock.Lock()
			if mt, ok := install_mutex[dep.Module.Cached]; ok {
				install_lock.Unlock()
				log.Trace("Waiting on existing installation for %s", dep.Module)
				log.Trace("Path: %s", dep.Module.Path())
				mt.Lock()
				mt.Unlock()
				log.Trace("Existing installation complete for %s", dep.Module)
				return
			}

			log.Trace("Creating new installation lock for module %s", dep.Module)
			install_mutex[dep.Module.Cached] = new(sync.Mutex)
			install_mutex[dep.Module.Cached].Lock()

			//log.Trace("%s:: Sending semaphore", dep.module)
			install_semaphore <- 1
			install_lock.Unlock()

			o, err := dep.Module.Install()
			//log.Trace("%s:: Waiting on semaphore", dep.module)
			<-install_semaphore
			//log.Trace("%s:: Got semaphore", dep.module)

			global_installed[dep.Module.Name+"-"+dep.Module.Version] = dep.Module
			global_installed[dep.Module.Cached] = dep.Module
			global_unique[dep.Module.Name] = 1

			n += o
			if err != nil {
				log.Error("Error installing module: %s", err)
				errorLock.Lock()
				errs = append(errs, dep.Module.String())
				errorLock.Unlock()
			}

			install_lock.Lock()
			install_mutex[dep.Module.Cached].Unlock()
			install_lock.Unlock()

			n++
		}(dep)
	}

	wg.Wait()

	if len(errs) > 0 {
		log.Error("Failed to install dependencies:")
		for _, err := range errs {
			log.Error("=> %s", err)
		}
		return n, errors.New("Failed to install dependencies")
	}

	return n, nil
}

// Resolve dependencies in a dependency list
// Resolves dependencies in order they occured originally
func (d *DependencyList) Resolve() error {
	if d == nil {
		log.Debug("No dependencies to resolve")
		return nil
	}

	log.Debug("Resolving dependencies")

	errs := make([]string, 0)

	for _, dep := range d.Dependencies {
		log.Debug("Resolving module dependency: %s", dep)
		if err := dep.Resolve(d.Parent); err != nil {
			log.Error("Error resolving module dependencies [%s]: %s", dep, err)
			errs = append(errs, dep.String())
			break
		}
	}

	if len(errs) > 0 {
		log.Error("Failed to find dependencies:")
		for _, err := range errs {
			log.Error("=> %s", err)
		}
		return errors.New("Failed to find dependencies")
	}

	return nil
}

// Resolve a dependency (i.e. one module), trying all sources
func (d *Dependency) Resolve(p *Module) error {
	if gm, ok := global_modules[d.Name+"-"+d.Version]; ok {
		log.Trace("Dependency %s already resolved (S1): %s", d, gm)
		d.Module = gm
		return nil
	}

	log.Trace("Resolving dependency: %s", d)

	for _, s := range config.Sources {
		log.Trace("=> Trying source: %s", s)
		m, err := s.Find(d)
		if err != nil {
			log.Trace("=> Error from source: %s", err)
			continue
		}
		if m != nil {
			log.Trace("=> Resolved dependency: %s", m)
			d.Module = m
			break
		}
	}
	if d.Module == nil {
		log.Error("Failed to resolve dependency: %s", d)
		return fmt.Errorf("Dependency not found from any source: %s", d)
	}

	if gm, ok := global_modules[d.Module.Name+"-"+d.Module.Version+"~"+d.Module.Source.URL]; ok {
		log.Trace("Dependency %s already resolved (S2): %s", d, gm)
		d.Module = gm
	} else if gm, ok := global_modules[d.Module.Name]; ok {
		log.Trace("Dependency %s already resolved (S3): %s", d, gm)

		// See if the already resolved version is acceptable
		if !d.MatchesVersion(gm.Version) {
			errstr := fmt.Sprintf("Version conflict in dependency tree: %s => %s", d, gm)
			log.Error(errstr)
			return errors.New(errstr)
		}

		log.Trace("Version %s matches %s", d.Module, gm.Version)

		// TODO See if downloading a new version would be better
		d.Module = gm
	} else {
		log.Debug("Downloading: %s", d.Module)
		if err := d.Module.Download(); err != nil {
			log.Error("Error downloading module %s: %s", d.Module, err)
			return err
		}

		if p != nil {
			if p.IsCircular(d.Module) {
				log.Error("Detected circular dependency %s from module %s", d.Module, p)
				return fmt.Errorf("Detected circular dependency %s from module %s", d.Module, p)
			}
		}

		// module can't exist because of global_lock
		global_modules[d.Module.Name] = d.Module
		global_modules[d.Module.Name+"-"+d.Module.Version] = d.Module
		global_modules[d.Module.Name+"-"+d.Module.Version+"~"+d.Module.Source.URL] = d.Module

		log.Debug("Resolving module dependencies: %s", d.Module)
		d.Module.Deps = &DependencyList{
			Parent:       d.Module,
			Dependencies: make([]*Dependency, 0),
		}

		if d.Additional != nil && len(d.Additional) > 0 {
			log.Trace("Adding cpanfile additional REQS")
			for _, additional := range d.Additional {
				log.Trace("Adding additional dependency from cpanfile: %s", additional)
				d.Module.Deps.AddDependency(additional)
			}
		}

		if err := d.Module.loadDependencies(); err != nil {
			return err
		}
	}

	return nil
}

func (v *Dependency) MatchesVersion(version string) bool {
	dversion := v.Version

	dv := gopan.VersionFromString(dversion)
	mv := gopan.VersionFromString(version)

	valid := false
	switch v.Modifier {
	case "==":
		log.Trace("Matches: %f == %f", mv, dv)
		if mv == dv {
			valid = true
		}
	case "<=":
		log.Trace("Matches: %f <= %f", mv, dv)
		if mv <= dv {
			valid = true
		}
	case ">=":
		log.Trace("Matches: %f >= %f", mv, dv)
		if mv >= dv {
			valid = true
		}
	case ">":
		log.Trace("Matches: %f > %f", mv, dv)
		if mv > dv {
			valid = true
		}
	case "<":
		log.Trace("Matches: %f < %f", mv, dv)
		if mv < dv {
			valid = true
		}
	}
	log.Trace("=> Result: %t", valid)
	return valid
}

func (v *Dependency) Matches(module *Module) bool {
	return v.MatchesVersion(module.Version)
}

func DependencyFromString(name string, dependency string) (*Dependency, error) {

	version := "0.00"
	modifier := ">"

	matches := versionRe.FindStringSubmatch(dependency)

	if len(matches) == 3 {
		if len(matches[1]) > 0 {
			modifier = matches[1]
		}
		if len(matches[2]) > 0 {
			version = matches[2]
			if len(matches[1]) == 0 {
				modifier = ">="
			}
		}
	}

	dep := &Dependency{
		Name:       name,
		Version:    version,
		Modifier:   modifier,
		Additional: make([]*Dependency, 0),
	}
	return dep, nil
}

func MkIndent(d int) string {
	indent := ""
	for i := 0; i < d; i++ {
		indent = indent + "  "
	}
	return indent
}

func (deps *DependencyList) PrintDeps(d int) {
	for _, dep := range deps.Dependencies {
		if dep.Module == nil {
			log.Info(MkIndent(0)+"%s not found", dep.Name)
			continue
		}
		dep.Module.PrintDeps(d + 1)
	}
}

func (m *Module) PrintDeps(d int) {
	log.Info(MkIndent(d)+"%s (%s): %s", m.Name, m.Version, m.Cached)
	if m.Deps != nil {
		m.Deps.PrintDeps(d + 1)
	}
}

func (m *Module) Download() error {
	m.Dir = config.CacheDir + "/" + path.Dir(m.Url)
	p := strings.TrimSuffix(path.Base(m.Url), ".tar.gz") // FIXME
	p = strings.TrimSuffix(p, ".tgz")
	m.Extracted = m.Dir + "/" + p
	m.Cached = config.CacheDir + "/" + m.Url

	log.Trace("Downloading to: %s", m.Dir)
	log.Trace("Cached file: %s", m.Cached)
	log.Trace("Extracting to: %s", m.Extracted)

	log.Trace("Aquiring lock on download: %s", m.Cached)
	file_lock.Lock()
	if mtx, ok := file_get[m.Cached]; ok {
		file_lock.Unlock()
		log.Trace("Waiting for existing download: %s", m.Cached)
		mtx.Lock()
		mtx.Unlock()
		log.Trace("Existing download complete: %s", m.Cached)
		return nil
	} else {
		log.Trace("Creating new lock")
		file_get[m.Cached] = new(sync.Mutex)
		file_get[m.Cached].Lock()
		defer file_get[m.Cached].Unlock()
		file_lock.Unlock()
		log.Trace("Lock aquired: %s", m.Cached)
	}

	if _, err := os.Stat(m.Cached); err != nil {
		os.MkdirAll(m.Dir, 0777)
		out, err := os.Create(m.Cached)
		if err != nil {
			return err
		}

		url := m.Source.URL + "/" + m.Url
		log.Trace("Downloading: %s", url)
		resp, err := http.Get(url)

		if err != nil {
			return err
		}

		if resp.StatusCode != 200 {
			return errors.New("404 not found")
		}

		_, err = io.Copy(out, resp.Body)
		if err != nil {
			return err
		}

		c := exec.Command("tar", "-zxf", m.Cached, "-C", m.Dir)

		var stdout2 bytes.Buffer
		var stderr2 bytes.Buffer

		c.Stderr = &stderr2
		c.Stdout = &stdout2

		if err := c.Start(); err != nil {
			return fmt.Errorf("Error extracting %s (%s): %s", m.Name, m.Version, err)
		}

		if err := c.Wait(); err != nil {
			return fmt.Errorf("Error extracting %s %s: %s\nSTDERR:\n%sSTDOUT:\n%s", m.Name, m.Version, err, stderr2.String(), stdout2.String())
		}

		out.Close()
		resp.Body.Close()

		log.Trace("File extracted to: %s", m.Extracted)
	} else {
		log.Trace("File already cached: %s", m.Cached)
	}

	return nil
}

func (m *Module) loadDependencies() error {
	yml, err := ioutil.ReadFile(m.Extracted + "/META.yml")
	if err != nil {
		// TODO this isnt an error (it shouldnt make build fail)
		log.Error("Error opening META.yml for %s: %s", m.Name, err)
		// return nil to prevent build fail
		return nil
	}

	meta := make(map[interface{}]interface{})
	err = yaml.Unmarshal(yml, &meta)
	if err != nil {
		// TODO this isnt a real error, probably
		log.Error("Error parsing YAML: %s", err)
		// return nil to prevent build fail
		return nil
	}

	if reqs, ok := meta["requires"]; ok {
		log.Debug("Found dependencies for module %s", m.Name)
		switch reqs.(type) {
		case map[interface{}]interface{}:
			for req, ver := range reqs.(map[interface{}]interface{}) {
				v := float64(0)
				switch ver.(type) {
				case string:
					v = gopan.VersionFromString(ver.(string))
				case int:
					v = float64(ver.(int))
				}
				log.Printf("=> %s (%f)", req, v)
				dep, err := DependencyFromString(req.(string), fmt.Sprintf("%f", ver))
				if err != nil {
					log.Error("Error parsing dependency: %s", err)
					continue
				}
				if _, ok := perl_core[dep.Name]; ok {
					log.Trace("Module is from perl core: %s", dep.Name)
					continue
				}
				m.Deps.AddDependency(dep)
			}
		}

		log.Debug("Resolving module dependency list")

		if err := m.Deps.Resolve(); err != nil {
			log.Error("Error resolving dependency list [%s]: %s", m.Name, err)
			return err
		}

		return nil
	}

	// FIXME repeat of block above, just with more nested levels
	if p, ok := meta["prereqs"]; ok {
		if r, ok := p.(map[interface{}]interface{})["runtime"]; ok {
			if reqs, ok := r.(map[interface{}]interface{})["requires"]; ok {
				log.Debug("Found dependencies for module %s", m.Name)
				switch reqs.(type) {
				case map[interface{}]interface{}:
					for req, ver := range reqs.(map[interface{}]interface{}) {
						v := float64(0)
						switch ver.(type) {
						case string:
							v = gopan.VersionFromString(ver.(string))
						case int:
							v = float64(ver.(int))
						}
						log.Printf("=> %s (%f)", req, v)
						dep, err := DependencyFromString(req.(string), fmt.Sprintf("%f", ver))
						if err != nil {
							log.Error("Error parsing dependency: %s", err)
							continue
						}
						if _, ok := perl_core[dep.Name]; ok {
							log.Trace("Module is from perl core: %s", dep.Name)
							continue
						}
						m.Deps.AddDependency(dep)
					}
				}
			}
		}
		if t, ok := p.(map[interface{}]interface{})["test"]; ok {
			if reqs, ok := t.(map[interface{}]interface{})["requires"]; ok {
				log.Debug("Found dependencies for module %s", m.Name)
				switch reqs.(type) {
				case map[interface{}]interface{}:
					for req, ver := range reqs.(map[interface{}]interface{}) {
						v := float64(0)
						switch ver.(type) {
						case string:
							v = gopan.VersionFromString(ver.(string))
						case int:
							v = float64(ver.(int))
						}
						log.Printf("=> %s (%f)", req, v)
						dep, err := DependencyFromString(req.(string), fmt.Sprintf("%f", ver))
						if err != nil {
							log.Error("Error parsing dependency: %s", err)
							continue
						}
						if _, ok := perl_core[dep.Name]; ok {
							log.Trace("Module is from perl core: %s", dep.Name)
							continue
						}
						m.Deps.AddDependency(dep)
					}
				}
			}
		}

		log.Debug("Resolving module dependency list")
		if err := m.Deps.Resolve(); err != nil {
			log.Error("Error resolving dependency list: %s", err)
			return err
		}

		return nil
	}

	log.Debug("No dependencies for module %s", m.Name)
	return nil
}

func (m *Module) getCmd() *exec.Cmd {
	var c *exec.Cmd
	if _, ok := config.Test.Modules[m.Name]; ok || config.Test.Global {
		log.Trace("Executing cpanm install without --notest flag for %s", m.Cached)
		c = exec.Command("cpanm", "-L", config.InstallDir, m.Cached)
	} else {
		log.Trace("Executing cpanm install with --notest flag for %s", m.Cached)
		c = exec.Command("cpanm", "--notest", "-L", config.InstallDir, m.Cached)
	}
	return c
}

func (m *Module) Install() (int, error) {
	log.Debug("Installing module: %s", m)

	n := 0

	if m.Deps != nil {
		log.Trace("Installing module dependencies for %s", m)

		<-install_semaphore
		o, err := m.Deps.Install()
		install_semaphore <- 1

		n += o
		if err != nil {
			log.Error("Error installing module dependencies for %s: %s", m, err)
			return n, err
		}
	}

	var c *exec.Cmd
	var stdout *bytes.Buffer
	var stderr *bytes.Buffer

	cpanm_cache_dir, err := filepath.Abs(config.CacheDir)
	if err != nil {
		log.Error("Failed to get absolute path of gopan cache directory: %s", err)
		return n, err
	}

	os.Setenv("PERL_CPANM_HOME", cpanm_cache_dir)

	done := false
	attempts := 0
	for !done {
		time.Sleep(time.Duration(100) * time.Millisecond)

		c = m.getCmd()
		stdout = new(bytes.Buffer)
		stderr = new(bytes.Buffer)
		c.Stderr = stderr
		c.Stdout = stdout

		// brute force cpanm text file busy errors
		attempts++
		if err := c.Start(); err != nil {
			if attempts > 10 {
				log.Error("Error installing module %s: %s", m, err)
				return n, err
			}
		} else {
			done = true
		}
	}

	if err := c.Wait(); err != nil {
		if !strings.HasPrefix(strings.ToLower(stderr.String()), "plenv: cannot rehash:") && !strings.Contains(strings.ToLower(stderr.String()), "text file busy") &&
			!strings.HasPrefix(strings.ToLower(stdout.String()), "plenv: cannot rehash:") && !strings.Contains(strings.ToLower(stdout.String()), "text file busy") {
			log.Error(m.Name + "-" + m.Version + " failed to install")
			log.Error("Error installing %s %s: %s\nSTDERR:\n%sSTDOUT:\n%s", m.Name, m.Version, err, stderr.String(), stdout.String())
			return n, err
		}
	}

	n++

	log.Printf("Installed " + m.Name + " (" + m.Version + ")")
	return n, nil
}

func flattenForMod(m *Module) string {
	s := m.Url
	if m.Formod != nil {
		s = flattenForMod(m.Formod) + "->" + s
	}
	return s
}

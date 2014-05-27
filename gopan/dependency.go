package main

import (
	"github.com/ian-kent/go-log/log"
	"regexp"
	"strconv"
	"strings"
	"errors"
	"fmt"
	"sync"
	"os"
	"net/http"
	"io"
	"os/exec"
	"bytes"
	"path"
	"io/ioutil"
	"gopkg.in/yaml.v1"
	"time"
)

type DependencyList struct {
	Dependencies []*Dependency
	parent *Module
}

var global_modules = make(map[string]*Module)
var global_installed = make(map[string]*Module)
var versionRe = regexp.MustCompile("^([=><!]+)?\\s*(.*)$")
var file_lock = new(sync.Mutex)
var file_get = make(map[string]*sync.Mutex)
var install_lock = new(sync.Mutex)
var install_mutex = make(map[string]*sync.Mutex)
var install_semaphore chan int

type Dependency struct {
	name     string
	version  string
	modifier string
	module   *Module
	additional []*Dependency
}

func (d *Dependency) String() string {
	return fmt.Sprintf("%s (%s %s)", d.name, d.modifier, d.version)
}

type Module struct {
	name    string
	version string
	url     string
	source  *Source
	cached  string
	extracted string
	dir     string
	deps    *DependencyList
	formod  *Module
}

func (m *Module) String() string {
	return fmt.Sprintf("%s (%s) from %s", m.name, m.version, m.source)
}

func (m1 *Module) IsCircular(m2 *Module) bool {
	if m1.cached == m2.cached {
		return true
	}
	if m1.formod != nil {
		return m1.formod.IsCircular(m2)
	}
	return false
}

func (m *Module) Path() string {
	path := ""
	if m.formod != nil {
		path = m.formod.Path() + "->"
	}
	path = path + m.name + "-" + m.version
	return path
}

func (d *DependencyList) AddDependency(dep *Dependency) {
	if _, ok := perl_core[dep.name]; ok {
		log.Trace("Dependency " + dep.name + " is from perl core")
		return
	}
	if d.Dependencies == nil {
		d.Dependencies = make([]*Dependency, 0)
	}
	d.Dependencies = append(d.Dependencies, dep)
}

func (d *DependencyList) Install() (int, error) {
	if d == nil {
		log.Debug("No dependencies to install")
		return 0, nil
	}

	n := 0

	install_semaphore = make(chan int, config.CPUs)

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
			}(d.parent)
			
			_, ok1 := global_installed[dep.module.cached]
			_, ok2 := global_installed[dep.module.name + "-" + dep.module.version]
			if ok1 || ok2 {
			   	log.Trace("Module is already installed: %s", dep.module)
			   	return
			}

			log.Trace("Aquiring install lock for module %s", dep.module)
			install_lock.Lock()
			if mt, ok := install_mutex[dep.module.cached]; ok {
				install_lock.Unlock()
				log.Trace("Waiting on existing installation for %s", dep.module)
				log.Trace("Path: %s", dep.module.Path())
				mt.Lock()
				mt.Unlock()
				log.Trace("Existing installation complete for %s", dep.module)				
				return
			}

			log.Trace("Creating new installation lock for module %s", dep.module)
			install_mutex[dep.module.cached] = new(sync.Mutex)
			install_mutex[dep.module.cached].Lock()

			//log.Trace("%s:: Sending semaphore", dep.module)
			//install_semaphore <- 1
			install_lock.Unlock()

			o, err := dep.module.Install()
			//log.Trace("%s:: Waiting on semaphore", dep.module)
			//<-install_semaphore
			//log.Trace("%s:: Got semaphore", dep.module)

			global_installed[dep.module.name + "-" + dep.module.version] = dep.module
			global_installed[dep.module.cached] = dep.module

			n += o
			if err != nil {
				log.Error("Error installing module: %s", err)
				errorLock.Lock()
				errs = append(errs, dep.module.String())
				errorLock.Unlock()
			}

			install_mutex[dep.module.cached].Unlock()

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

func (d *DependencyList) Resolve() error {
	if d == nil {
		log.Debug("No dependencies to resolve")
		return nil
	}

	var wg sync.WaitGroup
	semaphore := make(chan int, config.CPUs)
	var errorLock sync.Mutex

	errs := make([]string, 0)

	for _, dep := range d.Dependencies {
		log.Debug("Resolving dependency: %s", dep)
		wg.Add(1)
		go func(dep *Dependency) {
			defer wg.Done()

			semaphore <- 1

			if gm, ok := global_modules[dep.name + "-" + dep.version]; ok {
				log.Trace("Dependency %s already resolved (S1): %s", dep, gm)
				dep.module = gm
				<-semaphore
				return
			}

			log.Debug("Resolving: %s", dep)
			err := dep.Resolve()
			if err != nil {
				log.Error("Error resolving dependency: %s", dep)
				errorLock.Lock()
				errs = append(errs, dep.String())
				errorLock.Unlock()
				<-semaphore
				return
			}

			if gm, ok := global_modules[dep.module.name + "-" + dep.module.version + "~" + dep.module.source.URL]; ok {
				log.Trace("Dependency %s already resolved (S2): %s", dep, dep.module)
				dep.module = gm
				<-semaphore
				return
			}

			log.Debug("Downloading: %s", dep.module)
			err = dep.module.Download()
			if err != nil {
				log.Error("Error downloading module %s: %s", dep.module, err)
				errorLock.Lock()
				errs = append(errs, dep.module.String())
				errorLock.Unlock()
				<-semaphore
				return
			}

			if d.parent != nil {
				if d.parent.IsCircular(dep.module) {
					log.Error("Detected circular dependency %s from module %s", dep.module, d.parent)
					return
				}
			}

			global_modules[dep.module.name + "-" + dep.module.version] = dep.module
			global_modules[dep.module.name + "-" + dep.module.version + "~" + dep.module.source.URL] = dep.module

			log.Debug("Resolving module dependencies: %s", dep.module)
			dep.module.deps = &DependencyList{
				parent: dep.module,
				Dependencies: make([]*Dependency, 0),
			}

			if dep.additional != nil && len(dep.additional) > 0 {
				log.Trace("Adding cpanfile additional REQS")
				for _, additional := range dep.additional {
					log.Trace("Adding additional dependency from cpanfile: %s", additional)
					dep.module.deps.AddDependency(additional)
				}
			}

			err = dep.module.loadDependencies()
			if err != nil {
				log.Error("Error resolving module dependencies: %s", err)
				errorLock.Lock()
				errs = append(errs, dep.module.String())
				errorLock.Unlock()
				<-semaphore
				return
			}

			<-semaphore
		}(dep)
	}

	wg.Wait()

	if len(errs) > 0 {
		log.Error("Failed to find dependencies:")
		for _, err := range errs {
			log.Error("=> %s", err)
		}
		return errors.New("Failed to find dependencies")
	}

	return nil
}

func (v *Dependency) Resolve() error {
	log.Trace("Resolving dependency: %s", v)

	for _, s := range config.Sources {
		log.Trace("=> Trying source: %s", s)
		m, err := s.Find(v)
		if err != nil {
			log.Trace("=> Error from source: %s", err)
			continue
		}
		if m != nil {
			log.Trace("=> Resolved dependency: %s", m)
			v.module = m
			return nil
		}
	}

	return errors.New(fmt.Sprintf("Dependency not found from any source: %s", v))
}

func (v *Dependency) Matches(module *Module) bool {
	dversion := v.version
	if strings.HasPrefix(dversion, "v") {
		dversion = strings.TrimPrefix(dversion, "v")
	}

	mversion := module.version
	if strings.HasPrefix(mversion, "v") {
		mversion = strings.TrimPrefix(mversion, "v")
	}

	dv, _ := strconv.ParseFloat(dversion, 64)
	mv, _ := strconv.ParseFloat(mversion, 64)

	valid := false
	switch v.modifier {
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

func DependencyFromString(name string, dependency string) (*Dependency, error) {	
	matches := versionRe.FindStringSubmatch(dependency)

	if len(matches) == 3 {
		if len(matches[1]) == 0 {
			matches[1] = ">="
		}
		if len(matches[2]) == 0 {
			matches[2] = "0.00"
		}

		dep := &Dependency{
			name:     name,
			version:  matches[2],
			modifier: matches[1],
			additional: make([]*Dependency, 0),
		}
		return dep, nil
	}

	return nil, errors.New(fmt.Sprintf("Unrecognised version string: %s", dependency))
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
		if dep.module == nil {
			log.Info(MkIndent(0) + "%s not found", dep.name)
			continue
		}
		dep.module.PrintDeps(d + 1)
	}
}

func (m *Module) PrintDeps(d int) {
	log.Info(MkIndent(d)+"%s (%s): %s", m.name, m.version, m.cached)
	if m.deps != nil {
		m.deps.PrintDeps(d + 1)
	}
}

func (m *Module) Download() error {
	m.dir = config.CacheDir + "/" + path.Dir(m.url)
	p := strings.TrimSuffix(path.Base(m.url), ".tar.gz") // FIXME
	m.extracted = m.dir + "/" + p
	m.cached = config.CacheDir + "/" + m.url

	log.Trace("Downloading to: %s", m.dir)
	log.Trace("Cached file: %s", m.cached)
	log.Trace("Extracting to: %s", m.extracted)

	log.Trace("Aquiring lock on download: %s", m.cached)
	file_lock.Lock()
	if mtx, ok := file_get[m.cached]; ok {
		file_lock.Unlock()
		log.Trace("Waiting for existing download: %s", m.cached)
		mtx.Lock()
		mtx.Unlock()
		log.Trace("Existing download complete: %s", m.cached)
		return nil
	} else {
		log.Trace("Creating new lock")
		file_get[m.cached] = new(sync.Mutex)
		file_get[m.cached].Lock()
		defer file_get[m.cached].Unlock()
		file_lock.Unlock()
		log.Trace("Lock aquired: %s", m.cached)
	}
	
	if _, err := os.Stat(m.cached); err != nil {
		os.MkdirAll(m.dir, 0777)
		out, err := os.Create(m.cached)
		if err != nil {
			return err
		}

		url := m.source.URL + "/" + m.url
		log.Trace("Downloading: %s", url)
		resp, err := http.Get(url)

		if err != nil {
			return err
		}

		_, err = io.Copy(out, resp.Body)
		if err != nil {
			return err
		}

		c := exec.Command("tar", "-zxf", m.cached, "-C", m.dir)

		var stdout2 bytes.Buffer
		var stderr2 bytes.Buffer

		c.Stderr = &stderr2
		c.Stdout = &stdout2

		if err := c.Start(); err != nil {
			errstr := fmt.Sprintf("Error extracting %s (%s): %s", m.name, m.version, err)
			return errors.New(errstr)
		}

		if err := c.Wait(); err != nil {
			errstr := fmt.Sprintf("Error extracting %s %s: %s\nSTDERR:\n%sSTDOUT:\n%s", m.name, m.version, err, stderr2.String(), stdout2.String())
			return errors.New(errstr)
		}

		out.Close()
		resp.Body.Close()

		log.Trace("File extracted to: %s", m.extracted)
	} else {
		log.Trace("File already cached: %s", m.cached)
	}

	return nil
}

func (m *Module) loadDependencies() error {
	yml, err := ioutil.ReadFile(m.extracted + "/META.yml")
	if err != nil {
		// TODO this isnt an error (it shouldnt make build fail)
		log.Error("Error opening META.yml for %s: %s", m.name, err)
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
		log.Debug("Found dependencies for module %s", m.name)
		switch reqs.(type) {
		case map[interface{}]interface{}:
			for req, ver := range reqs.(map[interface{}]interface{}) {
				v := float64(0)
				switch ver.(type) {
				case string:
					v, _ = strconv.ParseFloat(ver.(string), 64)
				case int:
					v = float64(ver.(int))
				}
				log.Printf("=> %s (%f)", req, v)
				dep, err := DependencyFromString(req.(string), fmt.Sprintf("%f", ver))
				if err != nil {
					log.Error("Error parsing dependency: %s", err)
					continue
				}
				if _, ok := perl_core[dep.name]; ok {
					log.Trace("Module is from perl core: %s", dep.name)
					continue
				}
				m.deps.AddDependency(dep)
			}
		}

		log.Debug("Resolving module dependency list")
		err := m.deps.Resolve()
		if err != nil {
			log.Error("Error resolving dependency list: %s", err)
			return err
		}

		return nil
	}

	log.Debug("No dependencies for module %s", m.name)
	return nil
}

func (m *Module) getCmd() *exec.Cmd {
	var c *exec.Cmd
	if _, ok := config.NoTest.Modules[m.name]; ok || config.NoTest.Global {
		log.Trace("Executing cpanm install with --notest flag for %s", m.cached)
		c = exec.Command("cpanm", "--notest", "-l", "./local", m.cached)
	} else {
		log.Trace("Executing cpanm install for %s", m.cached)
		c = exec.Command("cpanm", "-l", "./local", m.cached)
	}
	return c
}

func (m *Module) Install() (int, error) {
	log.Printf("Installing module: %s", m)

	n := 0

	if m.deps != nil {
		log.Trace("Installing module dependencies for %s", m)
		o, err := m.deps.Install()
		n += o
		if err != nil {
			log.Error("Error installing module dependencies for %s: %s", m, err)
			return n, err
		}
	}

	var c *exec.Cmd
	var stdout *bytes.Buffer
	var stderr *bytes.Buffer

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
			log.Error(m.name + "-" + m.version + " failed to install")
			log.Error("Error installing %s %s: %s\nSTDERR:\n%sSTDOUT:\n%s", m.name, m.version, err, stderr.String(), stdout.String())
			return n, err
		}
	}

	n++

	log.Printf("Installed " + m.name + " (" + m.version + ")")
	return n, nil
}

func flattenForMod(m *Module) string {
	s := m.url
	if m.formod != nil {
		s = flattenForMod(m.formod) + "->" + s
	}
	return s
}

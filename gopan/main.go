package main

import(
	"flag"
	"io/ioutil"
	"strings"
	"regexp"
	"net/http"
	"strconv"
	"os"
	"io"
	"compress/gzip"
	"os/exec"
	"bytes"
	"runtime"
	"sync"
	"gopkg.in/yaml.v1"
	"fmt"
	"github.com/ian-kent/go-log/log"
	"github.com/ian-kent/go-log/layout"
)

// TODO add -backpan so additional backpan indexes can be specified
// TODO add -nocpan -nobackpan options to ignore *.cpan.org
// FIXME different dependency versions of same module all get "installed"
// FIXME install mutex doesn't appear to work properly

var mirrors []string
var notest []string
var notestm map[string]int
var backpan = "http://backpan.cpan.org"
var cpanfile string

var module_download_locks = make(map[string]sync.Mutex, 0)
var module_install_locks = make(map[string]sync.Mutex, 0)

var reqcount int
var depcount int
var lockpending int 

var max int

type Dependency struct {
	name string
	version string
	modifier string
	module *Module
}

type Module struct {
	name string
	version string
	url string
	cached string
	dir string
	deps []*Dependency
	formod *Module
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
		//log.Printf("Matches: %f == %f", dv, mv)
		if mv == dv { valid = true }
	case "<=":
		if mv <= dv { valid = true }
	case ">=":
		if mv >= dv { valid = true }
	case ">":
		if mv > dv { valid = true }
	case "<":
		if mv < dv { valid = true }
	}
	return valid
}

var cpanindex map[string]*Module;
var backpanindex map[string]*Module;

func DependencyFromString(name string, dependency string) *Dependency {
	re := regexp.MustCompile("^([=><!]+)?\\s*(.*)$")
	matches := re.FindStringSubmatch(dependency)
	if len(matches) == 3 {
		if len(matches[1]) == 0 {
			matches[1] = ">="
		}
		if len(matches[2]) == 0 {
			matches[2] = "0.00"
		}

		return &Dependency{
			name: name,
			version: matches[2],
			modifier: matches[1],
		}
	} else {
		log.Printf("Unknown version string: %s", dependency)
		return nil
	}
}

var cpanRe = regexp.MustCompile("^\\s*([^\\s]+)\\s*([^\\s]+)\\s*(.*)$")
var backpanRe = regexp.MustCompile("^authors/id/\\w/\\w{2}/\\w+/([^\\s]+)[-_]v?([\\d\\._\\w]+)(?:-\\w+)?.tar.gz$")

func ModuleFromCPANIndex(mirror string, module string) *Module {
	//log.Printf("Module: module%s\n", module)	
	matches := cpanRe.FindStringSubmatch(module)
	url := "authors/id/" + matches[3]
	version := matches[2]
	if version == "undef" {
		ms := backpanRe.FindStringSubmatch(url)
		if len(ms) == 0 {
			version = "0.00"
		} else {
			version = ms[2]
		}
	}

	vb := strings.Split(version, ".")
	if len(vb) == 2 {
		version = strings.Join(vb[:2], ".")
	} else {
		version = vb[0]
	}
	
	return &Module{
		name: matches[1],
		version: version,
		url: mirror + "/" + url,
	}
}
func ModuleFromBackPANIndex(module string) *Module {
	bits := strings.Split(module, " ")
	path := bits[0]

	if !strings.HasSuffix(path, ".tar.gz") {
		//log.Printf("Skipping: %s\n", path)
		return nil;
	}

	//log.Printf("Found: %s\n", path)
	matches := backpanRe.FindStringSubmatch(path)

	if len(matches) == 0 {
		//log.Printf("FAILED: %s\n", path)
		return nil
	}

	name := strings.Replace(matches[1], "-", "::", -1) // FIXME archive might not match module name
	version := matches[2]
	//log.Printf("BACKPAN: %s (%s) -> %s", name, version, path)

	return &Module{
		name: name,
		version: version,
		url: backpan + "/" + path,
	}
}

func getBackPANIndex() {
	log.Printf("Loading BackPAN index: backpan-index")

	file, err := os.Open("backpan-index")
	if err != nil {
		log.Fatal(err)
	}

	index, err := ioutil.ReadAll(file)
	file.Close()
	if err != nil {
		log.Fatal(err)
	}

	if backpanindex == nil {
		backpanindex = make(map[string]*Module)
	}
	for _, p := range strings.Split(string(index), "\n") {
		if !strings.HasPrefix(p, "authors/id/") {
			continue
		}

		//log.Printf("Parsing: %s\n", p)
		m := ModuleFromBackPANIndex(p)
		if m != nil {
			backpanindex[m.name + "-" + m.version] = m
		}
	}

	log.Printf("Found %d packages", len(backpanindex))
}

func getCPANIndex(mirror string) {
	cpanurl := mirror + "/modules/02packages.details.txt.gz"
	log.Printf("Loading CPAN index: %s", cpanurl)

	res, err := http.Get(cpanurl)
	if err != nil {
		log.Fatal(err)
	}

	r, err := gzip.NewReader(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	packages, err := ioutil.ReadAll(r)
	res.Body.Close()
	if err != nil {
		log.Fatal(err)
	}

	if cpanindex == nil {
		cpanindex = make(map[string]*Module)
	}
	scount := len(cpanindex)
	foundnl := false
	for _, p := range strings.Split(string(packages), "\n") {
		if !foundnl && len(p) == 0 {
			foundnl = true;
			continue;
		}
		if !foundnl || len(p) == 0 {
			continue
		}
		//log.Printf("Parsing: %s\n", p)
		m := ModuleFromCPANIndex(mirror, p)
		cpanindex[m.name] = m
	}
	ecount := len(cpanindex)

	log.Printf("Found %d additional packages (%d total)", ecount - scount, len(cpanindex))
}

func installModule(m *Module, depth int) []string {
	errors := make([]string, 0)

	depth++
	prefix := ""
	for i := 0; i < depth; i++ {
		prefix = prefix + "  "
	}

	//log.Printf(prefix + "Attempting to install module: %s-%s", m.name, m.version)

	if mt, ok := module_install_locks[m.url]; ok {
		fm := flattenForMod(m)
		if strings.Contains(fm, m.name + "-" + m.version + "->") {
			log.Printf(prefix + "Detected circular dependency: %s", fm)
			return errors
		}

		// log.Printf(prefix + "Waiting on install lock for " + fm)
		lockpending++
		mt.Lock()
		mt.Unlock()
		lockpending--
		log.Printf(prefix + m.name + "-" + m.version + " is already installed")
		return errors
	} else {
		var s sync.Mutex
		module_install_locks[m.url] = s
		s.Lock()
		defer s.Unlock()
	}

	log.Printf(prefix + "Installing %s-%s: %s", m.name, m.version, m.url)

	for _, dep := range m.deps {
		if dep.module != nil {
			errs := installModule(dep.module, depth)
			if len(errs) > 0 {
				for _, err := range errs {
					if !strings.HasPrefix(strings.ToLower(err), "plenv: cannot rehash:") && !strings.Contains(strings.ToLower(err), "text file busy") {
						log.Error(prefix + m.name + "-" + m.version + " failed to install")
						errors = append(errors, "Error installing dependency for " + m.name + "-" + m.version + ": " + dep.module.name + "-" + dep.module.version + ": " + err)
					}
				}
			}
			//log.Printf(prefix + "Resuming installation of %s-%s: %s", m.name, m.version, m.url)
		}
	}

	// FIXME we're not finding all dependencies? passing --mirror to cpanm breaks most installs
	var c *exec.Cmd
	if _, ok := notestm[m.name]; ok {
		c = exec.Command("cpanm", "-l", "./local", m.cached, "--notest")
	} else {
		c = exec.Command("cpanm", "-l", "./local", m.cached)
	}	
	
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	c.Stderr = &stderr
	c.Stdout = &stdout

	done := false
	attempts := 0
	for !done {
		// brute force cpanm text file busy errors
		attempts++
		if err := c.Start(); err != nil {
			if attempts > 5 {
				errors = append(errors, fmt.Sprintf("Error installing %s (%s): %s", m.name, m.version, err))
				log.Error(prefix + m.name + "-" + m.version + " failed to install")
				return errors
			}
		} else {
			done = true
		}
	}

	if err := c.Wait(); err != nil {
		if !strings.HasPrefix(strings.ToLower(stderr.String()), "plenv: cannot rehash:") && !strings.Contains(strings.ToLower(stderr.String()), "text file busy") &&
		   !strings.HasPrefix(strings.ToLower(stdout.String()), "plenv: cannot rehash:") && !strings.Contains(strings.ToLower(stdout.String()), "text file busy") {
		   	log.Error(prefix + m.name + "-" + m.version + " failed to install")
			errors = append(errors, fmt.Sprintf("Error installing %s %s: %s\nSTDERR:\n%sSTDOUT:\n%s", m.name, m.version, err, stderr.String(), stdout.String()))
			return errors
		}
	}
	
	log.Printf(prefix + "Installed " + m.name + " " + m.version)
	return errors
}

func flattenForMod(m *Module) string {
	s := m.url
	if m.formod != nil {
		s = flattenForMod(m.formod) + "->" + s
	}
	return s	
}

func downloadModule(m *Module) []string {
	errors := make([]string, 0)

	if _, ok := module_download_locks[m.name + "-" + m.version]; ok {
		// Doesn't matter for downloads (assuming they come from the same *PAN)
		//log.Printf("Waiting on download lock for " + flattenForMod(m))
		//mt.Lock()
		//mt.Unlock()
		return errors
	}

	var s sync.Mutex
	s.Lock()
	defer s.Unlock()
	module_download_locks[m.name + "-" + m.version] = s

	os.MkdirAll(".cpancache", 0777)

	name := strings.Replace(m.name, "::", "-", -1)
	m.cached = ".cpancache/" + name + "-" + m.version + ".tar.gz"

	if _, err := os.Stat(m.cached); err != nil {
		out, err := os.Create(m.cached)
		if err != nil {
			errors = append(errors, err.Error())
			return errors
		}
		defer out.Close()

		resp, err := http.Get(m.url)
		if err != nil {
			errors = append(errors, err.Error())
			return errors
		}
		defer resp.Body.Close();

		_, err = io.Copy(out, resp.Body)
		if err != nil {
			errors = append(errors, err.Error())
			return errors
		}

		c := exec.Command("tar", "-zxf", m.cached, "-C", ".cpancache/")
		
		var stdout2 bytes.Buffer
		var stderr2 bytes.Buffer

		c.Stderr = &stderr2
		c.Stdout = &stdout2

		if err := c.Start(); err != nil {
			errors = append(errors, fmt.Sprintf("Error extracting %s (%s): %s", m.name, m.version, err))
			return errors
		}

		if err := c.Wait(); err != nil {
			errors = append(errors, fmt.Sprintf("Error extracting %s %s: %s\nSTDERR:\n%sSTDOUT:\n%s", m.name, m.version, err, stderr2.String(), stdout2.String()))
			return errors
		}
	}

	m.dir = ".cpancache/" + strings.Replace(m.name, "::", "-", -1) + "-" + m.version

	errors = loadDependencies(m)
	return errors
}

func loadDependencies(m *Module) []string {
	errors := make([]string, 0)

	yml, err := ioutil.ReadFile(m.dir + "/META.yml")
	if err != nil {
		// TODO this isnt an error (it shouldnt make build fail)
		log.Printf("Error opening META.yml for %s: %s", m.name, err)
		return errors
	}

	meta := make(map[interface{}]interface{})
	err = yaml.Unmarshal(yml, &meta)
	if err != nil {
		// TODO this isnt an error (it shouldnt make build fail)
		log.Printf("Error parsing YAML: %s", err)
		return errors
	}

	if reqs, ok := meta["requires"]; ok {
		if m.deps == nil {
			m.deps = make([]*Dependency, 0)
		}
		log.Printf("Found dependencies for module %s", m.name)
		switch reqs.(type) {
		case map[interface{}]interface{}:
			for req, ver := range reqs.(map[interface{}]interface{}) {
				log.Printf("=> %s (%f)", req, ver)
				dep := DependencyFromString(req.(string), fmt.Sprintf("%f", ver))
				m.deps = append(m.deps, dep)
			}
		}

		for _, dep := range m.deps {
			if _, ok := perl_core[dep.name]; ok {
				continue
			}

			if cpandep, ok := cpanindex[dep.name]; ok {		
				if dep.Matches(cpandep) {	
					depcount++			
					log.Printf("  => %s (%s %s) found in CPAN: %s", dep.name, dep.modifier, dep.version, cpandep.url)
					dep.module = cpandep
					dep.module.formod = m
					downloadModule(dep.module)
					continue
				}

				log.Printf("%s (%s) found in CPAN doesn't match requested version '%s %s'", dep.name, cpandep.version, dep.modifier, dep.version)
			} 

			// TODO better versioning (e.g. 3.00 doesn't match 3.0)
			if backpandep, ok := backpanindex[dep.name + "-" + dep.version]; ok {
				depcount++
				log.Printf("  => %s (%s %s) found in BackPAN: %s", dep.name, dep.modifier, dep.version, backpandep.url)
				dep.module = backpandep
				dep.module.formod = m
				downloadModule(dep.module)
				continue
			}

			depcount++
			errors = append(errors, "  => Dependency not found: " + dep.name + " (" + dep.modifier + " " + dep.version + ")")
		}

		return errors
	}

	log.Printf("No dependencies for module %s", m.name)
	return errors
}

func main() {
	log.Logger().Appender().SetLayout(layout.Pattern("[%d] [%p] %m"))
	log.Logger().SetLevel(log.Stol("TRACE"))

	mirrors = make([]string, 0)
	flag.Var((*AppendSliceValue)(&mirrors), "mirror", "A CPAN mirror (can be specified multiple times)")
	notest = make([]string, 0)
	flag.Var((*AppendSliceValue)(&notest), "notest", "A module to install without testing (can be specified multiple times)")
	flag.StringVar(&cpanfile, "cpanfile", "cpanfile", "The cpanfile to install")
	flag.IntVar(&max, "cpus", runtime.NumCPU(), "The number of CPUs to use, defaults to " + strconv.Itoa(runtime.NumCPU()) + " for your environment")
	flag.Parse()

	notestm = make(map[string]int)
	for _, n := range notest {
		notestm[n] = 1
		log.Printf("Skipping tests for: %s", n)
	}

	log.Printf("Using cpanfile: %s", cpanfile)

	bytes, err := ioutil.ReadFile(cpanfile)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Using mirror: http://www.cpan.org")	
	for _, mirror := range mirrors {
		log.Printf("Using mirror: %s", mirror)
	}

	// FIXME putting primary CPAN before mirrors means mirrors take priority
	// might want to index each mirror separately, and try in order specified
	getCPANIndex("http://www.cpan.org")
	for _, mirror := range mirrors {
		getCPANIndex(mirror)
	}
	getBackPANIndex()

	re := regexp.MustCompile("^\\s*requires\\s+['\"]([^'\"]+)['\"](?:,\\s+['\"]([^'\"]+)['\"])?;\\s*(#.*)?")
	deps := make(map[string]*Dependency)
	lines := strings.Split(string(bytes), "\n")

	errors := make([]string, 0)

	for _, l := range lines {
		if len(l) == 0 { continue }

		matches := re.FindStringSubmatch(l)
		module := matches[1]
		
		dependency := DependencyFromString(module, matches[2])

		deps[module] = dependency

		if cpandep, ok := cpanindex[module]; ok {		
			if dependency.Matches(cpandep) {				
				log.Printf("%s (%s %s) found in CPAN: %s", module, dependency.modifier, dependency.version, cpandep.url)
				deps[module].module = cpandep
			} else {
				// log.Printf("%s (%s) found in CPAN doesn't match requested version '%s %s'", module, cpandep.version, dependency.modifier, dependency.version)
			}
		} 

		// TODO better versioning (e.g. 3.00 doesn't match 3.0)
		if backpandep, ok := backpanindex[module + "-" + dependency.version]; ok {
			log.Printf("%s (%s %s) found in BackPAN: %s", module, dependency.modifier, dependency.version, backpandep.url)
			deps[module].module = backpandep
		}

		if deps[module].module != nil && len(matches) >= 3 && strings.HasPrefix(strings.Trim(matches[3], " "), "# REQS: ") {
			comment := matches[3]
			//log.Error("COMMENT: %s", comment)
			comment = strings.TrimPrefix(comment, "# REQS: ")
			comment = strings.Trim(comment, " ")
			new_reqs := strings.Split(comment, ";")
			for _, req := range new_reqs {
				log.Printf("Adding additional dependency: %s", req)
				req = strings.Trim(req, " ")
				bits := strings.Split(req, "-")
				mod := bits[0]
				ver := bits[1]
				new_dep := DependencyFromString(mod, ver)

				dependency.module.deps = append(dependency.module.deps, new_dep)

				if cpandep, ok := cpanindex[mod]; ok {		
					if new_dep.Matches(cpandep) {				
						log.Printf("%s (%s %s) found in CPAN: %s", mod, new_dep.modifier, new_dep.version, cpandep.url)
						new_dep.module = cpandep
						continue
					}

					log.Printf("%s (%s) found in CPAN doesn't match requested version '%s %s'", module, cpandep.version, new_dep.modifier, new_dep.version)
				} 

				// TODO better versioning (e.g. 3.00 doesn't match 3.0)
				if backpandep, ok := backpanindex[mod + "-" + ver]; ok {
					log.Printf("%s (%s %s) found in BackPAN: %s", mod, new_dep.modifier, new_dep.version, backpandep.url)
					new_dep.module = backpandep
					continue
				}

				reqcount++

				errors = append(errors, mod + " (" + new_dep.modifier + " " + new_dep.version + ") not found")
			}
		}

		reqcount++

		if deps[module].module == nil {
			errors = append(errors, module + " (" + dependency.modifier + " " + dependency.version + ") not found")
		}
	}

	if len(errors) > 0 {
		log.Println("Failed to find dependencies:")
		for _, err := range errors {
			log.Println(err)
		}
		return
	}

	log.Printf("Found %d dependencies in cpanfile")

	var wg sync.WaitGroup
	semaphore := make(chan int, max)
	var errorLock sync.Mutex

	log.Printf("Number of parallel downloads/installs: %d", max)
	
	for _, dep := range deps {
		wg.Add(1)		
		go func(dep *Dependency) {
			defer wg.Done()
			if dep.module == nil {
				errorLock.Lock()
				defer errorLock.Unlock()
				errors = append(errors, "No source found for module: " + dep.name + " (" + dep.version + ")")
				return
			}
			semaphore <- 1
			log.Printf("Downloading: %s", dep.module.url)
			errs := downloadModule(dep.module)
			if len(errs) > 0 {
				errorLock.Lock()
				defer errorLock.Unlock()
				for _, r := range errs {
					errors = append(errors, "Error downloading module " + dep.name + ": " + r)
				}
				return
			}
			<-semaphore
		}(dep)
	}
	wg.Wait()

	if len(errors) > 0 {
		log.Printf("Failed to download dependencies (%d found):", depcount)
		for _, err := range errors {
			log.Println(err)
		}
		return
	}

	log.Printf("Found %d additional dependencies (%d total)", depcount, depcount + reqcount)

	for _, dep := range deps {
		wg.Add(1)
		go func(dep *Dependency) {
			defer wg.Done()
			if dep.module == nil {
				errorLock.Lock()
				defer errorLock.Unlock()
				errors = append(errors, "No install for module: " + dep.name)
				return
			}
			semaphore <- 1
			errs := installModule(dep.module, 0)
			if len(errs) > 0 {
				errorLock.Lock()
				errors = append(errors, "Error installing module " + dep.name + ":")
				for _, err := range errs {
					errors = append(errors, "    " + err)
				}
				errorLock.Unlock()
			}
			<-semaphore
		}(dep)
	}
	wg.Wait()

	if len(errors) > 0 {
		log.Println("Failed installation:")
		for _, err := range errors {
			log.Println(err)
		}
		return
	}

	log.Printf("Successfully installed %d modules", depcount + reqcount)
}

package getpan

import (
	"compress/gzip"
	"errors"
	"fmt"
	"github.com/ian-kent/go-log/log"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"
)

// Matches cpan 02packages.details.txt format
var cpanRe = regexp.MustCompile("^\\s*([^\\s]+)\\s*([^\\s]+)\\s*(.*)$")

// Matches gitpan backpan-index format
var backpanRe = regexp.MustCompile("^authors/id/\\w/\\w{2}/\\w+/([^\\s]+)[-_]v?([\\d\\._\\w]+)(?:-\\w+)?.tar.gz$")

type Source struct {
	Type       string
	Index      string
	URL        string
	ModuleList map[string]*Module
}

func NewSource(Type string, Index string, URL string) *Source {
	return &Source{
		Type:       Type,
		Index:      Index,
		URL:        URL,
		ModuleList: make(map[string]*Module),
	}
}

func (s *Source) Find(d *Dependency) (*Module, error) {
	log.Debug("Finding dependency: %s", d)

	switch s.Type {
	case "CPAN":
		log.Debug("=> Using CPAN source")
		if mod, ok := s.ModuleList[d.Name]; ok {
			log.Trace("=> Found in source: %s", mod)
			if d.Matches(mod) {
				log.Trace("=> Version (%s) matches dependency: %s", mod.Version, d)
				return mod, nil
			}
			log.Trace("=> Version (%s) doesn't match dependency: %s", mod.Version, d)
			return nil, nil
		}
	case "BackPAN":
		log.Debug("=> Using BackPAN source")
		// TODO better version matching - new backpan index?
		if mod, ok := s.ModuleList[d.Name+"-"+d.Version]; ok {
			log.Trace("=> Found in source: %s", mod)
			if d.Matches(mod) {
				log.Trace("=> Version (%s) matches dependency: %s", mod.Version, d)
				return mod, nil
			}
			log.Trace("=> Version (%s) doesn't match dependency: %s", mod.Version, d)
			return nil, nil
		}
	default:
		log.Error("Unrecognised source type: %s", s.Type)
		return nil, errors.New(fmt.Sprintf("Unrecognised source: %s", s))
	}
	log.Trace("=> Not found in source")
	return nil, nil
}

func (s *Source) String() string {
	return fmt.Sprintf("%s: %s", s.Type, s.URL)
}

func (s *Source) Load() error {
	log.Debug("Loading source: %s", s)

	switch s.Type {
	case "CPAN":
		log.Debug("=> Got CPAN source")
		return s.loadCPANSource()
	case "BackPAN":
		log.Debug("=> Got BackPAN source")
		return s.loadBackPANSource()
	default:
		log.Error("Unrecognised source type: %s", s.Type)
		return errors.New(fmt.Sprintf("Unrecognised source: %s", s))
	}
}

func (s *Source) loadCPANSource() error {
	log.Info("Loading CPAN index: %s", s.Index)

	res, err := http.Get(s.Index)
	if err != nil {
		log.Warn(err)
		return nil
	}

	// TODO optional gzip
	r, err := gzip.NewReader(res.Body)
	if err != nil {
		log.Warn(err.Error())
		b, _ := ioutil.ReadAll(res.Body)
		log.Info("%s", string(b))
		return nil
	}

	packages, err := ioutil.ReadAll(r)
	res.Body.Close()
	if err != nil {
		log.Warn(err)
		return nil
	}

	foundnl := false
	for _, p := range strings.Split(string(packages), "\n") {
		if !foundnl && len(p) == 0 {
			foundnl = true
			continue
		}
		if !foundnl || len(p) == 0 {
			continue
		}
		m := s.ModuleFromCPANIndex(p)
		s.ModuleList[m.Name] = m
	}

	log.Info("Found %d packages for source: %s", len(s.ModuleList), s)
	return nil
}

func (s *Source) loadBackPANSource() error {
	log.Info("Loading BackPAN index: backpan-index")

	file, err := os.Open("backpan-index")
	if err != nil {
		log.Warn(err.Error())
		return nil
	}

	index, err := ioutil.ReadAll(file)
	file.Close()
	if err != nil {
		log.Fatal(err)
	}

	for _, p := range strings.Split(string(index), "\n") {
		if !strings.HasPrefix(p, "authors/id/") {
			continue
		}

		//log.Printf("Parsing: %s\n", p)
		m := s.ModuleFromBackPANIndex(p)
		if m != nil {
			s.ModuleList[m.Name+"-"+m.Version] = m
		}
	}

	log.Printf("Found %d packages for source: %s", len(s.ModuleList), s)
	return nil
}

func (s *Source) ModuleFromCPANIndex(module string) *Module {
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
		Name:    matches[1],
		Version: version,
		Source:  s,
		Url:     url,
	}
}
func (s *Source) ModuleFromBackPANIndex(module string) *Module {
	bits := strings.Split(module, " ")
	path := bits[0]

	if !strings.HasSuffix(path, ".tar.gz") {
		//log.Printf("Skipping: %s\n", path)
		return nil
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
		Name:    name,
		Version: version,
		Source:  s,
		Url:     path,
	}
}

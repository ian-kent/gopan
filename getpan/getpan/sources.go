package getpan

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"path/filepath"

	"github.com/ian-kent/go-log/log"
)

// Matches cpan 02packages.details.txt format
var cpanRe = regexp.MustCompile("^\\s*([^\\s]+)\\s*([^\\s]+)\\s*(.*)$")

// Matches gitpan backpan-index format
var backpanRe = regexp.MustCompile("^authors/id/\\w/\\w{2}/\\w+/([^\\s]+)[-_]v?([\\d\\._\\w]+)(?:-\\w+)?.tar.gz$")

var sourceRe = regexp.MustCompile("^(\\d+:)?.*")

// Matches 'cpanm --info' string
var metacpanRe = regexp.MustCompile("^(\\w+)/([^\\s]+)[-_]v?([\\d\\._\\w]+)(?:-\\w+)?(.tar.gz|.tgz)$")

type Source struct {
	Type       string
	Index      string
	URL        string
	ModuleList map[string]*Module
	Priority   int
}

// FIXME same structs in both smartpan and getpan
type VersionOutput struct {
	Path    string
	URL     string
	Index   string
	Version float64
}

type WhereOutput struct {
	Module   string
	Latest   float64
	Versions []*VersionOutput
}

func NewSource(Type string, Index string, URL string) *Source {
	priority := 1000

	matches := sourceRe.FindStringSubmatch(URL)

	if len(matches[1]) > 0 {
		i, err := strconv.Atoi(strings.TrimSuffix(matches[1], ":"))
		if err != nil {
			log.Fatal(err)
		}
		priority = i
		URL = strings.TrimPrefix(URL, matches[1])
	}

	return &Source{
		Priority:   priority,
		Type:       Type,
		Index:      Index,
		URL:        URL,
		ModuleList: make(map[string]*Module),
	}
}

func NewMetaSource(Type string, Index string, URL string, ModuleList map[string]*Module) *Source {
	return &Source{
		Type:       Type,
		Index:      Index,
		URL:        URL,
		ModuleList: ModuleList,
	}
}

func (s *Source) Find(d *Dependency) (*Module, error) {
	log.Debug("Finding dependency: %s", d)

	switch s.Type {
	case "SmartPAN":
		log.Debug("=> Using SmartPAN source")

		url := s.URL
		if !strings.HasSuffix(s.URL, "/") {
			url += "/"
		}
		url += "where/" + d.Name + "/" + d.Modifier + d.Version

		log.Info("Query: %s", url)
		res, err := http.Get(url)

		if err != nil {
			log.Error("Error querying SmartPAN: %s", err.Error())
			return nil, err
		}

		defer res.Body.Close()

		body, err := ioutil.ReadAll(res.Body)
		log.Trace("Got response: %s", string(body))

		if res.StatusCode != http.StatusOK {
			log.Info("Module not found in SmartPAN: %s", d.Name)
			return nil, nil
		}

		var v *WhereOutput
		if err = json.Unmarshal(body, &v); err != nil {
			log.Error("Error parsing JSON: %s", err.Error())
			return nil, err
		}

		log.Trace("Found module %s", v.Module)

		if len(v.Versions) == 0 {
			log.Info("Found module but no versions returned")
			return nil, nil
		}

		var lv *VersionOutput
		for _, ver := range v.Versions {
			if ver.Version == v.Latest {
				log.Info("Using latest version of %s: %f", v.Module, ver.Version)
				lv = ver
				break
			}
		}
		if lv == nil {
			log.Info("Couldn't find latest version, selecting first available")
			lv = v.Versions[0]
		}

		return &Module{
			Name:    d.Name,
			Version: fmt.Sprintf("%f", lv.Version),
			Source:  s,
			Url:     lv.URL,
		}, nil
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
	case "MetaCPAN":
		log.Debug("=> Using MetaCPAN source")

		var sout, serr bytes.Buffer
		var cpanm_args string = fmt.Sprintf("-L %s --info %s~\"%s%s\"", config.InstallDir, d.Name, d.Modifier, d.Version)

		cpanm_cache_dir, err := filepath.Abs(config.CacheDir)
		if err != nil {
			log.Error("Failed to get absolute path of gopan cache directory: %s", err)
			return nil, err
		}

		log.Trace("About to exec: cpanm %s", cpanm_args)
		os.Setenv("CPANM_INFO_ARGS", cpanm_args)
		os.Setenv("PERL_CPANM_HOME", cpanm_cache_dir)
		cmd := exec.Command("bash", "-c", `eval cpanm $CPANM_INFO_ARGS`)
		cmd.Stdout = &sout
		cmd.Stderr = &serr

		if err := cmd.Run(); err != nil {
			log.Error("cpanm %s: %s,\n%s\n", cpanm_args, err, serr.String())
			return nil, nil
		}

		if 0 == len(sout.String()) {
			log.Warn("No author/module from cpanm")
			return nil, nil
		}

		author_module := strings.TrimRight(sout.String(), "\n")
		mematches := metacpanRe.FindStringSubmatch(author_module)
		if nil == mematches {
			log.Error("Match failed for: %s", author_module)
			return nil, nil
		}

		log.Trace("Resolved: %s", author_module)
		for _, mesource := range config.MetaSources {

			meurl := fmt.Sprintf("authors/id/%s/%s/%s",
				mematches[1][0:1],
				mematches[1][0:2],
				mematches[0])

			archive_url := fmt.Sprintf("%s/%s", mesource.URL, meurl)

			log.Trace("Checking: " + archive_url)
			resp, err := http.Head(archive_url)
			if err != nil {
				log.Trace(err)
				continue
			}

			log.Trace("HEAD status code: %d", resp.StatusCode)
			if 200 == resp.StatusCode {
				// No module/version check since 'cpanm --info' may resolve to
				// archive and version that may not match source
				return &Module{
					Name:    mematches[2],
					Version: mematches[3],
					Source:  mesource,
					Url:     meurl,
				}, nil
			}

		}
		log.Error("Could not get archive URL via 'cpanm %s'", cpanm_args)
		return nil, nil
	default:
		log.Error("Unrecognised source type: %s", s.Type)
		return nil, errors.New(fmt.Sprintf("Unrecognised source: %s", s))
	}
	log.Trace("=> Not found in source")
	return nil, nil
}

func (s *Source) String() string {
	return fmt.Sprintf("[%d] %s: %s", s.Priority, s.Type, s.URL)
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
	case "SmartPAN":
		log.Debug("=> Got SmartPAN source")
		return nil
	case "MetaCPAN":
		log.Debug("=> Got MetaCPAN source")
		return nil
	default:
		log.Error("Unrecognised source type: %s", s.Type)
		return errors.New(fmt.Sprintf("Unrecognised source: %s", s))
	}
}

func (s *Source) loadCPANSource() error {
	log.Info("Loading CPAN index: %s", s.Index)

	res, err := http.Get(s.Index)
	if err != nil {
		log.Warn("Error loading index: %s", err)
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

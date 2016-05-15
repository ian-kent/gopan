package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"html/template"
	"io/ioutil"
	nethttp "net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/companieshouse/gopan/getpan/getpan"
	"github.com/companieshouse/gopan/gopan"
	gotcha "github.com/companieshouse/gotcha/app"
	"github.com/companieshouse/gotcha/events"
	"github.com/companieshouse/gotcha/http"
	"github.com/ian-kent/go-log/log"
)

var CurrentRelease = "0.9"

type Releases []*Release
type Release struct {
	TagName string `json:"tag_name"`
	URL     string `json:"html_url"`
}

var update_indexes func()
var load_index func(string, string)

func main() {
	configure()

	args := flag.Args()

	if len(args) > 0 && args[0] == "init" {
		log.Info("Initialising SmartPAN")

		log.Info("=> Installing Perl dependencies")

		// FIXME most of this is repeated from getpan/main.go
		cfg := getpan.DefaultConfig()
		cfg.CacheDir = config.CacheDir

		for _, source := range cfg.Sources {
			err := source.Load()
			if err != nil {
				log.Error("Error loading sources: %s", err)
				os.Exit(1)
				return
			}
		}

		deps := &getpan.DependencyList{
			Dependencies: make([]*getpan.Dependency, 0),
		}

		d1, _ := getpan.DependencyFromString("Parse::LocalDistribution", "")
		d2, _ := getpan.DependencyFromString("JSON::XS", "")
		deps.AddDependency(d1)
		deps.AddDependency(d2)

		err := deps.Resolve()
		if err != nil {
			log.Error("Error resolving dependencies: %s", err)
			os.Exit(1)
			return
		}

		_, err = deps.Install()
		if err != nil {
			log.Error("Error installing dependencies: %s", err)
			os.Exit(2)
			return
		}

		log.Info("   - Installed %d modules", deps.UniqueInstalled())

		log.Info("SmartPAN initialisation complete")

		return
	}

	if config.TestDeps {
		perldeps := gopan.TestPerlDeps()
		perldeps.Dump()
		if !perldeps.Ok {
			log.Error("Required perl dependencies are missing")
			os.Exit(1)
			return
		}
	}

	if len(args) > 0 && args[0] == "import" {
		if len(args) < 4 {
			log.Error("Invalid arguments, expecting: smartpan import FILE AUTHORID INDEX")
			return
		}

		fname := args[1]
		log.Info("Importing module from %s", fname)
		log.Info("Author ID: %s", args[2])
		log.Info("Index    : %s", args[3])

		extraParams := map[string]string{
			"importinto": args[3],
			"authorid":   args[2],
			"newindex":   "",
			"cpanmirror": "",
			"importurl":  "",
			"fromdir":    "",
		}

		if strings.HasPrefix(fname, "http://") || strings.HasPrefix(fname, "https://") {
			log.Info("URL: %s", fname)

			extraParams["importurl"] = fname

			request, err := newFormPostRequest(config.RemoteHost+"/import?stream=y", extraParams)
			if err != nil {
				log.Error("Create request error: %s", err.Error())
				return
			}

			client := &nethttp.Client{}
			resp, err := client.Do(request)

			if err != nil {
				log.Error("Error connecting to host: %s", err.Error())
				return
			} else {
				// TODO stream this
				body := &bytes.Buffer{}
				_, err := body.ReadFrom(resp.Body)
				if err != nil {
					log.Error("Error reading response: %s", err.Error())
					return
				}
				resp.Body.Close()
				//log.Info("%d", resp.StatusCode)
				//log.Info("%s", resp.Header)
				log.Info("%s", body.String())
			}
		} else {
			fname = strings.TrimPrefix(fname, "file://")
			log.Info("File: %s", fname)

			if _, err := os.Stat(fname); err != nil {
				log.Error("File not found: %s", err.Error())
				return
			}

			request, err := newfileUploadRequest(config.RemoteHost+"/import?stream=y", extraParams, "fromfile", fname)
			if err != nil {
				log.Error("Create upload error: %s", err.Error())
				return
			}

			client := &nethttp.Client{}
			resp, err := client.Do(request)
			if err != nil {
				log.Error("Error connecting to host: %s", err.Error())
				return
			} else {
				// TODO stream this
				body := &bytes.Buffer{}
				_, err := body.ReadFrom(resp.Body)
				if err != nil {
					log.Error("Error reading response: %s", err.Error())
					return
				}
				resp.Body.Close()
				//log.Info("%d", resp.StatusCode)
				//log.Info("%s", resp.Header)
				log.Info("%s", body.String())
			}
		}

		return
	}

	config.CurrentRelease = CurrentRelease

	var wg sync.WaitGroup

	load_index = func(index string, file string) {
		indexes[index] = gopan.LoadIndex(file)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		indexes = make(map[string]map[string]*gopan.Source)

		// Load CPAN index
		if fi, err := os.Stat(config.CacheDir + "/" + config.CPANIndex); err == nil {
			config.HasCPANIndex = true
			config.CPANIndexDate = fi.ModTime().String()
			config.CPANStatus = "Loading"
			wg.Add(1)
			go func() {
				defer wg.Done()
				load_index(config.CPANIndex, config.CacheDir+"/"+config.CPANIndex)
				config.CPANStatus = "Loaded"
			}()
		}

		// Load BackPAN index
		if fi, err := os.Stat(config.CacheDir + "/" + config.BackPANIndex); err == nil {
			config.HasBackPANIndex = true
			config.BackPANIndexDate = fi.ModTime().String()
			config.BackPANStatus = "Loading"
			wg.Add(1)
			go func() {
				defer wg.Done()
				load_index(config.BackPANIndex, config.CacheDir+"/"+config.BackPANIndex)
				config.BackPANStatus = "Loaded"
			}()
		}

		// Load our secondary indexes
		for _, idx := range config.Indexes {
			wg.Add(1)
			go func() {
				defer wg.Done()
				load_index(idx, config.CacheDir+"/"+idx)
			}()
		}

		// Load our primary index (this is the only index written back to)
		wg.Add(1)
		go func() {
			defer wg.Done()
			load_index(config.Index, config.CacheDir+"/"+config.Index)
		}()
	}()

	update_indexes = func() {
		wg.Wait()
		wg.Add(1)
		go func() {
			wg.Wait()
			config.ImportAvailable = true

			nsrc, nauth, npkg, nprov := gopan.CountIndex(indexes)
			// TODO should probably be in the index - needs to udpate when index changes
			summary = &Summary{nsrc, nauth, npkg, nprov}

			// Do this now so changing the level doesn't interfere with index load
			log.Logger().SetLevel(log.Stol(config.LogLevel))
		}()
		defer wg.Done()
		// Create in-memory indexes for UI/search etc
		for fname, _ := range indexes {
			for idn, idx := range indexes[fname] {
				mapped[idx.Name] = make(map[string]map[string]map[string]*gopan.Author)
				for _, auth := range idx.Authors {
					// author name
					if _, ok := mapped[idx.Name][auth.Name[:1]]; !ok {
						mapped[idx.Name][auth.Name[:1]] = make(map[string]map[string]*gopan.Author)
					}
					if _, ok := mapped[idx.Name][auth.Name[:1]][auth.Name[:2]]; !ok {
						mapped[idx.Name][auth.Name[:1]][auth.Name[:2]] = make(map[string]*gopan.Author)
					}
					mapped[idx.Name][auth.Name[:1]][auth.Name[:2]][auth.Name] = auth

					// wildcards
					if _, ok := mapped[idx.Name]["*"]; !ok {
						mapped[idx.Name]["*"] = make(map[string]map[string]*gopan.Author)
					}
					if _, ok := mapped[idx.Name]["*"]["**"]; !ok {
						mapped[idx.Name]["*"]["**"] = make(map[string]*gopan.Author)
					}
					mapped[idx.Name]["*"]["**"][auth.Name] = auth

					// combos
					if _, ok := mapped[idx.Name][auth.Name[:1]]["**"]; !ok {
						mapped[idx.Name][auth.Name[:1]]["**"] = make(map[string]*gopan.Author)
					}
					if _, ok := mapped[idx.Name]["*"][auth.Name[:2]]; !ok {
						mapped[idx.Name]["*"][auth.Name[:2]] = make(map[string]*gopan.Author)
					}
					mapped[idx.Name][auth.Name[:1]]["**"][auth.Name] = auth
					mapped[idx.Name]["*"][auth.Name[:2]][auth.Name] = auth

					for _, pkg := range auth.Packages {
						filemap[pkg.AuthorURL()] = idn
						for _, prov := range pkg.Provides {
							parts := strings.Split(prov.Name, "::")
							log.Trace("PACKAGE: %s", prov.Name)

							if _, ok := packages[parts[0]]; !ok {
								packages[parts[0]] = &PkgSpace{
									Namespace: parts[0],
									Packages:  make([]*gopan.PerlPackage, 0),
									Children:  make(map[string]*PkgSpace),
									Parent:    nil,
									Versions:  make(map[float64]*gopan.PerlPackage),
								}
							}
							if _, ok := idxpackages[idx.Name]; !ok {
								idxpackages[idx.Name] = make(map[string]*PkgSpace)
							}
							if _, ok := idxpackages[idx.Name][parts[0]]; !ok {
								idxpackages[idx.Name][parts[0]] = &PkgSpace{
									Namespace: parts[0],
									Packages:  make([]*gopan.PerlPackage, 0),
									Children:  make(map[string]*PkgSpace),
									Parent:    nil,
									Versions:  make(map[float64]*gopan.PerlPackage),
								}
							}
							if len(parts) == 1 {
								packages[parts[0]].Packages = append(packages[parts[0]].Packages, prov)
								packages[parts[0]].Versions[gopan.VersionFromString(prov.Version)] = prov
								idxpackages[idx.Name][parts[0]].Packages = append(idxpackages[idx.Name][parts[0]].Packages, prov)
								idxpackages[idx.Name][parts[0]].Versions[gopan.VersionFromString(prov.Version)] = prov
								log.Trace("Version linked: %f for %s", gopan.VersionFromString(prov.Version), prov.Name)
							} else {
								packages[parts[0]].Populate(parts[1:], prov)
								idxpackages[idx.Name][parts[0]].Populate(parts[1:], prov)
							}
						}
					}
				}
			}
		}
	}
	go update_indexes()

	// Get latest SmartPAN version
	go func() {
		res, err := nethttp.Get("https://api.github.com/repos/companieshouse/gopan/releases")
		if err != nil {
			log.Error("Error getting latest version: %s", err.Error())
			return
		}
		defer res.Body.Close()
		b, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Error("Error reading stream: %s", err.Error())
			return
		}

		var r Releases
		err = json.Unmarshal(b, &r)
		if err != nil {
			log.Error("Error unmarshalling JSON: %s", err.Error())
			return
		}

		log.Info("Current release: %s", config.CurrentRelease)
		rel := strings.TrimPrefix(r[0].TagName, "v")
		log.Info("Latest release: %s", rel)
		config.LatestRelease = rel
		config.UpdateURL = r[0].URL

		if config.CurrentRelease < rel {
			config.CanUpdate = true
			log.Info("Your version of SmartPAN can be updated.")
		}
	}()

	// Create our Gotcha application
	var app = gotcha.Create(Asset)
	app.Config.Listen = config.Bind

	summary = &Summary{0, 0, 0, 0}

	app.On(events.BeforeHandler, func(session *http.Session, next func()) {
		session.Stash["summary"] = summary
		session.Stash["config"] = config

		next()
	})

	// Get the router
	r := app.Router

	// Create some routes
	r.Get("/", search)
	r.Post("/", search)

	r.Get("/help", help)
	r.Get("/settings", settings)
	r.Get("/browse", browse)

	r.Get("/import", import1)
	r.Post("/import", import1)

	r.Get("/import/(?P<jobid>[^/]+)", import2)
	r.Get("/import/(?P<jobid>[^/]+)/stream", importstream)

	r.Post("/get-index/(?P<index>(CPAN|BackPAN))/?", getindex)

	// Serve static content (but really use a CDN)
	r.Get("/images/(?P<file>.*)", r.Static("assets/images/{{file}}"))
	r.Get("/css/(?P<file>.*)", r.Static("assets/css/{{file}}"))

	// JSON endpoints
	r.Get("/where/(?P<module>[^/]+)/?", where)
	r.Get("/where/(?P<module>[^/]+)/(?P<version>[^/]+)/?", where)

	// Put these last so they only match /{repo} if nothing else matches
	r.Get("/(?P<repo>[^/]+)/?", browse)
	r.Get("/(?P<repo>[^/]+)/(?P<type>[^/]+)/?", browse)
	r.Get("/(?P<repo>[^/]+)/modules/02packages\\.details\\.txt(?P<gz>\\.gz)?", pkgindex)
	r.Get("/(?P<repo>[^/]+)/authors/id/(?P<file>.*\\.tar\\.gz)", download)
	r.Post("/delete/(?P<repo>[^/]+)/authors/id/(?P<auth1>[^/]+)/(?P<auth2>[^/]+)/(?P<auth3>[^/]+)/(?P<file>.*\\.tar\\.gz)", delete_file)
	r.Get("/(?P<repo>[^/]+)/(?P<type>[^/]+)/(?P<path>.*)/?", browse)

	// Start our application
	app.Start()

	<-make(chan int)
}

func getindex(session *http.Session) {
	idx := session.Stash["index"]

	switch idx {
	case "CPAN":
		go func() {
			config.CPANStatus = "Downloading"

			res, err := nethttp.Get("https://s3-eu-west-1.amazonaws.com/gopan/cpan_index.gz")
			if err != nil {
				log.Error("Error downloading index: %s", err.Error())
				session.RenderException(500, errors.New("Error downloading CPAN index: "+err.Error()))
				config.CPANStatus = "Failed"
				return
			}
			defer res.Body.Close()
			b, err := ioutil.ReadAll(res.Body)
			if err != nil {
				log.Error("Error reading index: %s", err.Error())
				session.RenderException(500, errors.New("Error reading CPAN index: "+err.Error()))
				config.CPANStatus = "Failed"
				return
			}
			fi, err := os.Create(config.CacheDir + "/" + config.CPANIndex)
			if err != nil {
				log.Error("Error creating output file: %s", err.Error())
				session.RenderException(500, errors.New("Error creating output file: "+err.Error()))
				config.CPANStatus = "Failed"
				return
			}
			defer fi.Close()
			fi.Write(b)

			config.CPANStatus = "Downloaded"
			config.HasCPANIndex = true
			config.CPANIndexDate = time.Now().String()

			config.CPANStatus = "Loading"
			load_index(config.CPANIndex, config.CacheDir+"/"+config.CPANIndex)

			config.CPANStatus = "Indexing"
			update_indexes()

			config.CPANStatus = "Loaded"
		}()

		session.Redirect(&url.URL{Path: "/settings"})
		return
	case "BackPAN":
		go func() {
			config.BackPANStatus = "Downloading"

			res, err := nethttp.Get("https://s3-eu-west-1.amazonaws.com/gopan/backpan_index.gz")
			if err != nil {
				log.Error("Error downloading index: %s", err.Error())
				session.RenderException(500, errors.New("Error downloading BackPAN index: "+err.Error()))
				config.BackPANStatus = "Failed"
				return
			}
			defer res.Body.Close()
			b, err := ioutil.ReadAll(res.Body)
			if err != nil {
				log.Error("Error reading index: %s", err.Error())
				session.RenderException(500, errors.New("Error reading BackPAN index: "+err.Error()))
				config.BackPANStatus = "Failed"
				return
			}
			fi, err := os.Create(config.CacheDir + "/" + config.BackPANIndex)
			if err != nil {
				log.Error("Error creating output file: %s", err.Error())
				session.RenderException(500, errors.New("Error creating output file: "+err.Error()))
				config.BackPANStatus = "Failed"
				return
			}
			defer fi.Close()
			fi.Write(b)

			config.BackPANStatus = "Downloaded"
			config.HasBackPANIndex = true
			config.BackPANIndexDate = time.Now().String()

			config.BackPANStatus = "Loading"
			load_index(config.BackPANIndex, config.CacheDir+"/"+config.BackPANIndex)

			config.BackPANStatus = "Indexing"
			update_indexes()

			config.BackPANStatus = "Loaded"
		}()

		session.Redirect(&url.URL{Path: "/settings"})
		return
	}

	session.RenderNotFound()
}

func help(session *http.Session) {
	session.Stash["Title"] = "SmartPAN Help"
	html, _ := session.RenderTemplate("help.html")

	session.Stash["Page"] = "Help"
	session.Stash["Content"] = template.HTML(html)
	session.Render("layout.html")
}

func settings(session *http.Session) {
	session.Stash["Title"] = "SmartPAN Settings"
	html, _ := session.RenderTemplate("settings.html")

	session.Stash["Page"] = "Settings"
	session.Stash["Content"] = template.HTML(html)
	session.Render("layout.html")
}

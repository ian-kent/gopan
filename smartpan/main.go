package main

import (
	"encoding/json"
	"github.com/ian-kent/go-log/log"
	"github.com/ian-kent/gopan/gopan"
	gotcha "github.com/ian-kent/gotcha/app"
	"github.com/ian-kent/gotcha/events"
	"github.com/ian-kent/gotcha/http"
	"html/template"
	"io/ioutil"
	nethttp "net/http"
	"strings"
	"sync"
)

var CurrentRelease = "0.3b"

type Releases []*Release
type Release struct {
	TagName string `json:"tag_name"`
	URL     string `json:"html_url"`
}

func main() {
	configure()

	config.CurrentRelease = CurrentRelease

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()

		// Load our secondary indexes
		indexes = make(map[string]map[string]*gopan.Source)
		for _, idx := range config.Indexes {
			indexes[idx] = gopan.LoadIndex(config.CacheDir + "/" + idx)
		}

		// Load our primary index (this is the only index written back to)
		indexes[config.Index] = gopan.LoadIndex(config.CacheDir + "/" + config.Index)
	}()

	go func() {
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
	}()

	// Get latest SmartPAN version
	go func() {
		res, err := nethttp.Get("https://api.github.com/repos/ian-kent/gopan/releases")
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
	r.Get("/(?P<repo>[^/]+)/(?P<type>[^/]+)/(?P<path>.*)/?", browse)

	// Start our application
	app.Start()

	<-make(chan int)
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

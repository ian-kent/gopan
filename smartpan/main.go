package main

import (
	gotcha "github.com/ian-kent/gotcha/app"
	"github.com/ian-kent/gotcha/events"
	"github.com/ian-kent/gotcha/http"
	"github.com/ian-kent/gotcha/form"
	"github.com/ian-kent/gopan/gopan"
	"github.com/ian-kent/gopan/pandex/pandex"
	"github.com/ian-kent/gopan/getpan/getpan"
	"github.com/ian-kent/go-log/log"
	"html/template"
	"strings"
	"sort"
	"time"
	"errors"
	"crypto/rand" 
    "encoding/base64" 
    "net/url"
    "sync"
    "path/filepath"
    "os"
    "io"
    nethttp "net/http"
    "mime/multipart"
    "io/ioutil"
    "fmt"
    "strconv"
)

var indexes map[string]*gopan.Source
var mapped = make(map[string]map[string]map[string]map[string]*gopan.Author)
var packages = make(map[string]*PkgSpace)
var idxpackages = make(map[string]map[string]*PkgSpace)

type PkgSpace struct {
	Namespace string
	Packages []*gopan.PerlPackage
	Children map[string]*PkgSpace
	Parent   *PkgSpace
}

func (p *PkgSpace) FullName() string {
	s := ""
	if p.Parent != nil {
		s = p.Parent.FullName() + "::"
	}
	s += p.Namespace
	return s
}

func (p *PkgSpace) Version() float64 {
	// FIXME find latest
	if len(p.Packages) == 0 {
		return 0
	}

	return p.Packages[0].Package.Version()
}

func (p *PkgSpace) Populate(parts []string, pkg *gopan.PerlPackage) {
	if len(parts) > 0 {
		if _, ok := p.Children[parts[0]]; !ok {
			p.Children[parts[0]] = &PkgSpace{
				Namespace: parts[0],
				Packages: make([]*gopan.PerlPackage, 0),
				Children: make(map[string]*PkgSpace),
				Parent: p,
			}
		}
		if len(parts) == 1 {
			p.Children[parts[0]].Packages = append(p.Children[parts[0]].Packages, pkg)
		} else {
			p.Children[parts[0]].Populate(parts[1:], pkg)
		}
	}
}

type Summary struct {
	Sources int
	Authors int
	Modules int
	Packages int
}
var summary *Summary

func main() {	
	configure()

	indexes = gopan.LoadIndex(config.CacheDir + "/" + config.Index)
	for _, idx := range indexes {
		mapped[idx.Name] = make(map[string]map[string]map[string]*gopan.Author)
		for _, auth := range idx.Authors {
			if _, ok := mapped[idx.Name][auth.Name[:1]]; !ok {
				mapped[idx.Name][auth.Name[:1]] = make(map[string]map[string]*gopan.Author)
			}
			if _, ok := mapped[idx.Name][auth.Name[:1]][auth.Name[:2]]; !ok {
				mapped[idx.Name][auth.Name[:1]][auth.Name[:2]] = make(map[string]*gopan.Author)
			}
			mapped[idx.Name][auth.Name[:1]][auth.Name[:2]][auth.Name] = auth

			for _, pkg := range auth.Packages {
				for _, prov := range pkg.Provides {
					parts := strings.Split(prov.Name, "::")
					if _, ok := packages[parts[0]]; !ok {
						packages[parts[0]] = &PkgSpace{
							Namespace: parts[0],
							Packages: make([]*gopan.PerlPackage, 0),
							Children: make(map[string]*PkgSpace),
							Parent: nil,
						}
					}
					if _, ok := idxpackages[idx.Name]; !ok {
						idxpackages[idx.Name] = make(map[string]*PkgSpace)
					}
					if _, ok := idxpackages[idx.Name][parts[0]]; !ok {
						idxpackages[idx.Name][parts[0]] = &PkgSpace{
							Namespace: parts[0],
							Packages: make([]*gopan.PerlPackage, 0),
							Children: make(map[string]*PkgSpace),
							Parent: nil,
						}
					}
					if len(parts) == 1 {
						packages[parts[0]].Packages = append(packages[parts[0]].Packages, prov)
						idxpackages[idx.Name][parts[0]].Packages = append(idxpackages[idx.Name][parts[0]].Packages, prov)
					} else {
						packages[parts[0]].Populate(parts[1:], prov)
						idxpackages[idx.Name][parts[0]].Populate(parts[1:], prov)
					}
				}
			}
		}
	}

	log.Logger().SetLevel(log.Stol(config.LogLevel))

	// Create our Gotcha application
	var app = gotcha.Create(Asset)

	nsrc, nauth, npkg, nprov := gopan.CountIndex(indexes)
	// TODO should probably be in the index - needs to udpate when index changes
	summary = &Summary{nsrc, nauth, npkg, nprov}

	app.On(events.BeforeHandler, func(session *http.Session, next func()) {
		session.Stash["summary"] = summary

		next()
	})

	// Get the router
	r := app.Router

	// Create some routes
	r.Get("/", search)
	r.Post("/", search)

	r.Get("/help", help)

	r.Get("/import", import1)
	r.Post("/import", import1)

	r.Get("/import/(?P<jobid>[^/]+)", import2)
	r.Get("/import/(?P<jobid>[^/]+)/stream", importstream)

	r.Get("/browse/(?P<repo>[^/]+)/authors/id/(?P<file>.*\\.tar\\.gz)", download)
	r.Get("/browse/(?P<repo>[^/]+)/modules/02packages\\.details\\.txt(?P<gz>\\.gz)?", pkgindex)

	r.Get("/browse", browse)
	r.Get("/browse/(?P<repo>[^/]+)/?", browse)
	r.Get("/browse/(?P<repo>[^/]+)/(?P<type>[^/]+)/?", browse)
	r.Get("/browse/(?P<repo>[^/]+)/(?P<type>[^/]+)/(?P<path>.*)/?", browse)

	// Serve static content (but really use a CDN)
	r.Get("/images/(?P<file>.*)", r.Static("assets/images/{{file}}"))
	r.Get("/css/(?P<file>.*)", r.Static("assets/css/{{file}}"))

	// Start our application
	app.Start()

	<-make(chan int)
}

type SearchResult struct {
	Name string
	Type string
	URL string
	Obj interface{}
	Glyph string
}

type ByName []*SearchResult
func (a ByName) Len() int { return len(a) }
func (a ByName) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByName) Less(i, j int) bool { return a[i].Name < a[j].Name }

func help(session *http.Session) {
	session.Stash["Title"] = "SmartPAN Help"
	html, _ := session.RenderTemplate("help.html")

	session.Stash["Page"] = "Help"
	session.Stash["Content"] = template.HTML(html)
	session.Render("layout.html")
}

func search(session *http.Session) {
	if q, ok := session.Request.Form()["q"]; ok {
		query := strings.ToLower(q[0])
		session.Stash["Query"] = q[0]
		results := make([]*SearchResult, 0)
		var lock sync.Mutex

		tStart := time.Now().UnixNano()

		var wg sync.WaitGroup

		for _, idx := range indexes {
			wg.Add(1)
			go func(idx *gopan.Source) {
				defer wg.Done()

				if strings.Contains(strings.ToLower(idx.Name), query) {
					lock.Lock()
					results = append(results, &SearchResult{
						Name: idx.Name,
						Type: "Index",
						URL: idx.Name,
						Obj: idx,
						Glyph: "list",
					})
					lock.Unlock()
				}
				
				for _, auth := range idx.Authors {
					wg.Add(1)
					go func(idx *gopan.Source, auth *gopan.Author) {
						defer wg.Done()
						
						if strings.Contains(strings.ToLower(auth.Name), query) {
							lock.Lock()
							results = append(results, &SearchResult{
								Name: auth.Name,
								Type: "Author",
								URL: idx.Name + "/authors/id/" + auth.Name[:1] + "/" + auth.Name[:2] + "/" + auth.Name,
								Obj: auth,
								Glyph: "user",
							})
							lock.Unlock()
						}
						for _, pkg := range auth.Packages {
							if strings.Contains(strings.ToLower(pkg.Name), query) {
								lock.Lock()
								results = append(results, &SearchResult{
									Name: pkg.Name,
									Type: "Module",
									URL: idx.Name + "/authors/id/" + auth.Name[:1] + "/" + auth.Name[:2] + "/" + auth.Name + "/" + pkg.Name,
									Obj: pkg,
									Glyph: "compressed",
								})
								lock.Unlock()
							}
							for _, prov := range pkg.Provides {
								if strings.Contains(strings.ToLower(prov.Name), query) {
									lock.Lock()
									results = append(results, &SearchResult{
										Name: prov.Name,
										Type: "Package",
										URL: idx.Name + "/authors/id/" + auth.Name[:1] + "/" + auth.Name[:2] + "/" + auth.Name + "/" + pkg.Name,
										Obj: prov,
										Glyph: "briefcase",
									})
									lock.Unlock()
								}
							}
						}
					}(idx, auth)
				}
			}(idx)
		}

		wg.Wait()

		t := float64(time.Now().UnixNano()-tStart) / 100000 // ms

		sort.Sort(ByName(results))

		session.Stash["Results"] = results
		session.Stash["Duration"] = t
	}

	session.Stash["Title"] = "SmartPAN"
	html, _ := session.RenderTemplate("search.html")

	session.Stash["Page"] = "Search"
	session.Stash["Content"] = template.HTML(html)
	session.Render("layout.html")
}

// Get top level repo list
func toplevelRepo() map[string]map[string]string {
	dirs := make(map[string]map[string]string, 0)
	for pos, _ := range mapped {
		dirs[pos] = map[string]string{
			"Name": pos,
			"Path": "/browse/" + pos,
		}
	}	
	dirs["SmartPAN"] = map[string]string{
		"Name": "SmartPAN",
		"Path": "/browse/SmartPAN",
	}
	return dirs
}

// modules/authors
func tlRepo1(idx string) (map[string]map[string]string, map[string]map[string]string) {
	dirs := make(map[string]map[string]string, 0)
	files := make(map[string]map[string]string, 0)

	dirs["modules"] = map[string]string{
		"Name": "modules",
		"Path": "/browse/" + idx + "/modules",
	}
	dirs["authors"] = map[string]string{
		"Name": "authors",
		"Path": "/browse/" + idx + "/authors",
	}
	return dirs, files
}

// 02packages/id
func tlRepo2(idx string, tl string) (map[string]map[string]string, map[string]map[string]string) {
	dirs := make(map[string]map[string]string, 0)
	files := make(map[string]map[string]string, 0)

	if tl == "modules" {
		files["02packages.details.txt"] = map[string]string{
			"Name": "02packages.details.txt",
			"Path": "/browse/" + idx + "/modules/02packages.details.txt",
		}
		files["02packages.details.txt.gz"] = map[string]string{
			"Name": "02packages.details.txt.gz",
			"Path": "/browse/" + idx + "/modules/02packages.details.txt.gz",
		}
		for k, _ := range packages {
			files[k] = map[string]string{
				"Name": k,
				"Path": "/browse/" + idx + "/modules/" + k,
			}
		}
	} else if tl == "authors" {
		dirs["id"] = map[string]string{
			"Name": "id",
			"Path": "/browse/" + idx + "/authors/id",
		}
	}
	return dirs, files
}

// Get list of author first letters
func tlAuthor1(idx string) map[string]map[string]string {
	dirs := make(map[string]map[string]string, 0)

	if idx == "SmartPAN" {
		for idx, _ := range mapped {
			for pos, _ := range mapped[idx] {
				dirs[pos] = map[string]string{
					"Name": pos,
					"Path": "/browse/SmartPAN/authors/id/" + pos,
				}
			}
		}
	} else {
		for pos, _ := range mapped[idx] {
			dirs[pos] = map[string]string{
				"Name": pos,
				"Path": "/browse/" + idx + "/authors/id/" + pos,
			}
		}
	}
	return dirs
}

// Get list of author second letters
func tlAuthor2(idx string, fl string) map[string]map[string]string {
	dirs := make(map[string]map[string]string, 0)

	if idx == "SmartPAN" {
		for idx, _ := range mapped {
			for pos, _ := range mapped[idx][fl] {
				dirs[pos] = map[string]string{
					"Name": pos,
					"Path": "/browse/SmartPAN/authors/id/" + fl + "/" + pos,
				}
			}
		}
	} else {
		for pos, _ := range mapped[idx][fl] {
			dirs[pos] = map[string]string{
				"Name": pos,
				"Path": "/browse/" + idx + "/authors/id/" + fl + "/" + pos,
			}
		}
	}
	return dirs
}

// Get list of author second letters
func tlAuthor3(idx string, fl string, sl string) map[string]map[string]string {
	dirs := make(map[string]map[string]string, 0)

	if idx == "SmartPAN" {
		for idx, _ := range mapped {
			for pos, _ := range mapped[idx][fl][sl] {
				dirs[pos] = map[string]string{
					"Name": pos,
					"Path": "/browse/SmartPAN/authors/id/" + fl + "/" + sl + "/" + pos,
				}
			}
		}
	} else {
		for pos, _ := range mapped[idx][fl][sl] {
			dirs[pos] = map[string]string{
				"Name": pos,
				"Path": "/browse/" + idx + "/authors/id/" + fl + "/" + sl + "/" + pos,
			}
		}
	}
	return dirs
}

func tlModuleList(idx string, author string) map[string]map[string]string {
	files := make(map[string]map[string]string, 0)

	if idx == "SmartPAN" {
		for idx, _ := range mapped {
			if auth, ok := mapped[idx][author[:1]][author[:2]][author]; ok {
				for pos, _ := range auth.Packages {
					files[pos] = map[string]string{
						"Name": pos,
						"Path": "/browse/" + idx + "/authors/id/" + author[:1] + "/" + author[:2] + "/" + author + "/" + pos,
					}
				}
			}
		}
	} else {
		for pos, _ := range mapped[idx][author[:1]][author[:2]][author].Packages {
			files[pos] = map[string]string{
				"Name": pos,
				"Path": "/browse/" + idx + "/authors/id/" + author[:1] + "/" + author[:2] + "/" + author + "/" + pos,
			}
		}
	}

	return files
}

func writepkgindex(session *http.Session, pkgspace *PkgSpace) {
	if len(pkgspace.Packages) > 0 {
		latest := pkgspace.Packages[0]
		if len(latest.Version) == 0 {
			latest.Version = "undef"
		}
		session.Response.WriteText(fmt.Sprintf("%-40s %-10s %s\n", pkgspace.FullName(), latest.Version, latest.Package.AuthorURL()))
	}

	if len(pkgspace.Children) > 0 {
		for _, ps := range pkgspace.Children {
			writepkgindex(session, ps)
		}
	}
}

func pkgindex(session *http.Session) {
	if _, ok := session.Stash["repo"]; !ok {
		session.RenderNotFound()
		return
	}

	repo := session.Stash["repo"].(string)

	if _, ok := indexes[repo]; !ok && repo != "SmartPAN" {
		session.RenderNotFound()
		return	
	}

	if g, ok := session.Stash["gz"]; ok {
		if len(g.(string)) > 0 {
			session.Response.Gzip()
			log.Debug("Using gzip")
		}
	}

	session.Response.WriteText("File:         02packages.details.txt\n")
	session.Response.WriteText("Description:  Package names found in directory " + repo + "/authors/id\n")
	session.Response.WriteText("Columns:      package name, version, path\n")
	session.Response.WriteText("Written-By:   SmartPAN (from GoPAN)\n")
	session.Response.WriteText("Line-Count:   " + strconv.Itoa(summary.Packages) + "\n") // FIXME wrong count
	session.Response.WriteText("\n")

	if repo == "SmartPAN" {
		for _, pkg := range packages {
			writepkgindex(session, pkg)
		}
	} else {
		for _, pkg := range idxpackages[repo] {
			writepkgindex(session, pkg)
		}
	}
}

func download(session *http.Session) {
	if _, ok := session.Stash["repo"]; !ok {
		session.RenderNotFound()
		return
	}

	if _, ok := session.Stash["file"]; !ok {
		session.RenderNotFound()
		return
	}

	repo := session.Stash["repo"].(string)
	file := session.Stash["file"].(string)

	log.Debug("Repo [%s], file [%s]", repo, file)

	nfile := ".gopancache/" + repo + "/" + file

	f, err := os.Open(nfile)
	if err != nil {
		log.Error(err.Error())
		session.RenderNotFound()
		return
	}

	defer f.Close()

	b, err := ioutil.ReadAll(f)
	if err != nil {
		log.Error(err.Error())
		session.RenderNotFound()
		return
	}

	session.Response.Write(b)
}

func browse(session *http.Session) {
	session.Stash["Title"] = "SmartPAN"

	path := ""
	repo := ""
	itype := ""
	fpath := ""
	if r, ok := session.Stash["repo"]; ok {
		repo = r.(string)
		fpath += repo + "/"
	}
	if i, ok := session.Stash["type"]; ok {
		itype = i.(string)
		fpath += itype + "/"
	}
	if p, ok := session.Stash["path"]; ok {
		path = p.(string)
		fpath += path + "/"
	}

	fpath = strings.TrimSuffix(fpath, "/")
	session.Stash["path"] = fpath

	bits := strings.Split(path, "/")
	fbits := strings.Split(fpath, "/")
	dirs := make(map[string]map[string]string, 0)
	files := make(map[string]map[string]string, 0)

	log.Info("Path: %s, Bits: %d", path, len(bits))

	if repo == "" {
		dirs = toplevelRepo()
	} else if itype == "" {
		dirs, files = tlRepo1(repo)
	} else {
		switch itype {
		case "authors":
			if len(path) == 0 {
				dirs, files = tlRepo2(repo, itype)
			} else {
				switch len(bits) {
				case 1:
					log.Info("tlAuthor1")
					dirs = tlAuthor1(repo)
				case 2:
					log.Info("tlAuthor2: %s", bits[1])
					dirs = tlAuthor2(repo, bits[1])
				case 3:
					log.Info("tlAuthor3: %s, %s", bits[1], bits[2])
					dirs = tlAuthor3(repo, bits[1], bits[2])
				case 4:
					log.Info("tlModuleList: %s, %s", repo, bits[3])
					files = tlModuleList(repo, bits[3])
				default:
					log.Info("Invalid path - rendering not found")
					session.RenderNotFound()
				}
			}
		case "modules":
			if path == "" {
				dirs, files = tlRepo2(repo, itype)
			} else {
				if repo == "SmartPAN" {
					rbits := bits[1:]
					ctx := packages[bits[0]]
					log.Info("Starting with context: %s", ctx.Namespace)
					for len(rbits) > 0 {
						ctx = ctx.Children[rbits[0]]
						rbits = rbits[1:]
					}
					log.Info("Stashing package context: %s", ctx.Namespace)
					session.Stash["Package"] = ctx
					for ns, _ := range ctx.Children {
						files[ns] = map[string]string{
							"Name": ns,
							"Path": "/browse/" + repo + "/modules/" + path + "/" + ns,
						}
					}
				} else {
					rbits := bits[1:]
					ctx := idxpackages[repo][bits[0]]
					log.Info("Starting with context: %s", ctx.Namespace)
					for len(rbits) > 0 {
						ctx = ctx.Children[rbits[0]]
						rbits = rbits[1:]
					}
					session.Stash["Package"] = ctx
					log.Info("Stashing package context: %s", ctx.Namespace)
					for ns, _ := range ctx.Children {
						files[ns] = map[string]string{
							"Name": ns,
							"Path": "/browse/" + repo + "/modules/" + path + "/" + ns,
						}
					}
				}
			}
		default:
			session.RenderNotFound()
		}
	}

	session.Stash["Dirs"] = dirs
	session.Stash["Files"] = files

	pp := make([]map[string]string, 0)
	cp := ""
	for _, b := range fbits {
		cp = cp + "/" + b
		pp = append(pp, map[string]string{
			"Name": b,
			"Path": cp,
		})
	}
	session.Stash["PathBits"] = pp
	
	html, _ := session.RenderTemplate("browse.html")

	session.Stash["Page"] = "Browse"
	session.Stash["Content"] = template.HTML(html)
	session.Render("layout.html")
}

type ImportForm struct {
	ImportInto string `minlength: 1`
	NewIndex string
	Cpanfile string
	ImportURL string
	FromFile string
	FromDir string
	AuthorID string
	CPANMirror string
}

type ImportJob struct {
	Form *ImportForm
	Complete bool
	Id string
	Watchers []func(string)
	History []string
	Deps *getpan.CPANFile
}
var imports = make(map[string]*ImportJob)

func import1(session *http.Session) {
	session.Stash["indexes"] = indexes
	session.Stash["Title"] = "SmartPAN Import"

	m := &ImportForm{}
	f := form.New(session, m)
	session.Stash["fh"] = f

	if session.Request.Method != "POST" {		
		render_import(session)
		return
	}

	f.Populate(true)
	f.Validate()

	if f.HasErrors {
		render_import(session)
		return
	}

	log.Info("Importing into: %s", m.ImportInto)
	if m.ImportInto == "new_index" {
		if len(m.NewIndex) == 0 {
			f.HasErrors = true
			f.Errors["NewIndex"] = make(map[string]error)
			f.Errors["NewIndex"]["required"] = errors.New("Please give the new repository a name")
			render_import(session)
			return
		}
		log.Info("=> Creating new index: %s", m.NewIndex)
	}

	b := make([]byte, 20)
	rand.Read(b)
	en := base64.URLEncoding
	d := make([]byte, en.EncodedLen(len(b)))
	en.Encode(d, b)

	job := &ImportJob{
		Form: m,
		Complete: false,
		Id: string(d),
		Watchers: make([]func(string), 0),
	}

	if len(m.Cpanfile) > 0 {
		log.Info("Got cpanfile:")
		log.Info(m.Cpanfile)
	}

	log.Info("=> Created import job: %s", job.Id)
	imports[job.Id] = job

	go do_import(session, job)

	//render_import(session)
	session.Redirect(&url.URL{Path: "/import/" + job.Id})
}

func do_import(session *http.Session, job *ImportJob) {
	log.Info("Running import job %s", job.Id)

	reponame := job.Form.ImportInto
	if reponame == "new_index" {
		reponame = job.Form.NewIndex
	}

	msg := func(m string) {
		if m != ":DONE" { 
			job.History = append(job.History, m)
			log.Info(m)
		}
		for _, w := range job.Watchers {
			w(m)
		}
	}

	mods := make([]*getpan.Module, 0)

	// TODO cpanm mirror when using getpan_import

	if len(job.Form.Cpanfile) > 0 {
		msg("Parsing cpanfile input")
		_, modules := getpan_import(job, msg)
		mods = append(mods, modules...)
	}

	if len(job.Form.ImportURL) > 0 {
		msg("Importing from URL: " + job.Form.ImportURL)

		// TODO support cpanfile urls

		nauth := job.Form.AuthorID

		if len(nauth) < 3 {
			// FIXME move to form validation
			msg("Author ID must be at least 3 characters")
			msg(":DONE")
			job.Complete = true
			return;
		}

		npath := ".gopancache/authors/id/" + nauth[:1] + "/" + nauth[:2] + "/" + nauth
		_, fn := filepath.Split(job.Form.ImportURL)
		nfile := npath + "/" + fn

		msg("Caching to " + nfile)

		if _, err := os.Stat(nfile); err != nil {
			os.MkdirAll(npath, 0777)
			out, err := os.Create(nfile)
			if err != nil {
				msg(err.Error())
				msg(":DONE")
				job.Complete = true
				return;
			}

			url := job.Form.ImportURL
			log.Trace("Downloading: %s", url)
			resp, err := nethttp.Get(url)

			if err != nil {
				msg(err.Error())
				msg(":DONE")
				job.Complete = true
				return;
			}

			_, err = io.Copy(out, resp.Body)
			if err != nil {
				msg(err.Error())
				msg(":DONE")
				job.Complete = true
				return;
			}

			out.Close()
			resp.Body.Close()
		} else {
			log.Trace("File already exists in cache: %s", nfile)
		}

		fn = strings.TrimSuffix(fn, ".tar.gz")
		bits := strings.Split(fn, "-")
		name := strings.Join(bits[0:len(bits)-1], "-")
		version := bits[len(bits)-1]

		s := getpan.NewSource("CPAN", "/modules/02packages.details.txt.gz", "")
		m := &getpan.Module{
			Source: s,
			Name: name,
			Version: version,
			Url: "/authors/id/" + nauth[:1] + "/" + nauth[:2] + "/" + nauth + "/" + fn,
			Cached: nfile,
			Dir: npath,
		}
		m.Deps = &getpan.DependencyList{
			Parent: m,
			Dependencies: make([]*getpan.Dependency, 0),
		}
		mods = append(mods, m)
	}

	if len(job.Form.FromDir) > 0 {
		msg("Importing from local directory: " + job.Form.FromDir)

		// TODO support cpanfile paths

		nauth := job.Form.AuthorID

		if len(nauth) < 3 {
			// FIXME move to form validation
			msg("Author ID must be at least 3 characters")
			msg(":DONE")
			job.Complete = true
			return;
		}

		npath := ".gopancache/authors/id/" + nauth[:1] + "/" + nauth[:2] + "/" + nauth
		_, fn := filepath.Split(job.Form.FromDir)
		nfile := npath + "/" + fn

		msg("Caching to " + nfile)

		_, err := CopyFile(nfile, job.Form.FromDir)
		if err != nil {
			msg(err.Error())
			msg(":DONE")
			job.Complete = true
			return;
		}

		fn = strings.TrimSuffix(fn, ".tar.gz")
		bits := strings.Split(fn, "-")
		name := strings.Join(bits[0:len(bits)-1], "-")
		version := bits[len(bits)-1]

		s := getpan.NewSource("CPAN", "/modules/02packages.details.txt.gz", "")
		m := &getpan.Module{
			Source: s,
			Name: name,
			Version: version,
			Url: "/authors/id/" + nauth[:1] + "/" + nauth[:2] + "/" + nauth + "/" + fn,
			Cached: nfile,
			Dir: npath,
		}
		m.Deps = &getpan.DependencyList{
			Parent: m,
			Dependencies: make([]*getpan.Dependency, 0),
		}
		mods = append(mods, m)
	}

	if f, fh, err := session.Request.File("fromfile"); err == nil {	
		fn := fh.Filename

		msg("Importing from uploaded module/cpanfile: " + fn)

		if !strings.HasSuffix(fn, ".tar.gz") && fn != "cpanfile" {
			msg("Only cpanfile and *.tar.gz files are supported")
			msg(":DONE")
			job.Complete = true
			return
		}

		if fn == "cpanfile" {
			msg("Importing cpanfile")
			b, _ := ioutil.ReadAll(f)
			f.Close()
			job.Form.Cpanfile = string(b)
			_, modules := getpan_import(job, msg)
			mods = append(mods, modules...)
		} else {
			msg("Importing .tar.gz")

			nauth := job.Form.AuthorID

			if len(nauth) < 3 {
				// FIXME move to form validation
				msg("Author ID must be at least 3 characters")
				msg(":DONE")
				job.Complete = true
				return;
			}

			npath := ".gopancache/authors/id/" + nauth[:1] + "/" + nauth[:2] + "/" + nauth
			_, fn = filepath.Split(fn)
			nfile := npath + "/" + fn

			msg("Caching to " + nfile)

			_, err := CopyToFile(nfile, f)
			if err != nil {
				msg(err.Error())
				msg(":DONE")
				job.Complete = true
				return;
			}

			fn = strings.TrimSuffix(fn, ".tar.gz")
			bits := strings.Split(fn, "-")
			name := strings.Join(bits[0:len(bits)-1], "-")
			version := bits[len(bits)-1]

			s := getpan.NewSource("CPAN", "/modules/02packages.details.txt.gz", "")
			m := &getpan.Module{
				Source: s,
				Name: name,
				Version: version,
				Url: "/authors/id/" + nauth[:1] + "/" + nauth[:2] + "/" + nauth + "/" + fn,
				Cached: nfile,
				Dir: npath,
			}
			m.Deps = &getpan.DependencyList{
				Parent: m,
				Dependencies: make([]*getpan.Dependency, 0),
			}
			mods = append(mods, m)
		}
	} else {
		msg("Error importing file upload: " + err.Error())
	}

	if len(mods) == 0 {
		msg("Nothing to do")
		msg(":DONE")
		job.Complete = true
		return
	}

	msg("Adding modules to GoPAN index")

	for _, m := range mods {
		msg("=> " + m.Name + " (" + m.Cached + ")")

		dn, fn := filepath.Split(m.Cached)
		dnb := strings.Split(strings.TrimSuffix(dn, string(os.PathSeparator)), string(os.PathSeparator))
		auth := dnb[len(dnb)-1]
		ndir := config.CacheDir + "/" + reponame + "/" + auth[:1] + "/" + auth[:2] + "/" + auth
		npath := ndir + "/" + fn
		
		if _, err := os.Stat(npath); err == nil {
			msg(" | Already exists in repository")
		} else {
			os.MkdirAll(ndir, 0777)

			msg(" | Copying to " + npath)
			_, err := CopyFile(npath, m.Cached)
			if err != nil {
				msg(" ! " + err.Error())
				continue;
			}
		}

		if _, ok := indexes[reponame]; !ok {
			msg(" | Creating index: " + reponame)
			indexes[reponame] = &gopan.Source{
				Name: reponame,
				URL: "/authors/id",
				Authors: make(map[string]*gopan.Author),
			}

			mapped[reponame] = make(map[string]map[string]map[string]*gopan.Author)
		}

		if _, ok := indexes[reponame].Authors[auth]; !ok {
			msg(" | Creating author: " + auth)
			indexes[reponame].Authors[auth] = &gopan.Author{
					Source: indexes[reponame],
					Name: auth,
					Packages: make(map[string]*gopan.Package),
					URL: "/authors/id/" + auth[:1] + "/" + auth[:2] + "/" + auth + "/",
			}
			if _, ok := mapped[reponame][auth[:1]]; !ok {
				mapped[reponame][auth[:1]] = make(map[string]map[string]*gopan.Author)	
			}
			if _, ok := mapped[reponame][auth[:1]][auth[:2]]; !ok {
				mapped[reponame][auth[:1]][auth[:2]] = make(map[string]*gopan.Author)	
			}
			if _, ok := mapped[reponame][auth[:1]][auth[:2]][auth]; !ok {
				mapped[reponame][auth[:1]][auth[:2]][auth] = indexes[reponame].Authors[auth]
			}
		}

		if _, ok := indexes[reponame].Authors[auth].Packages[fn]; !ok {
			msg(" | Creating module: " + fn)
			indexes[reponame].Authors[auth].Packages[fn] = &gopan.Package{
					Author: indexes[reponame].Authors[auth],
					Name: fn,
					URL: indexes[reponame].Authors[auth].URL + fn,
					Provides: make(map[string]*gopan.PerlPackage),
			}

			msg(" | Getting list of packages")
			modnm := strings.TrimSuffix(fn, ".tar.gz")
			pkg := indexes[reponame].Authors[auth].Packages[fn]
			pandex.Provides(pkg, npath, ndir + "/" + modnm, ndir)

			//pkg := indexes[reponame].Authors[auth].Packages[fn]
			msg(" | Adding packages to index")
			if _, ok := idxpackages[reponame]; !ok {
				idxpackages[reponame] = make(map[string]*PkgSpace)
			}
			for _, prov := range pkg.Provides {
				parts := strings.Split(prov.Name, "::")
				if _, ok := packages[parts[0]]; !ok {
					packages[parts[0]] = &PkgSpace{
						Namespace: parts[0],
						Packages: make([]*gopan.PerlPackage, 0),
						Children: make(map[string]*PkgSpace),
						Parent: nil,
					}
				}
				if _, ok := idxpackages[reponame][parts[0]]; !ok {
					idxpackages[reponame][parts[0]] = &PkgSpace{
						Namespace: parts[0],
						Packages: make([]*gopan.PerlPackage, 0),
						Children: make(map[string]*PkgSpace),
						Parent: nil,
					}
				}
				if len(parts) == 1 {
					packages[parts[0]].Packages = append(packages[parts[0]].Packages, prov)
					idxpackages[reponame][parts[0]].Packages = append(idxpackages[reponame][parts[0]].Packages, prov)
				} else {
					packages[parts[0]].Populate(parts[1:], prov)
					idxpackages[reponame][parts[0]].Populate(parts[1:], prov)
				}
			}

			msg(" | Writing to index file")
			gopan.AppendToIndex(config.CacheDir + "/" + config.Index, indexes[reponame], indexes[reponame].Authors[auth], indexes[reponame].Authors[auth].Packages[fn])
		}

		msg(" | Imported module")
	}

	nsrc, nauth, npkg, nprov := gopan.CountIndex(indexes)
	// TODO should probably be in the index - needs to udpate when index changes
	summary = &Summary{nsrc, nauth, npkg, nprov}

	msg(":DONE")
	job.Complete = true
}

func CopyToFile(dstName string, file multipart.File) (written int64, err error) {
    dst, err := os.Create(dstName)
    if err != nil {
        return
    }

    written, err = io.Copy(dst, file)

    dst.Close()
    return
}

func CopyFile(dstName, srcName string) (written int64, err error) {
    src, err := os.Open(srcName)
    if err != nil {
        return
    }

    dst, err := os.Create(dstName)
    if err != nil {
        return
    }

    written, err = io.Copy(dst, src)
    dst.Close()
    src.Close()
    return
}

func render_import(session *http.Session) {
	html, _ := session.RenderTemplate("import.html")
	session.Stash["Page"] = "Import"
	session.Stash["Content"] = template.HTML(html)
	session.Render("layout.html")
}

func import2(session *http.Session) {
	
	session.Stash["Page"] = "Import"
	session.Stash["Title"] = "Import job " + session.Stash["jobid"].(string)

	job := imports[session.Stash["jobid"].(string)]
	session.Stash["Job"] = job

	html, _ := session.RenderTemplate("import2.html")
	session.Stash["Content"] = template.HTML(html)

	session.Render("layout.html")
}

func importstream(session *http.Session) {
	job := imports[session.Stash["jobid"].(string)]
	session.Stash["Job"] = job
	c := session.Response.Chunked()

	session.Response.Headers.Add("Content-Type", "text/plain")

	header, _ := session.RenderTemplate("layout_streamstart.html")
	c <- []byte(header)

	for _, s := range job.History {
		c <- []byte(s + "<br />\n")
	}

	var wg sync.WaitGroup
	if !job.Complete {
		wg.Add(1)
		job.Watchers = append(job.Watchers, func(m string) {
			if m == ":DONE" {
				wg.Done()
				return
			}
			c <- []byte(m + "<br />\n")
		});
	}

	wg.Wait()



	footer, _ := session.RenderTemplate("layout_streamend.html")
	c <- []byte(footer)

	c <- make([]byte, 0)
}
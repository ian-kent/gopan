package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"github.com/ian-kent/go-log/log"
	"github.com/ian-kent/gopan/getpan/getpan"
	"github.com/ian-kent/gopan/gopan"
	"github.com/ian-kent/gopan/pandex/pandex"
	"github.com/ian-kent/gotcha/form"
	"github.com/ian-kent/gotcha/http"
	"html/template"
	"io"
	"io/ioutil"
	"mime/multipart"
	nethttp "net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type ImportForm struct {
	ImportInto string `minlength: 1`
	NewIndex   string
	Cpanfile   string
	ImportURL  string
	FromFile   string
	FromDir    string
	AuthorID   string
	CPANMirror string
}

type ImportJob struct {
	Form     *ImportForm
	Complete bool
	Id       string
	Watchers []func(string)
	History  []string
	Deps     *getpan.CPANFile
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
		Form:     m,
		Complete: false,
		Id:       string(d),
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
			return
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
				return
			}

			url := job.Form.ImportURL
			log.Trace("Downloading: %s", url)
			resp, err := nethttp.Get(url)

			if err != nil {
				msg(err.Error())
				msg(":DONE")
				job.Complete = true
				return
			}

			_, err = io.Copy(out, resp.Body)
			if err != nil {
				msg(err.Error())
				msg(":DONE")
				job.Complete = true
				return
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
			Source:  s,
			Name:    name,
			Version: version,
			Url:     "/authors/id/" + nauth[:1] + "/" + nauth[:2] + "/" + nauth + "/" + fn,
			Cached:  nfile,
			Dir:     npath,
		}
		m.Deps = &getpan.DependencyList{
			Parent:       m,
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
			return
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
			return
		}

		fn = strings.TrimSuffix(fn, ".tar.gz")
		bits := strings.Split(fn, "-")
		name := strings.Join(bits[0:len(bits)-1], "-")
		version := bits[len(bits)-1]

		s := getpan.NewSource("CPAN", "/modules/02packages.details.txt.gz", "")
		m := &getpan.Module{
			Source:  s,
			Name:    name,
			Version: version,
			Url:     "/authors/id/" + nauth[:1] + "/" + nauth[:2] + "/" + nauth + "/" + fn,
			Cached:  nfile,
			Dir:     npath,
		}
		m.Deps = &getpan.DependencyList{
			Parent:       m,
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
				return
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
				return
			}

			fn = strings.TrimSuffix(fn, ".tar.gz")
			bits := strings.Split(fn, "-")
			name := strings.Join(bits[0:len(bits)-1], "-")
			version := bits[len(bits)-1]

			s := getpan.NewSource("CPAN", "/modules/02packages.details.txt.gz", "")
			m := &getpan.Module{
				Source:  s,
				Name:    name,
				Version: version,
				Url:     "/authors/id/" + nauth[:1] + "/" + nauth[:2] + "/" + nauth + "/" + fn,
				Cached:  nfile,
				Dir:     npath,
			}
			m.Deps = &getpan.DependencyList{
				Parent:       m,
				Dependencies: make([]*getpan.Dependency, 0),
			}
			mods = append(mods, m)
		}
	} else {
		// there is no file... so no error
		//msg("Error importing file upload: " + err.Error())
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
				continue
			}
		}

		if _, ok := indexes[config.Index][reponame]; !ok {
			msg(" | Creating index: " + reponame)
			indexes[config.Index][reponame] = &gopan.Source{
				Name:    reponame,
				URL:     "/authors/id",
				Authors: make(map[string]*gopan.Author),
			}

			mapped[reponame] = make(map[string]map[string]map[string]*gopan.Author)
		}

		if _, ok := indexes[config.Index][reponame].Authors[auth]; !ok {
			msg(" | Creating author: " + auth)
			author := &gopan.Author{
				Source:   indexes[config.Index][reponame],
				Name:     auth,
				Packages: make(map[string]*gopan.Package),
				URL:      "/authors/id/" + auth[:1] + "/" + auth[:2] + "/" + auth + "/",
			}
			indexes[config.Index][reponame].Authors[auth] = author

			// author name
			if _, ok := mapped[reponame][author.Name[:1]]; !ok {
				mapped[reponame][author.Name[:1]] = make(map[string]map[string]*gopan.Author)
			}
			if _, ok := mapped[reponame][author.Name[:1]][author.Name[:2]]; !ok {
				mapped[reponame][author.Name[:1]][author.Name[:2]] = make(map[string]*gopan.Author)
			}
			mapped[reponame][author.Name[:1]][author.Name[:2]][author.Name] = author

			// wildcards
			if _, ok := mapped[reponame]["*"]; !ok {
				mapped[reponame]["*"] = make(map[string]map[string]*gopan.Author)
			}
			if _, ok := mapped[reponame]["*"]["**"]; !ok {
				mapped[reponame]["*"]["**"] = make(map[string]*gopan.Author)
			}
			mapped[reponame]["*"]["**"][author.Name] = author

			// combos
			if _, ok := mapped[reponame][author.Name[:1]]["**"]; !ok {
				mapped[reponame][author.Name[:1]]["**"] = make(map[string]*gopan.Author)
			}
			if _, ok := mapped[reponame]["*"][author.Name[:2]]; !ok {
				mapped[reponame]["*"][author.Name[:2]] = make(map[string]*gopan.Author)
			}
			mapped[reponame][author.Name[:1]]["**"][author.Name] = author
			mapped[reponame]["*"][author.Name[:2]][author.Name] = author
		}

		if _, ok := indexes[config.Index][reponame].Authors[auth].Packages[fn]; !ok {
			msg(" | Creating module: " + fn)
			indexes[config.Index][reponame].Authors[auth].Packages[fn] = &gopan.Package{
				Author:   indexes[config.Index][reponame].Authors[auth],
				Name:     fn,
				URL:      indexes[config.Index][reponame].Authors[auth].URL + fn,
				Provides: make(map[string]*gopan.PerlPackage),
			}

			msg(" | Getting list of packages")
			modnm := strings.TrimSuffix(fn, ".tar.gz")
			pkg := indexes[config.Index][reponame].Authors[auth].Packages[fn]
			pandex.Provides(pkg, npath, ndir+"/"+modnm, ndir)

			//pkg := indexes[config.Index][reponame].Authors[auth].Packages[fn]
			msg(" | Adding packages to index")
			if _, ok := idxpackages[reponame]; !ok {
				idxpackages[reponame] = make(map[string]*PkgSpace)
			}
			filemap[pkg.AuthorURL()] = reponame
			for _, prov := range pkg.Provides {
				parts := strings.Split(prov.Name, "::")
				if _, ok := packages[parts[0]]; !ok {
					packages[parts[0]] = &PkgSpace{
						Namespace: parts[0],
						Packages:  make([]*gopan.PerlPackage, 0),
						Children:  make(map[string]*PkgSpace),
						Parent:    nil,
						Versions:  make(map[float64]*gopan.PerlPackage),
					}
				}
				if _, ok := idxpackages[reponame][parts[0]]; !ok {
					idxpackages[reponame][parts[0]] = &PkgSpace{
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
					idxpackages[reponame][parts[0]].Packages = append(idxpackages[reponame][parts[0]].Packages, prov)
					idxpackages[reponame][parts[0]].Versions[gopan.VersionFromString(prov.Version)] = prov
				} else {
					packages[parts[0]].Populate(parts[1:], prov)
					idxpackages[reponame][parts[0]].Populate(parts[1:], prov)
				}
			}

			msg(" | Writing to index file")
			gopan.AppendToIndex(config.CacheDir+"/"+config.Index, indexes[config.Index][reponame], indexes[config.Index][reponame].Authors[auth], indexes[config.Index][reponame].Authors[auth].Packages[fn])
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
		})
	}

	wg.Wait()

	footer, _ := session.RenderTemplate("layout_streamend.html")
	c <- []byte(footer)

	c <- make([]byte, 0)
}

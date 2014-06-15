package main

import (
	"github.com/ian-kent/go-log/log"
	"github.com/ian-kent/gotcha/http"
	"io/ioutil"
	nethttp "net/http"
	"os"
	"strings"
)

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

	if repo == "SmartPAN" {
		if _, ok := filemap[file]; !ok {
			log.Debug("SmartPAN repo - file [%s] not found in any index", file)
			session.RenderNotFound()
			return
		}

		repo = filemap[file]
		log.Debug("SmartPAN repo - file [%s] found in [%s]", file, repo)
	}

	log.Debug("Repo [%s], file [%s]", repo, file)

	nfile := config.CacheDir + "/" + repo + "/" + file

	if _, err := os.Stat(nfile); err != nil {
		log.Debug("File not found on disk, considering readthrough")

		for fn, _ := range indexes {
			log.Debug("Trying file: %s", fn)
			if src, ok := indexes[fn][repo]; ok {
				log.Debug("Found matching repo")
				if strings.HasPrefix(src.URL, "http:") {
					log.Debug("Found HTTP URL, trying: %s", src.URL+"/"+file)

					res, err := nethttp.Get(src.URL + "/" + file)
					if err != nil {
						log.Debug("Error on readthrough: %s", err.Error())
						continue
					}
					defer res.Body.Close()
					b, err := ioutil.ReadAll(res.Body)
					if err != nil {
						log.Debug("Error reading body: %s", err.Error())
						continue
					}

					session.Response.Write(b)
					return
				}
			}
		}

		log.Debug("No readthrough available")
		session.RenderNotFound()
		return
	}

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

package main

import (
	"github.com/ian-kent/go-log/log"
	"github.com/ian-kent/gotcha/http"
	"io/ioutil"
	"os"
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

package main

import (
	"github.com/ian-kent/go-log/log"
	"github.com/ian-kent/gopan/gopan"
	"github.com/ian-kent/gotcha/http"
	"html/template"
	"sort"
	"strings"
	"sync"
	"time"
)

type SearchResult struct {
	Name  string
	Type  string
	URL   string
	Obj   interface{}
	Glyph string
}

type ByName []*SearchResult

func (a ByName) Len() int           { return len(a) }
func (a ByName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByName) Less(i, j int) bool { return a[i].Name < a[j].Name }

func search(session *http.Session) {
	if q, ok := session.Request.Form()["q"]; ok {
		query := strings.ToLower(q[0])
		session.Stash["Query"] = q[0]
		results := make([]*SearchResult, 0)
		var lock sync.Mutex

		tStart := time.Now().UnixNano()

		log.Trace("Searching for [%s]", query)

		var wg sync.WaitGroup

		for fname, _ := range indexes {
			log.Trace("=> Searching file: %s", fname)
			for _, idx := range indexes[fname] {
				log.Trace("=> Searching index: %s", idx.Name)
				wg.Add(1)
				go func(idx *gopan.Source) {
					defer wg.Done()

					if strings.Contains(strings.ToLower(idx.Name), query) {
						lock.Lock()
						results = append(results, &SearchResult{
							Name:  idx.Name,
							Type:  "Index",
							URL:   idx.Name,
							Obj:   idx,
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
									Name:  auth.Name,
									Type:  "Author",
									URL:   idx.Name + "/authors/id/" + auth.Name[:1] + "/" + auth.Name[:2] + "/" + auth.Name,
									Obj:   auth,
									Glyph: "user",
								})
								lock.Unlock()
							}
							for _, pkg := range auth.Packages {
								if strings.Contains(strings.ToLower(pkg.Name), query) {
									lock.Lock()
									results = append(results, &SearchResult{
										Name:  pkg.Name,
										Type:  "Module",
										URL:   idx.Name + "/authors/id/" + auth.Name[:1] + "/" + auth.Name[:2] + "/" + auth.Name + "/" + pkg.Name,
										Obj:   pkg,
										Glyph: "compressed",
									})
									lock.Unlock()
								}
								for _, prov := range pkg.Provides {
									if strings.Contains(strings.ToLower(prov.Name), query) {
										lock.Lock()
										results = append(results, &SearchResult{
											Name:  prov.Name,
											Type:  "Package",
											URL:   idx.Name + "/modules/" + strings.Replace(prov.Name, "::", "/", -1),
											Obj:   prov,
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

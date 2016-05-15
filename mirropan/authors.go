package main

import (
	"github.com/companieshouse/gopan/gopan"
	"github.com/ian-kent/go-log/log"
	"golang.org/x/net/html"
	"net/http"
	"strings"
)

func getAuthors() int {
	newauth := 0

	var al func(*html.Node, *gopan.Source, string)
	al = func(n *html.Node, source *gopan.Source, prefix string) {
		log.Trace("NODE: %s [%s, %s, %s]", n.DataAtom, n.Type, n.Data)
		if n.Type == html.ElementNode && n.Data == "a" {
			//log.Trace("NODE IS ELEMENTNODE")
			for _, attr := range n.Attr {
				log.Trace("==> TEXT: %s", n.FirstChild.Data)
				if attr.Key == "href" && strings.HasPrefix(n.FirstChild.Data, prefix) {
					log.Trace("==> HREF: %s", attr.Val)
					author := strings.TrimSuffix(n.FirstChild.Data, "/")
					if _, ok := source.Authors[author]; !ok {
						source.Authors[author] = &gopan.Author{
							Name:     author,
							Source:   source,
							Packages: make(map[string]*gopan.Package),
							URL:      source.URL + "/" + author[:1] + "/" + author[:2] + "/" + author + "/",
						}
						newauth++
						log.Debug("Found author: %s", author)
					}
				}
			}
			//log.Trace("%s", n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			al(c, source, prefix)
		}
	}

	log.Info("Building author list")

	for fname, _ := range indexes {
		for _, source := range indexes[fname] {
			log.Info("Generating index: %s", source.Name)
			wg.Add(1)
			go func(source *gopan.Source) {
				defer wg.Done()
				for _, p1 := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
					wg.Add(1)
					go func(p1 rune, source *gopan.Source) {
						defer wg.Done()
						for _, p2 := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
							wg.Add(1)
							go func(p2 rune) {
								defer wg.Done()
								sem <- 1

								log.Trace("=> %s%s", string(p1), string(p2))

								url := source.URL + "/" + string(p1) + "/" + string(p1) + string(p2) + "/"
								log.Trace("Getting URL: %s", url)

								res, err := http.Get(url)
								if err != nil {
									log.Error("HTTP GET - %s", err.Error())
									<-sem
									return
								}

								doc, err := html.Parse(res.Body)
								if err != nil {
									log.Error("HTML PARSE - %s", err.Error())
									<-sem
									return
								}

								al(doc, source, string(p1)+string(p2))
								<-sem
							}(p2)
						}
					}(p1, source)
				}
			}(source)
		}
	}
	wg.Wait()

	log.Info("Finished building author list")

	return newauth
}

package main

import (
	"code.google.com/p/go.net/html"
	"github.com/ian-kent/go-log/log"
	"github.com/ian-kent/gopan/gopan"
	"net/http"
	"strings"
)

func getPackages() int {
	newpkg := 0

	var pl func(*html.Node, *gopan.Source, *gopan.Author)
	pl = func(n *html.Node, source *gopan.Source, author *gopan.Author) {
		log.Trace("NODE: %s [%s, %s, %s]", n.DataAtom, n.Type, n.Data)
		if n.Type == html.ElementNode && n.Data == "a" {
			//log.Info("NODE IS ELEMENTNODE")
			for _, attr := range n.Attr {
				// FIXME stuff that isn't .tar.gz?
				if attr.Key == "href" && strings.HasSuffix(attr.Val, ".tar.gz") {
					log.Trace("==> HREF: %s", n.FirstChild.Data)
					pkg := strings.TrimSuffix(n.FirstChild.Data, "/")
					if _, ok := author.Packages[pkg]; !ok {
						author.Packages[pkg] = &gopan.Package{
							Name:   pkg,
							Author: author,
							URL:    author.URL + "/" + pkg,
						}
						newpkg++
						log.Debug("Found package: %s", pkg)
					}
				}
			}
			//log.Info("%s", n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			pl(c, source, author)
		}
	}

	log.Info("Building package list")

	for _, source := range indexes {
		log.Debug("Index: %s", source)
		wg.Add(1)
		go func(source *gopan.Source) {
			defer wg.Done()
			for _, author := range source.Authors {
				wg.Add(1)
				go func(author *gopan.Author) {
					defer wg.Done()
					sem <- 1
					log.Trace("=> %s", author)

					url := source.URL + "/" + author.Name[:1] + "/" + author.Name[:2] + "/" + author.Name + "/"
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

					pl(doc, source, author)

					<-sem
				}(author)
			}
		}(source)
	}

	wg.Wait()

	log.Info("Finished building package list")

	return newpkg
}

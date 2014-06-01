package main

import (
	"github.com/ian-kent/go-log/log"
	"code.google.com/p/go.net/html"
	"net/http"
	"sync"
	"strings"
)

type Source struct {
	Name string
	Authors map[string]*Author
	URL string
}
type Author struct {
	Name string
	Packages map[string]*Package
	URL string
}
type Package struct {
	Name string
	URL string
}

func main() {
	indexes := []*Source{
		&Source{
			Name: "CPAN",
			URL: "http://www.cpan.org/authors/id",
			Authors: make(map[string]*Author, 0),
		},
		&Source{
			Name: "BackPAN",
			URL: "http://backpan.cpan.org/authors/id",
			Authors: make(map[string]*Author, 0),
		},

	}

	var wg = new(sync.WaitGroup)
	var sem = make(chan int, 10)

	// Get author list

	var al func(*html.Node, *Source, string)
	al = func(n *html.Node, source *Source, prefix string) {
		//log.Info("NODE: %s [%s, %s, %s]", n.DataAtom, n.Type, n.Data)
		if n.Type == html.ElementNode && n.Data == "a" {
			//log.Info("NODE IS ELEMENTNODE")
			for _, attr := range n.Attr {
				//log.Info(":::: %s", n.FirstChild.Data)
				if attr.Key == "href" && strings.HasPrefix(n.FirstChild.Data, prefix) {
					//log.Info("==> HREF: %s", attr.Val)
					author := strings.TrimSuffix(n.FirstChild.Data, "/")
					if _, ok := source.Authors[author]; !ok {
						source.Authors[author] = &Author{
							Name: author,
							Packages: make(map[string]*Package),
							URL: source.URL + "/" + author[:1] + "/" + author[:2] + "/" + author + "/",
						}
					}
				}
			}
			//log.Info("%s", n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			al(c, source, prefix)
		}
	}

	for _, source := range indexes {
		log.Info("Generating index: %s", source.Name)
		wg.Add(1)
		go func(source *Source) {
			defer wg.Done()
			for _, p1 := range "D" { // "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
				wg.Add(1)
				go func(p1 rune, source *Source) {
					defer wg.Done()
					for _, p2 := range "U" { // "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
						wg.Add(1)
						go func(p2 rune) {
							defer wg.Done()
							sem <- 1
							//log.Info("=> %s%s", string(p1), string(p2))

							url := source.URL + "/" + string(p1) + "/" + string(p1) + string(p2) + "/"
							//log.Info("   - %s", url)

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

							al(doc, source, string(p1) + string(p2))
							<-sem
						}(p2)
					}
				}(p1, source)
			}
		}(source)
	}
	wg.Wait()

	// Get package list

	var pl func(*html.Node, *Source, *Author)
	pl = func(n *html.Node, source *Source, author *Author) {
		//log.Info("NODE: %s [%s, %s, %s]", n.DataAtom, n.Type, n.Data)
		if n.Type == html.ElementNode && n.Data == "a" {
			//log.Info("NODE IS ELEMENTNODE")
			for _, attr := range n.Attr {
				//log.Info(":::: %s", n.FirstChild.Data)
				// FIXME stuff that isn't .tar.gz?
				if attr.Key == "href" && strings.HasSuffix(attr.Val, ".tar.gz") {
					log.Info("==> HREF: %s", n.FirstChild.Data)
					pkg := strings.TrimSuffix(n.FirstChild.Data, "/")
					if _, ok := author.Packages[pkg]; !ok {
						author.Packages[pkg] = &Package{
							Name: pkg,
						}
					}
				}
			}
			//log.Info("%s", n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			pl(c, source, author)
		}
	}

	for _, source := range indexes {
		//log.Info("Index: %s", idx)
		wg.Add(1)
		go func(source *Source) {
			defer wg.Done()
			for _, author := range source.Authors {
				//log.Info("=> %s", author)
				wg.Add(1)
				go func(author *Author) {
					defer wg.Done()
					sem <- 1
					url := source.URL + "/" + author.Name[:1] + "/" + author.Name[:2] + "/" + author.Name + "/"
					//log.Info("=> " + url)

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
}

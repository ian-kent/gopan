package main

import (
	"code.google.com/p/go.net/html"
	"flag"
	"github.com/ian-kent/go-log/log"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"io/ioutil"
	"regexp"
)

type Source struct {
	Name    string
	Authors map[string]*Author
	URL     string
}
type Author struct {
	Source   *Source
	Name     string
	Packages map[string]*Package
	URL      string
}
type Package struct {
	Author *Author
	Name   string
	URL    string
}

func (s *Source) String() string {
	return s.Name
}
func (a *Author) String() string {
	return a.Name
}
func (p *Package) String() string {
	return p.Name
}

func main() {
	sources := make([]string, 0)
	flag.Var((*AppendSliceValue)(&sources), "source", "Name=URL *PAN source (can be specified multiple times)")

	nocache := false
	flag.BoolVar(&nocache, "nocache", false, "Don't use the cached index file (.gopancache/index)")
	update := false
	flag.BoolVar(&update, "update", false, "Update the cached index file with new sources/authors/packages")

	nomirror := false
	flag.BoolVar(&nomirror, "nomirror", false, "Don't mirror, just index")

	cpan := false
	flag.BoolVar(&cpan, "cpan", false, "Add default CPAN source (only required if using -source)")
	backpan := false
	flag.BoolVar(&backpan, "backpan", false, "Add default BackPAN source (only required if using -source)")

	loglevel := "INFO"
	flag.StringVar(&loglevel, "loglevel", "INFO", "Log level")

	flag.Parse()

	log.Logger().SetLevel(log.Stol(loglevel))
	log.Info("Using log level: %s", loglevel)

	indexes := make([]*Source, 0)
	nsource := 0
	nauthor := 0
	npackage := 0

	var wg = new(sync.WaitGroup)
	var sem = make(chan int, 100)

	if !nocache {
		log.Info("Loading cached index file .gopancache/index")

		if _, err := os.Stat(".gopancache/index"); err != nil {
			log.Error("Cached index file not found")
			return
		}

		bytes, err := ioutil.ReadFile(".gopancache/index")
		if err != nil {
			log.Error("Error reading index: %s", err.Error())
			return
		}

		lines := strings.Split(string(bytes), "\n")
		var csource *Source
		var cauth *Author
		resrcauth := regexp.MustCompile("^\\s*(.*)\\s\\[(.*)\\]\\s*$")
		repackage := regexp.MustCompile("^\\s*(.*)\\s=>\\s(.*)\\s*$")
		for _, l := range lines {
			log.Trace("Line: %s", l)
			if strings.HasPrefix(l, "  ") {
				// its a package
				log.Trace("=> Package")
				match := repackage.FindStringSubmatch(l)
				if len(match) > 0 {
					cauth.Packages[match[1]] = &Package{
						Name: match[1],
						URL: match[2],
						Author: cauth,
					}
				}
			} else if strings.HasPrefix(l, " ") {
				// its an author
				log.Trace("=> Author")
				match := resrcauth.FindStringSubmatch(l)
				if len(match) > 0 {
					if _, ok := csource.Authors[match[1]]; ok {
						// we've seen this author before
						cauth = csource.Authors[match[1]]
						continue
					}
					cauth = &Author{
						Name: match[1],
						URL: match[2],
						Source: csource,
						Packages: make(map[string]*Package, 0),
					}
					csource.Authors[match[1]] = cauth
				}
			} else {
				// its a source
				log.Trace("=> Source")
				match := resrcauth.FindStringSubmatch(l)
				if len(match) > 0 {
					for _, idx := range indexes {
						if idx.Name == match[1] {
							// we've seen this source before
							csource = idx
							continue
						}
					}
					csource = &Source{
						Name: match[1],
						URL: match[2],
						Authors: make(map[string]*Author, 0),
					}
					indexes = append(indexes, csource)
				}
			}
		}

		for _, source := range indexes {
			nsource++
			log.Trace(source.Name)
			for _, author := range source.Authors {
				nauthor++
				log.Trace("    %s", author.Name)
				for _, pkg := range author.Packages {
					npackage++
					log.Trace("        %s => %s", pkg.Name, pkg.URL)
				}
			}
		}
	}

	if nocache || update {
		if update {
			log.Debug("Updating author and package lists")
		}

		for _, s := range sources {
			b := strings.SplitN(s, "=", 2)
			if len(b) < 2 {
				log.Error("Expected Name=URL pair, got: %s", s)
				return
			}

			indexes = append(indexes, &Source{
				Name:    b[0],
				URL:     b[1],
				Authors: make(map[string]*Author, 0),
			})
		}

		if len(indexes) == 0 {
			log.Debug("No -source parameters, adding default CPAN/BackPAN")
			cpan = true
			backpan = true
		}

		if cpan {
			log.Debug("Adding CPAN index")
			indexes = append(indexes, &Source{
				Name:    "CPAN",
				URL:     "http://www.cpan.org/authors/id",
				Authors: make(map[string]*Author, 0),
			})
		}

		if backpan {
			log.Debug("Adding BackPAN index")
			indexes = append(indexes, &Source{
				Name:    "BackPAN",
				URL:     "http://backpan.cpan.org/authors/id",
				Authors: make(map[string]*Author, 0),
			})
		}

		log.Info("Using sources:")
		for _, source := range indexes {
			log.Info("=> %s", source.String())
		}

		// Get author list
		newauth := 0
		newpkg := 0

		var al func(*html.Node, *Source, string)
		al = func(n *html.Node, source *Source, prefix string) {
			log.Trace("NODE: %s [%s, %s, %s]", n.DataAtom, n.Type, n.Data)
			if n.Type == html.ElementNode && n.Data == "a" {
				//log.Trace("NODE IS ELEMENTNODE")
				for _, attr := range n.Attr {
					log.Trace("==> TEXT: %s", n.FirstChild.Data)
					if attr.Key == "href" && strings.HasPrefix(n.FirstChild.Data, prefix) {
						log.Trace("==> HREF: %s", attr.Val)
						author := strings.TrimSuffix(n.FirstChild.Data, "/")
						if _, ok := source.Authors[author]; !ok {
							source.Authors[author] = &Author{
								Name:     author,
								Source:   source,
								Packages: make(map[string]*Package),
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

		for _, source := range indexes {
			log.Info("Generating index: %s", source.Name)
			wg.Add(1)
			go func(source *Source) {
				defer wg.Done()
				for _, p1 := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
					wg.Add(1)
					go func(p1 rune, source *Source) {
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
		wg.Wait()

		log.Info("Finished building author list")

		// Get package list

		var pl func(*html.Node, *Source, *Author)
		pl = func(n *html.Node, source *Source, author *Author) {
			log.Trace("NODE: %s [%s, %s, %s]", n.DataAtom, n.Type, n.Data)
			if n.Type == html.ElementNode && n.Data == "a" {
				//log.Info("NODE IS ELEMENTNODE")
				for _, attr := range n.Attr {
					// FIXME stuff that isn't .tar.gz?
					if attr.Key == "href" && strings.HasSuffix(attr.Val, ".tar.gz") {
						log.Trace("==> HREF: %s", n.FirstChild.Data)
						pkg := strings.TrimSuffix(n.FirstChild.Data, "/")
						if _, ok := author.Packages[pkg]; !ok {
							author.Packages[pkg] = &Package{
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
			go func(source *Source) {
				defer wg.Done()
				for _, author := range source.Authors {
					wg.Add(1)
					go func(author *Author) {
						defer wg.Done()
						sem <- 1
						log.Debug("=> %s", author)
						
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

		os.MkdirAll(".gopancache", 0777)

		// TODO append, but needs to know which stuff is new
		//out, err := os.OpenFile(".gopancache/index", os.O_RDWR|os.O_APPEND, 0660)
		out, err := os.Create(".gopancache/index")
		if err != nil {
			log.Error("Error creating index: %s", err.Error())
		}
		for _, source := range indexes {
			nsource++
			out.Write([]byte(source.Name + " [" + source.URL + "]\n"))
			log.Trace(source.Name)
			for _, author := range source.Authors {
				nauthor++
				out.Write([]byte(" " + author.Name + " [" + author.URL + "]\n"))
				log.Trace("    %s", author.Name)
				for _, pkg := range author.Packages {
					npackage++
					out.Write([]byte("  " + pkg.Name + " => " + pkg.URL + "\n"))
					log.Trace("        %s => %s", pkg.Name, pkg.URL)
				}
			}
		}
		out.Close()

		if update {
			log.Info("Found %d new packages by %d new authors", newpkg, newauth)
		}
	}

	log.Info("Found %d packages by %d authors from %d sources", npackage, nauthor, nsource)

	if !nomirror {
		log.Info("Mirroring *PAN")

		for _, source := range indexes {
			log.Debug("Index: %s", source)
			wg.Add(1)
			go func(source *Source) {
				defer wg.Done()
				for _, author := range source.Authors {
					log.Debug("=> %s", author)
					wg.Add(1)
					go func(author *Author) {
						cachedir := ".gopancache/" + source.Name + "/" + author.Name[:1] + "/" + author.Name[:2] + "/" + author.Name + "/"
						os.MkdirAll(cachedir, 0777)

						defer wg.Done()
						for _, pkg := range author.Packages {
							wg.Add(1)
							go func(pkg *Package) {
								defer wg.Done()
								sem <- 1

								cache := cachedir + pkg.Name
								log.Trace("    - Caching to: %s", cache)

								if _, err := os.Stat(cache); err == nil {
									log.Debug("  |> %s", pkg)
									log.Trace("    - Already exists in cache")
									<-sem
									return
								}

								log.Debug("  => %s", pkg)

								url := source.URL + "/" + author.Name[:1] + "/" + author.Name[:2] + "/" + author.Name + "/" + pkg.Name
								log.Trace("    - From URL: %s", url)

								out, err := os.Create(cache)
								defer out.Close()
								if err != nil {
									log.Error("CREATE - %s", err.Error())
									<-sem
									return
								}

								resp, err := http.Get(url)
								if err != nil {
									log.Error("HTTP GET - %s", err.Error())
									<-sem
									return
								}

								_, err = io.Copy(out, resp.Body)
								if err != nil {
									log.Error("IO COPY - %s", err.Error())
								}

								<-sem
							}(pkg)
						}
					}(author)
				}
			}(source)
		}

		wg.Wait()
		log.Info("Finished mirroring *PAN")
	}
}

package main

import(
	"github.com/ian-kent/go-log/log"
	"github.com/ian-kent/gopan/gopan"
	"os"
	"io"
	"net/http"
)

func mirrorPan() {
	log.Info("Mirroring *PAN")

	// FIXME inefficient
	_, _, npkg := gopan.CountIndex(indexes)

	mirrored := 0
	var pc = func() int {
		return mirrored / npkg * 100
	}

	for _, source := range indexes {
		log.Debug("Index: %s", source)
		wg.Add(1)
		go func(source *gopan.Source) {
			defer wg.Done()
			for _, author := range source.Authors {
				log.Debug("=> %s", author)
				wg.Add(1)
				go func(author *gopan.Author) {
					cachedir := ".gopancache/" + source.Name + "/" + author.Name[:1] + "/" + author.Name[:2] + "/" + author.Name + "/"
					os.MkdirAll(cachedir, 0777)

					defer wg.Done()
					for _, pkg := range author.Packages {
						wg.Add(1)
						go func(pkg *gopan.Package) {
							defer wg.Done()

							cache := cachedir + pkg.Name
							log.Trace("    - Caching to: %s", cache)

							if _, err := os.Stat(cache); err == nil {
								log.Debug("%d%%  |> %s", pc(), pkg)
								log.Trace("    - Already exists in cache")
								mirrored++
								return
							}

							sem <- 1

							mirrored++

							log.Debug("%d%%  => %s", pc(), pkg)

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
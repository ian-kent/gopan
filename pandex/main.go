package main

import (
	"github.com/ian-kent/go-log/log"
	"github.com/ian-kent/gopan/gopan"
	"github.com/ian-kent/gopan/pandex/pandex"
	"sync"
	"os"
	"strings"
)

var wg = new(sync.WaitGroup)
var sem = make(chan int, 6)
var indexes map[string]*gopan.Source

func main() {
	configure()

	indexes := gopan.LoadIndex(config.CacheDir + "/index")

	log.Logger().SetLevel(log.Stol(config.LogLevel))
	log.Info("Using log level: %s", config.LogLevel)

	// FIXME inefficient
	_, _, tpkg, _ := gopan.CountIndex(indexes)

	npkg := 0
	nmod := 0

	var pc = func() float64 {
		return float64(nmod) / float64(tpkg) * 100
	}

	for _, idx := range indexes {
		log.Debug("Index: %s", idx)		
		for _, auth := range idx.Authors {			
			log.Debug("Author %s", auth)
			for _, pkg := range auth.Packages {
				wg.Add(1)
				go func(idx *gopan.Source, auth *gopan.Author, pkg *gopan.Package) {
					defer wg.Done()

					log.Debug("Package: %s", pkg)

					sem <- 1

					// TODO better handling of filenames
					modnm := strings.TrimSuffix(pkg.Name, ".tar.gz")

					tgzpath := config.CacheDir + "/" + idx.Name + "/" + auth.Name[:1] + "/" + auth.Name[:2] + "/" + auth.Name + "/" + pkg.Name

					if _, err := os.Stat(tgzpath); err != nil {
						log.Error("File not found: %s", tgzpath)
						return;
					}

					extpath := config.ExtDir + "/" + idx.Name + "/" + auth.Name[:1] + "/" + auth.Name[:2] + "/" + auth.Name + "/" + modnm
					dirpath := config.ExtDir + "/" + idx.Name + "/" + auth.Name[:1] + "/" + auth.Name[:2] + "/" + auth.Name

					log.Trace("=> tgzpath: %s", tgzpath)
					log.Trace(" > extpath: %s", extpath)
					log.Trace(" > dirpath: %s", dirpath)

					pandex.Provides(pkg, tgzpath, extpath, dirpath)

					npkg += len(pkg.Provides)
					nmod += 1

					if nmod > 0 && nmod % 100 == 0 {
						log.Info("%f%% Done %d/%d packages (%d provided so far)", pc(), nmod, tpkg, npkg)
					}

					<-sem
				}(idx, auth, pkg)
			}
		}
	}

	wg.Wait()

	log.Info("Writing packages index file")

	out, err := os.Create(config.CacheDir + "/packages")
	if err != nil {
		log.Error("Error creating packages index: %s", err.Error())
	}

	// FIXME same as gopan/index.go?
	for _, idx := range indexes {
		out.Write([]byte(idx.Name + " [" + idx.URL + "]\n"))
		for _, auth := range idx.Authors {
			out.Write([]byte(" " + auth.Name + " [" + auth.URL + "]\n"))
			for _, pkg := range auth.Packages {
				out.Write([]byte("  " + pkg.Name + " => " + pkg.URL + "\n"))
				for p, pk := range pkg.Provides {
					out.Write([]byte("   " + p + " (" + pk.Version + "): " + pk.File + "\n"))
				}
			}
		}
	}

	out.Close()

	log.Info("Found %d packages from %d modules", npkg, nmod)
}

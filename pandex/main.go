package main

import (
	"github.com/companieshouse/gopan/gopan"
	"github.com/companieshouse/gopan/pandex/pandex"
	"github.com/ian-kent/go-log/log"
	"os"
	"strings"
	"sync"
)

var wg = new(sync.WaitGroup)
var sem = make(chan int, 6)
var indexes map[string]map[string]*gopan.Source

func main() {
	configure()

	indexes = make(map[string]map[string]*gopan.Source)
	indexes[config.InputIndex] = gopan.LoadIndex(config.CacheDir + "/" + config.InputIndex)

	log.Logger().SetLevel(log.Stol(config.LogLevel))
	log.Info("Using log level: %s", config.LogLevel)

	// FIXME inefficient
	_, _, tpkg, _ := gopan.CountIndex(indexes)

	npkg := 0
	nmod := 0

	var pc = func() float64 {
		return float64(nmod) / float64(tpkg) * 100
	}

	log.Info("Writing packages index file")

	out, err := os.Create(config.CacheDir + "/" + config.OutputIndex)
	if err != nil {
		log.Error("Error creating packages index: %s", err.Error())
		return
	}

	for fname, _ := range indexes {
		log.Debug("File: %s", fname)
		for _, idx := range indexes[fname] {
			log.Debug("Index: %s", idx)
			out.Write([]byte(idx.Name + " [" + idx.URL + "]\n"))
			for _, auth := range idx.Authors {
				log.Debug("Author %s", auth)
				out.Write([]byte(" " + auth.Name + " [" + auth.URL + "]\n"))
				for _, pkg := range auth.Packages {
					out.Write([]byte("  " + pkg.Name + " => " + pkg.URL + "\n"))
					log.Debug("Package: %s", pkg)

					if !config.Flatten {
						if len(pkg.Provides) == 0 {
							// TODO better handling of filenames
							modnm := strings.TrimSuffix(pkg.Name, ".tar.gz")

							tgzpath := config.CacheDir + "/" + idx.Name + "/" + auth.Name[:1] + "/" + auth.Name[:2] + "/" + auth.Name + "/" + pkg.Name

							if _, err := os.Stat(tgzpath); err != nil {
								log.Error("File not found: %s", tgzpath)
								return
							}

							extpath := config.ExtDir + "/" + idx.Name + "/" + auth.Name[:1] + "/" + auth.Name[:2] + "/" + auth.Name + "/" + modnm
							dirpath := config.ExtDir + "/" + idx.Name + "/" + auth.Name[:1] + "/" + auth.Name[:2] + "/" + auth.Name

							log.Trace("=> tgzpath: %s", tgzpath)
							log.Trace(" > extpath: %s", extpath)
							log.Trace(" > dirpath: %s", dirpath)

							// Only index packages if they don't already exist
							if err := pandex.Provides(pkg, tgzpath, extpath, dirpath); err != nil {
								log.Error("Error retrieving package list: %s", err)
								continue
							}
						}

						npkg += len(pkg.Provides)
						nmod += 1

						for p, pk := range pkg.Provides {
							out.Write([]byte("   " + p + " (" + pk.Version + "): " + pk.File + "\n"))
						}

						if nmod > 0 && nmod%100 == 0 {
							log.Info("%f%% Done %d/%d packages (%d provided so far)", pc(), nmod, tpkg, npkg)
						}

					}
				}
			}
		}
	}

	out.Close()

	log.Info("Found %d packages from %d modules", npkg, nmod)
}

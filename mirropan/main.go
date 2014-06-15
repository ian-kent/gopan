package main

import (
	"github.com/ian-kent/go-log/log"
	"github.com/ian-kent/gopan/gopan"
	"os"
	"strings"
	"sync"
)

var wg = new(sync.WaitGroup)
var sem = make(chan int, 100)
var indexes map[string]map[string]*gopan.Source

func main() {
	configure()

	log.Logger().SetLevel(log.Stol(config.LogLevel))
	log.Info("Using log level: %s", config.LogLevel)

	indexes = make(map[string]map[string]*gopan.Source)
	if !config.NoCache {
		indexes[config.Index] = gopan.LoadIndex(config.CacheDir + "/" + config.Index)
	}

	if config.NoCache || config.Update {
		for _, s := range config.Sources {
			b := strings.SplitN(s, "=", 2)
			if len(b) < 2 {
				log.Error("Expected Name=URL pair, got: %s", s)
				return
			}

			if idx, ok := indexes[config.Index][b[0]]; ok {
				log.Warn("Index [%s] already exists with URL [%s], updating to [%s]", idx.URL, b[1])
				idx.URL = b[1]
			} else {
				indexes[config.Index][b[0]] = &gopan.Source{
					Name:    b[0],
					URL:     b[1],
					Authors: make(map[string]*gopan.Author, 0),
				}
			}
		}

		if len(config.Sources) == 0 && !config.CPAN && !config.BackPAN {
			log.Debug("No -source, -cpan, -backpan parameters, adding default CPAN/BackPAN")
			config.CPAN = true
			config.BackPAN = true
		}

		if config.CPAN {
			if _, ok := indexes[config.Index]["CPAN"]; !ok {
				log.Debug("Adding CPAN index")
				indexes[config.Index]["CPAN"] = gopan.CPANSource()
			} else {
				log.Debug("CPAN index already exists")
			}
		}

		if config.BackPAN {
			if _, ok := indexes[config.Index]["BackPAN"]; !ok {
				log.Debug("Adding BackPAN index")
				indexes[config.Index]["BackPAN"] = gopan.BackPANSource()
			} else {
				log.Debug("BackPAN index already exists")
			}
		}

		log.Info("Using sources:")
		for fname, _ := range indexes {
			log.Info("From %s", fname)
			for _, source := range indexes[fname] {
				log.Info("=> %s", source.String())
			}
		}

		newAuthors := getAuthors()
		newPackages := getPackages()

		os.MkdirAll(config.CacheDir, 0777)

		if !config.NoCache {
			gopan.SaveIndex(config.CacheDir+"/"+config.Index, indexes[config.Index])
		}

		if config.Update {
			log.Info("Found %d new packages by %d new authors", newAuthors, newPackages)
		}
	}

	nsrc, nauth, nmod, npkg := gopan.CountIndex(indexes)
	log.Info("Found %d packages in %d modules by %d authors from %d sources", npkg, nmod, nauth, nsrc)

	if !config.NoMirror {
		mirrorPan()
	}
}

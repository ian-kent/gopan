package main

import (
	"github.com/ian-kent/go-log/log"
	"github.com/ian-kent/gopan/gopan"
	"sync"
	"os/exec"
	"bytes"
	"strings"
	"os"
	"encoding/json"
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
	_, _, tpkg := gopan.CountIndex(indexes)

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

					sem <- 1

					log.Debug("Package: %s", pkg)

					// TODO better handling of filenames
					modnm := strings.TrimSuffix(pkg.Name, ".tar.gz")

					tgzpath := config.CacheDir + "/" + idx.Name + "/" + auth.Name[:1] + "/" + auth.Name[:2] + "/" + auth.Name + "/" + pkg.Name

					if _, err := os.Stat(tgzpath); err != nil {
						log.Error("File not found: %s", tgzpath)
						<-sem
						return;
					}

					extpath := config.ExtDir + "/" + idx.Name + "/" + auth.Name[:1] + "/" + auth.Name[:2] + "/" + auth.Name + "/" + modnm
					dirpath := config.ExtDir + "/" + idx.Name + "/" + auth.Name[:1] + "/" + auth.Name[:2] + "/" + auth.Name

					log.Trace("=> tgzpath: %s", tgzpath)
					log.Trace(" > extpath: %s", extpath)
					log.Trace(" > dirpath: %s", dirpath)

					// not required? path should already exist
					os.MkdirAll(dirpath, 0770)

					var stdout1 bytes.Buffer
					var stderr1 bytes.Buffer

					extract := exec.Command("tar", "-zxf", tgzpath, "-C", dirpath)
					extract.Stdout = &stdout1				
					extract.Stderr = &stderr1

					if err := extract.Run(); err != nil {
						log.Error("Extract run: %s", err.Error())
						log.Trace(stdout1.String())
						log.Error(stderr1.String())
						<-sem
						return;
					}

					log.Trace(stdout1.String())
					log.Trace(stderr1.String())

					defer func() {
						var stdout3 bytes.Buffer
						var stderr3 bytes.Buffer

						clean := exec.Command("rm", "-rf", extpath)
						clean.Stdout = &stdout3			
						clean.Stderr = &stderr3

						if err := clean.Run(); err != nil {
							log.Error("Clean run: %s", err.Error())
						}

						log.Trace(stdout3.String())
						log.Trace(stderr3.String())
					}()

					//var stdout2 bytes.Buffer
					var stderr2 bytes.Buffer

					cmd := exec.Command("perl", "-MModule::Metadata", "-MJSON::XS", "-e", "print encode_json(Module::Metadata->provides(version => 2, prefix => \"\", dir => $ARGV[0]))", extpath)
					//cmd.Stdout = &stdout2				
					cmd.Stderr = &stderr2

					stdout, err := cmd.StdoutPipe()
					defer stdout.Close()
					if err != nil {
						log.Error("StdoutPipe: %s", err.Error())
						<-sem
						return;
					}

					if err := cmd.Start(); err != nil {
						log.Error("Start: %s", err.Error())
						<-sem
						return;
					}

					if err := json.NewDecoder(stdout).Decode(&pkg.Provides); err != nil {
						log.Error("JSON decoder error: %s", err.Error())
						<-sem
						return;
					}

					if err := cmd.Wait(); err != nil {
						log.Error("Wait: %s", err.Error())
						<-sem
						return;
					}

					//log.Trace(stdout2.String())
					log.Trace(stderr2.String())

					for p, pk := range pkg.Provides {
						log.Trace("%s: %s %s", p, pk.Version, pk.File)
					}

					log.Debug("%s provides %d packages", pkg, len(pkg.Provides))

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

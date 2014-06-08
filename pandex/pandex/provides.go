package pandex

import (
	"github.com/ian-kent/go-log/log"
	"github.com/ian-kent/gopan/gopan"
	"os/exec"
	"bytes"
	"os"
	"encoding/json"
)

func Provides(pkg *gopan.Package, tgzpath string, extpath string, dirpath string) {
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
		return;
	}

	if err := cmd.Start(); err != nil {
		log.Error("Start: %s", err.Error())
		return;
	}

	if err := json.NewDecoder(stdout).Decode(&pkg.Provides); err != nil {
		log.Error("JSON decoder error: %s", err.Error())
		return;
	}

	if err := cmd.Wait(); err != nil {
		log.Error("Wait: %s", err.Error())
		return;
	}

	//log.Trace(stdout2.String())
	log.Trace(stderr2.String())

	for p, pk := range pkg.Provides {
		pk.Name = p
		pk.Package = pkg
		log.Trace("%s: %s %s", p, pk.Version, pk.File)
	}

	log.Debug("%s provides %d packages", pkg, len(pkg.Provides))
}
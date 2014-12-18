package pandex

import (
	"bytes"
	"encoding/json"
	"github.com/ian-kent/go-log/log"
	"github.com/ian-kent/gopan/gopan"
	"os"
	"os/exec"
)

type Entry struct {
	Parsed    int
	Filemtime int
	Version   string
	Infile    string
	Simile    string
}
type PLD map[string]*Entry

var pldArgs string

func Provides(pkg *gopan.Package, tgzpath string, extpath string, dirpath string) error {
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
		return err
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

	if len(pldArgs) == 0 {
		if len(os.Getenv("GOPAN_ALLOW_DEV_VERSIONS")) > 0 {
			pldArgs = "({ALLOW_DEV_VERSION=>1})"
		} else {
			pldArgs = "()"
		}
	}
	log.Trace("pldArgs: %s", pldArgs)
	//cmd := exec.Command("perl", "-MModule::Metadata", "-MJSON::XS", "-e", "print encode_json(Module::Metadata->provides(version => 2, prefix => \"\", dir => $ARGV[0]))", extpath)
	cmd := exec.Command("perl", "-MParse::LocalDistribution", "-MJSON::XS", "-e", "print encode_json(Parse::LocalDistribution->new"+pldArgs+"->parse($ARGV[0]))", extpath)
	//cmd.Stdout = &stdout2
	cmd.Stderr = &stderr2

	stdout, err := cmd.StdoutPipe()
	defer stdout.Close()
	if err != nil {
		log.Error("StdoutPipe: %s", err.Error())
		return err
	}

	if err := cmd.Start(); err != nil {
		log.Error("Start: %s", err.Error())
		return err
	}

	var pld PLD
	if err := json.NewDecoder(stdout).Decode(&pld); err != nil {
		log.Error("JSON decoder error: %s", err.Error())
		return err
	}

	if err := cmd.Wait(); err != nil {
		log.Error("Wait: %s", err.Error())
		return err
	}

	//log.Trace(stdout2.String())
	log.Trace(stderr2.String())

	pkg.Provides = make(map[string]*gopan.PerlPackage)
	for p, pk := range pld {
		pp := &gopan.PerlPackage{
			Package: pkg,
			Name:    p,
			Version: pk.Version,
			File:    pk.Infile,
		}
		pkg.Provides[p] = pp
		log.Trace("%s: %s %s", p, pp.Version, pp.File)
	}

	log.Debug("%s provides %d packages", pkg, len(pkg.Provides))
	return nil
}

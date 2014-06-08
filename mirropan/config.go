package main

import (
	"github.com/ian-kent/gopan/gopan"
	"flag"
)

type Config struct {
	Sources  []string
	NoCache  bool
	Update   bool
	NoMirror bool
	CPAN     bool
	BackPAN  bool
	LogLevel string
	CacheDir string
}

var config *Config

func configure() {
	sources := make([]string, 0)
	flag.Var((*gopan.AppendSliceValue)(&sources), "source", "Name=URL *PAN source (can be specified multiple times)")

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

	cachedir := ".gopancache"
	flag.StringVar(&cachedir, "cachedir", ".gopancache", "GoPAN cache directory")

	flag.Parse()

	config = &Config{
		Sources: sources,
		NoCache: nocache,
		Update: update,
		NoMirror: nomirror,
		CPAN: cpan,
		BackPAN: backpan,
		LogLevel: loglevel,
		CacheDir: cachedir,
	}
}

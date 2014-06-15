package main

import (
	"flag"
	"github.com/ian-kent/gopan/gopan"
)

type Config struct {
	LogLevel        string
	CacheDir        string
	Index           string
	Bind            string
	Indexes         []string
	LatestRelease   string
	CurrentRelease  string
	CanUpdate       bool
	UpdateURL       string
	ImportAvailable bool

	CPANIndex        string
	HasCPANIndex     bool
	CPANIndexDate    string
	BackPANIndex     string
	HasBackPANIndex  bool
	BackPANIndexDate string
}

var config *Config

func configure() {
	loglevel := "INFO"
	flag.StringVar(&loglevel, "loglevel", "INFO", "Log level")

	cachedir := ".gopancache"
	flag.StringVar(&cachedir, "cachedir", ".gopancache", "GoPAN cache directory")

	index := "index"
	flag.StringVar(&index, "index", "index", "Name of the primary GoPAN index file")

	bind := ":7050"
	flag.StringVar(&bind, "bind", ":7050", "Interface to bind to")

	cpan := "cpan_index"
	flag.StringVar(&cpan, "cpan", "cpan_index", "Name of CPAN index to support readthrough")

	backpan := "backpan_index"
	flag.StringVar(&backpan, "backpan", "backpan_index", "Name of BackPAN index to support readthrough")

	indexes := make([]string, 0)
	flag.Var((*gopan.AppendSliceValue)(&indexes), "s-index", "Secondary indexes to load (can be provided multiple times)")

	flag.Parse()

	config = &Config{
		LogLevel:         loglevel,
		CacheDir:         cachedir,
		Index:            index,
		Bind:             bind,
		Indexes:          indexes,
		CanUpdate:        false,
		LatestRelease:    "0.0",
		CurrentRelease:   "0.0", // set in main.go so its in one place
		UpdateURL:        "",
		ImportAvailable:  false,
		CPANIndex:        cpan,
		HasCPANIndex:     false,
		CPANIndexDate:    "",
		BackPANIndex:     backpan,
		HasBackPANIndex:  false,
		BackPANIndexDate: "",
	}
}

package main

import (
	"flag"

	"github.com/companieshouse/gopan/gopan"
	"github.com/ian-kent/go-log/layout"
	"github.com/ian-kent/go-log/log"
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

	BackPANStatus string
	CPANStatus    string

	TestDeps bool

	// For some command line commands, e.g. `smartpan import`
	RemoteHost string
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

	cpan := "cpan_index.gz"
	flag.StringVar(&cpan, "cpan", "cpan_index.gz", "Name of CPAN index to support readthrough")

	backpan := "backpan_index.gz"
	flag.StringVar(&backpan, "backpan", "backpan_index.gz", "Name of BackPAN index to support readthrough")

	remote := "http://localhost:7050"
	flag.StringVar(&remote, "remote", "http://localhost:7050", "Remote host and port to connect to")

	loglayout := "[%d] [%p] %m"
	flag.StringVar(&loglayout, "loglayout", loglayout, "Log layout (for github.com/ian-kent/go-log pattern layout)")

	indexes := make([]string, 0)
	flag.Var((*gopan.AppendSliceValue)(&indexes), "s-index", "Secondary indexes to load (can be provided multiple times)")

	testdeps := true
	flag.BoolVar(&testdeps, "test-deps", true, "Test dependencies on startup")

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
		BackPANStatus:    "Unavailable",
		CPANStatus:       "Unavailable",
		RemoteHost:       remote,
		TestDeps:         testdeps,
	}

	// log level and layout
	log.Logger().Appender().SetLayout(layout.Pattern(loglayout))
	log.Logger().SetLevel(log.Stol(loglevel))
}

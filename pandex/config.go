package main

import (
	"flag"
)

type Config struct {
	Update   bool
	LogLevel string
	CacheDir string
	ExtDir   string
}

var config *Config

func configure() {
	update := false
	flag.BoolVar(&update, "update", false, "Update the cached packages file with new packages")

	loglevel := "INFO"
	flag.StringVar(&loglevel, "loglevel", "INFO", "Log level")

	cachedir := ".gopancache"
	flag.StringVar(&cachedir, "cachedir", ".gopancache", "GoPAN cache directory")

	extdir := ".gopancache"
	flag.StringVar(&cachedir, "extdir", ".gopancache", "Temporary directory for extraction")

	flag.Parse()

	config = &Config{
		Update:   update,
		LogLevel: loglevel,
		CacheDir: cachedir,
		ExtDir:   extdir,
	}
}

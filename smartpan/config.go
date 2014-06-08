package main

import (
	"flag"
)

type Config struct {
	LogLevel string
	CacheDir string
	Index string
}

var config *Config

func configure() {
	loglevel := "INFO"
	flag.StringVar(&loglevel, "loglevel", "INFO", "Log level")

	cachedir := ".gopancache"
	flag.StringVar(&cachedir, "cachedir", ".gopancache", "GoPAN cache directory")

	index := "index"
	flag.StringVar(&index, "index", "index", "Name of GoPAN index file")

	flag.Parse()

	config = &Config{
		LogLevel: loglevel,
		CacheDir: cachedir,
		Index: index,
	}
}

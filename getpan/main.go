package main

import (
	"github.com/ian-kent/go-log/log"
	"github.com/ian-kent/gopan/getpan/getpan"
)

var config *getpan.Config

func main() {
	config = getpan.Configure()
	config.Dump()

	for _, source := range config.Sources {
		err := source.Load()
		if err != nil {
			log.Error("Error loading sources: %s", err)
			return // TODO exit code?
		}
	}

	deps, err := getpan.ParseCPANFile(config.CPANFile)
	if err != nil {
		log.Error("Error parsing cpanfile: %s", err)
		return
	}

	err = deps.Resolve()
	if err != nil {
		log.Error("Error resolving dependencies: %s", err)
		return
	}

	log.Info("Resolved dependency tree:")
	deps.PrintDeps(0)

	if config.NoInstall {
		log.Info("Skipping installation phase")
		return
	}

	n, err := deps.Install()

	if err != nil {
		log.Error("Error installing dependencies: %s", err)
		return
	}

	log.Info("Successfully installed %d modules", n)
}

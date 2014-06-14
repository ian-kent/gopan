package main

import (
	"github.com/ian-kent/go-log/log"
	"github.com/ian-kent/gopan/getpan/getpan"
	"flag"
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

	mods := flag.Args()

	var deps *getpan.DependencyList

	if len(mods) == 0 {
		log.Info("Installing from cpanfile: %s", config.CPANFile)
		d, err := getpan.ParseCPANFile(config.CPANFile)
		if err != nil {
			log.Error("Error parsing cpanfile: %s", err)
			return
		}
		deps = &d.DependencyList
	} else {
		log.Info("Installing from command line args")
		deps = &getpan.DependencyList{
			Dependencies: make([]*getpan.Dependency, 0),
		}
		for _, arg := range mods {
			dependency, err := getpan.DependencyFromString(arg, "")
			if err != nil {
				log.Error("Unable to parse input: %s", arg)
				continue
			}
			deps.AddDependency(dependency)
		}
	}

	err := deps.Resolve()
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

	_, err = deps.Install()

	if err != nil {
		log.Error("Error installing dependencies: %s", err)
		return
	}

	// FIXME hacky, need a better way of tracking installed deps
	log.Info("Successfully installed %d modules", deps.UniqueInstalled())
}

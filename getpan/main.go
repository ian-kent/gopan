package main

import (
	"github.com/ian-kent/go-log/log"
	"github.com/ian-kent/gopan/getpan/getpan"
	"flag"
	"strings"
	"os/exec"
	"os"
)

var config *getpan.Config

func main() {
	config = getpan.Configure()

	log.Debug("GoPAN configuration:")
	config.Dump()

	mods := flag.Args()

	if len(mods) > 0 && mods[0] == "exec" {
		log.Debug("getpan exec => " + strings.Join(mods[1:], " "))

		cmd := exec.Command(mods[1], mods[2:]...)

		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, "PERL5LIB=./local/lib/perl5")
		cmd.Env = append(cmd.Env, "PATH=$PATH:./local/bin")

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Run()

		if err != nil {
			// debug so it doesn't show up in stdout/stderr unless -loglevel is used
			log.Debug("Error in exec: %s", err.Error())
		}

		return
	}

	for _, source := range config.Sources {
		err := source.Load()
		if err != nil {
			log.Error("Error loading sources: %s", err)
			return // TODO exit code?
		}
	}

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

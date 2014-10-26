package main

import (
	"flag"
	"os"
	"os/exec"
	"strings"

	"github.com/ian-kent/go-log/log"
	"github.com/ian-kent/gopan/getpan/getpan"
)

var config *getpan.Config

func main() {
	config = getpan.Configure()
	config.Dump()

	mods := flag.Args()

	if len(mods) == 0 {
		if _, err := os.Stat(config.CPANFile); os.IsNotExist(err) {
			log.Error("cpanfile not found: %s", config.CPANFile)
			os.Exit(1)
		}
	}

	if len(mods) > 0 && mods[0] == "exec" {
		log.Debug("getpan exec => " + strings.Join(mods[1:], " "))

		cmd := exec.Command(mods[1], mods[2:]...)

		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, "PERL5LIB="+config.InstallDir+"/lib/perl5")
		cmd.Env = append(cmd.Env, "PATH="+os.Getenv("PATH")+":"+config.InstallDir+"/bin")

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Run()

		if err != nil {
			// debug so it doesn't show up in stdout/stderr unless -loglevel is used
			log.Debug("Error in exec: %s", err.Error())
			os.Exit(10)
		}

		return
	}

	for _, source := range config.Sources {
		err := source.Load()
		if err != nil {
			log.Error("Error loading sources: %s", err)
			os.Exit(1)
			return
		}
	}

	var deps *getpan.DependencyList

	if len(mods) == 0 {
		log.Info("Installing from cpanfile: %s", config.CPANFile)
		d, err := getpan.ParseCPANFile(config.CPANFile)
		if err != nil {
			log.Error("Error parsing cpanfile: %s", err)
			os.Exit(2)
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
		os.Exit(3)
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
		os.Exit(4)
		return
	}

	// FIXME hacky, need a better way of tracking installed deps
	log.Info("Successfully installed %d modules", deps.UniqueInstalled())
}

package main

import (
	"fmt"
	"github.com/companieshouse/gopan/getpan/getpan"
	"strings"
)

func getpan_import(job *ImportJob, msg func(string)) (*getpan.CPANFile, []*getpan.Module) {
	cfg := getpan.DefaultConfig()
	cfg.CacheDir = config.CacheDir

	if len(job.Form.CPANMirror) > 0 {
		msg("Adding CPAN mirror: " + job.Form.CPANMirror)
		cfg.Sources = append(cfg.Sources, getpan.NewSource("CPAN", job.Form.CPANMirror+"/modules/02packages.details.txt.gz", job.Form.CPANMirror))
	}

	defer func(job *ImportJob) {
		job.Complete = true
		msg("GetPAN import complete")
	}(job)

	for _, source := range cfg.Sources {
		err := source.Load()
		if err != nil {
			m := fmt.Sprintf("Error loading sources: %s", err)
			msg(m)
			return nil, nil
		}
	}

	m := "Loaded sources"
	msg(m)

	deps, err := getpan.ParseCPANLines(strings.Split(job.Form.Cpanfile, "\n"))
	if err != nil {
		m := fmt.Sprintf("Error parsing cpanfile: %s", err)
		msg(m)
		return nil, nil
	}

	m = "Parsed cpanfile"
	msg(m)

	err = deps.Resolve()
	if err != nil {
		m := fmt.Sprintf("Error resolving dependencies: %s", err)
		msg(m)
		return deps, nil
	}

	m = "Resolved dependency tree:"
	msg(m)

	modules := make([]*getpan.Module, 0)

	PrintCPANFile(job, deps, 0, msg, &modules)

	return deps, modules
}

func PrintCPANFile(job *ImportJob, deps *getpan.CPANFile, d int, msg func(string), modules *[]*getpan.Module) {
	for _, dep := range deps.Dependencies {
		if dep.Module == nil {
			m := fmt.Sprintf("%s not found", dep.Name)
			msg(m)
			continue
		}
		PrintModDeps(job, dep.Module, d+1, msg, modules)
	}
}

func PrintDeps(job *ImportJob, deps *getpan.DependencyList, d int, msg func(string), modules *[]*getpan.Module) {
	for _, dep := range deps.Dependencies {
		if dep.Module == nil {
			m := fmt.Sprintf("%s not found", dep.Name)
			msg(m)
			continue
		}
		PrintModDeps(job, dep.Module, d+1, msg, modules)
	}
}

func PrintModDeps(job *ImportJob, m *getpan.Module, d int, msg func(string), modules *[]*getpan.Module) {
	ms := fmt.Sprintf("%s (%s): %s", m.Name, m.Version, m.Cached)
	*modules = append(*modules, m)
	msg(ms)

	if m.Deps != nil {
		PrintDeps(job, m.Deps, d+1, msg, modules)
	}
}

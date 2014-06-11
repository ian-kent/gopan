package main

import(
	"github.com/ian-kent/gopan/gopan"
	"github.com/ian-kent/go-log/log"
)

// GoPAN indexes - CPAN, BackPAN etc
var indexes map[string]*gopan.Source

// Maps authors by id (A -> AB -> ABC)
var mapped = make(map[string]map[string]map[string]map[string]*gopan.Author)

// Maps package names (e.g. Mojolicious, Mojolicious::Command etc)
var packages = make(map[string]*PkgSpace)
var idxpackages = make(map[string]map[string]*PkgSpace)

// Maps SmartPAN virtual URLs (a/ab/abc/package.tar.gz) to indexes (BackPAN/CPAN)
var filemap = make(map[string]string)

// Represents a partial namespace (e.g. 'Mojolicious' and 'Mojolicious'->'Command' for Mojolicious::Command)
type PkgSpace struct {
	Namespace string
	Packages []*gopan.PerlPackage
	Children map[string]*PkgSpace
	Parent   *PkgSpace
	Versions map[float64]*gopan.PerlPackage
}

// Returns the full package name
func (p *PkgSpace) FullName() string {
	s := ""
	if p.Parent != nil {
		s = p.Parent.FullName() + "::"
	}
	s += p.Namespace
	return s
}

// Returns the latest version available from any source
func (p *PkgSpace) Version() float64 {
	if len(p.Packages) == 0 {
		return 0
	}

	// FIXME use a sort
	l := float64(0)
	for v, _ := range p.Versions {
		if v > l {
			l = v
		}
	}

	return l
}

// Populates a package namespace, e.g. constructing each part of the namespace
// when passed []string{'Mojolicious','Plugin','PODRenderer'}
func (p *PkgSpace) Populate(parts []string, pkg *gopan.PerlPackage) {
	if len(parts) > 0 {
		if _, ok := p.Children[parts[0]]; !ok {
			p.Children[parts[0]] = &PkgSpace{
				Namespace: parts[0],
				Packages: make([]*gopan.PerlPackage, 0),
				Children: make(map[string]*PkgSpace),
				Parent: p,
				Versions: make(map[float64]*gopan.PerlPackage),
			}
		}
		if len(parts) == 1 {
			p.Children[parts[0]].Packages = append(p.Children[parts[0]].Packages, pkg)
		} else {
			p.Children[parts[0]].Populate(parts[1:], pkg)
		}
		p.Versions[pkg.Package.Version()] = pkg
		log.Trace("Version linked: %f for %s in %s", pkg.Package.Version(), pkg.Name, p.Namespace)
	}
}

type Summary struct {
	Sources int
	Authors int
	Modules int
	Packages int
}
var summary *Summary

package main

import (
	"encoding/json"
	"fmt"
	"github.com/ian-kent/go-log/log"
	"github.com/companieshouse/gopan/getpan/getpan"
	"github.com/companieshouse/gopan/gopan"
	"github.com/companieshouse/gotcha/http"
	"regexp"
	"strings"
)

// parse tail of a 02packages.details.txt line
var packageUrlRe = regexp.MustCompile(`authors/id/\w/\w{2}/\w+/([^\s]+)[-_]v?([\d\._\w]+)(?:-\w+)?(.tar.gz|.tgz)$`)

// FIXME same structs in both smartpan and getpan
type VersionOutput struct {
	Path    string
	URL     string
	Index   string
	Version float64
}

type WhereOutput struct {
	Module   string
	Latest   float64
	Versions []*VersionOutput
}

func where(session *http.Session) {
	module := session.Stash["module"].(string)
	log.Info("Looking for module: %s", module)

	ns := strings.Split(module, "::")
	if _, ok := packages[ns[0]]; !ok {
		log.Info("Top-level namespace [%s] not found", ns[0])
		session.Response.Status = 404
		session.Response.Send()
		return
	}

	mod := packages[ns[0]]

	ns = ns[1:]
	for len(ns) > 0 {
		if _, ok := mod.Children[ns[0]]; !ok {
			log.Info("Child namespace [%s] not found", ns[0])
			session.Response.Status = 404
			session.Response.Send()
			return
		}
		log.Info("Found child namespace [%s]", ns[0])
		mod = mod.Children[ns[0]]
		ns = ns[1:]
	}

	var version string
	if _, ok := session.Stash["version"]; ok {
		version = session.Stash["version"].(string)
		if strings.HasPrefix(version, "v") {
			version = strings.TrimPrefix(version, "v")
		}
		log.Info("Looking for version: %s", version)
	}

	if len(mod.Versions) == 0 {
		log.Info("Module has no versions in index")
		session.Response.Status = 404
		session.Response.Send()
		return
	}

	versions := make([]*VersionOutput, 0)
	lv := float64(0)
	if len(version) > 0 {
		if ">0" == version {
			// Take account of packages that have a version of 'undef'
			for _, pkg := range mod.Packages {
				packageURL := pkg.Package.VirtualURL()
				urlmatches := packageUrlRe.FindStringSubmatch(packageURL)
				if nil == urlmatches {
					log.Info("Version requested [%s] not found", version)
					session.Response.Status = 404
					session.Response.Send()
					return
				}
				ver := gopan.VersionFromString(urlmatches[2])
				log.Info("Found version: %f", ver)
				versions = append(versions, &VersionOutput{
					Index:   pkg.Package.Author.Source.Name,
					URL:     packageURL,
					Path:    pkg.Package.AuthorURL(),
					Version: ver,
				})
				if ver > lv {
					lv = ver
				}
			}
		} else {
			dep, _ := getpan.DependencyFromString(module, version)
			for _, md := range mod.Versions {
				var sver string = md.Version
				if `undef` == md.Version {
					sver = fmt.Sprintf("%.2f", md.Package.Version())
				}
				log.Info("Matching [%s] against derived version [%s] (md.Version [%s], md.Package.Version [%f])", dep.Version, sver, md.Version, md.Package.Version())
				if dep.MatchesVersion(sver) {
					vout := &VersionOutput{
						Index:   md.Package.Author.Source.Name,
						URL:     md.Package.VirtualURL(),
						Path:    md.Package.AuthorURL(),
						Version: gopan.VersionFromString(sver),
					}
					versions = append(versions, vout)
					ver := gopan.VersionFromString(sver)
					if ver > lv {
						lv = ver
					}
				}
			}
		}

		if len(versions) == 0 {
			log.Info("Version requested [%s] not found", version)
			session.Response.Status = 404
			session.Response.Send()
			return
		}
	} else {
		for v, pkg := range mod.Versions {
			log.Info("Found version: %f", v)
			versions = append(versions, &VersionOutput{
				Index:   pkg.Package.Author.Source.Name,
				URL:     pkg.Package.VirtualURL(),
				Path:    pkg.Package.AuthorURL(),
				Version: gopan.VersionFromString(pkg.Version),
			})
			if v > lv {
				lv = v
			}
		}
	}

	session.Response.Headers.Set("Content-Type", "application/json")

	o := &WhereOutput{
		Module:   mod.FullName(),
		Latest:   lv,
		Versions: versions,
	}

	b, err := json.MarshalIndent(o, "", "  ")

	log.Info("Output: %s", string(b))

	if err != nil {
		log.Error("Failed encoding JSON: %s", err.Error())
		session.Response.Status = 500
		session.Response.Send()
		return
	}

	session.Response.Status = 200
	session.Response.Write(b)
}

package main

import (
	"github.com/ian-kent/gotcha/http"
	"github.com/ian-kent/go-log/log"
	"github.com/ian-kent/gopan/getpan/getpan"
	"github.com/ian-kent/gopan/gopan"
	"strings"
	"encoding/json"
)

// FIXME same structs in both smartpan and getpan
type VersionOutput struct {
	Path string
	URL string
	Index string
	Version float64
}

type WhereOutput struct {
	Module string
	Latest float64	
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
		dep, _ := getpan.DependencyFromString(module, version)
		for v, md := range mod.Versions {
			log.Info("Matching [%s] against md.Version [%s], md.Package.Version [%f]", dep.Version, md.Version, md.Package.Version())
			if dep.MatchesVersion(md.Version) {
				vout := &VersionOutput{
					Index: md.Package.Author.Source.Name,
					URL: md.Package.VirtualURL(),
					Path: md.Package.AuthorURL(),
					Version: gopan.VersionFromString(md.Version),
				}
				versions = append(versions, vout)
				if v > lv { lv = v }
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
				Index: pkg.Package.Author.Source.Name,
				URL: pkg.Package.VirtualURL(),
				Path: pkg.Package.AuthorURL(),
				Version: gopan.VersionFromString(pkg.Version),
			})
			if v > lv { lv = v }
		}
	}

	session.Response.Headers.Set("Content-Type", "application/json")
	
	o := &WhereOutput{
		Module: mod.FullName(),
		Latest: lv,
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

package main

import (
	"fmt"
	"github.com/ian-kent/go-log/log"
	"github.com/ian-kent/gotcha/http"
	"strconv"
)

func writepkgindex(session *http.Session, pkgspace *PkgSpace) {
	if len(pkgspace.Packages) > 0 {
		latest := pkgspace.Packages[0]
		if len(latest.Version) == 0 {
			latest.Version = "undef"
		}
		session.Response.WriteText(fmt.Sprintf("%-40s %-10s %s\n", pkgspace.FullName(), latest.Version, latest.Package.AuthorURL()))
	}

	if len(pkgspace.Children) > 0 {
		for _, ps := range pkgspace.Children {
			writepkgindex(session, ps)
		}
	}
}

func pkgindex(session *http.Session) {
	if _, ok := session.Stash["repo"]; !ok {
		session.RenderNotFound()
		return
	}

	repo := session.Stash["repo"].(string)

	if _, ok := indexes[repo]; !ok && repo != "SmartPAN" {
		session.RenderNotFound()
		return
	}

	if g, ok := session.Stash["gz"]; ok {
		if len(g.(string)) > 0 {
			// cheat and hijack gotchas gzip support
			session.Response.Headers.Set("Content-Type", "application/gzip")
			session.Response.Send()
			session.Response.Gzip()
			session.Response.Headers.Remove("Content-Encoding")
			log.Debug("Using gzip")
		}
	}

	session.Response.WriteText("File:         02packages.details.txt\n")
	session.Response.WriteText("Description:  Package names found in directory " + repo + "/authors/id\n")
	session.Response.WriteText("Columns:      package name, version, path\n")
	session.Response.WriteText("Written-By:   SmartPAN (from GoPAN)\n")
	session.Response.WriteText("Line-Count:   " + strconv.Itoa(summary.Packages) + "\n") // FIXME wrong count
	session.Response.WriteText("\n")

	if repo == "SmartPAN" {
		for _, pkg := range packages {
			writepkgindex(session, pkg)
		}
	} else {
		for _, pkg := range idxpackages[repo] {
			writepkgindex(session, pkg)
		}
	}
}

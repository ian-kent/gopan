package main

import (
	"github.com/companieshouse/gopan/gopan"
	"github.com/companieshouse/gotcha/http"
	"github.com/ian-kent/go-log/log"
	"html/template"
	"os"
	"strings"
)

func delete_file(session *http.Session) {
	session.Stash["Title"] = "Delete file"
	html, _ := session.RenderTemplate("delete.html")

	repo := session.Stash["repo"].(string)
	file := session.Stash["file"].(string)
	auth1 := session.Stash["auth1"].(string)
	auth2 := session.Stash["auth2"].(string)
	auth3 := session.Stash["auth3"].(string)

	fname := config.CacheDir + "/" + repo + "/" + auth1 + "/" + auth2 + "/" + auth3 + "/" + file

	if _, err := os.Stat(fname); err != nil {
		session.RenderNotFound()
		return
	}

	// Remove file from indexes
	for f, _ := range indexes {
		if _, ok := indexes[f][repo]; !ok {
			continue
		}
		if _, ok := indexes[f][repo].Authors[auth3]; !ok {
			continue
		}
		if _, ok := indexes[f][repo].Authors[auth3].Packages[file]; !ok {
			continue
		}
		log.Debug("Removing from index: %s", repo)

		pkg := indexes[f][repo].Authors[auth3].Packages[file]
		delete(indexes[f][repo].Authors[auth3].Packages, file)

		if len(indexes[f][repo].Authors[auth3].Packages) == 0 {
			log.Debug("Removing author")
			delete(indexes[f][repo].Authors, auth3)
		}

		if len(indexes[f][repo].Authors) == 0 {
			log.Debug("Removing index")
			delete(indexes[f], repo)
		}

		if auth, ok := mapped[repo][auth1][auth2][auth3]; ok {
			if len(auth.Packages) == 0 {
				log.Debug("Removing author from mapped index")
				delete(mapped[repo][auth1][auth2], auth3)
				delete(mapped[repo]["*"][auth2], auth3)
				delete(mapped[repo][auth1]["**"], auth3)
				delete(mapped[repo]["*"]["**"], auth3)
			}

			if len(mapped[repo][auth1][auth2]) == 0 {
				log.Debug("Removing auth1/auth2 from mapped index")
				delete(mapped[repo][auth1], auth2)
			}

			if len(mapped[repo]["*"][auth2]) == 0 {
				log.Debug("Removing author **/auth2 from mapped index")
				delete(mapped[repo][auth1], auth2)
			}
			if len(mapped[repo][auth1]["**"]) == 0 {
				log.Debug("Removing author auth1/** from mapped index")
				delete(mapped[repo][auth1], auth2)
			}
			if len(mapped[repo]["*"]["**"]) == 0 {
				log.Debug("Removing author */** from mapped index")
				delete(mapped[repo][auth1], auth2)
			}
			if len(mapped[repo]["*"]) == 0 {
				log.Debug("Removing author * from mapped index")
				delete(mapped[repo][auth1], auth2)
			}

			if len(mapped[repo][auth1]) == 1 {
				log.Debug("Removing author auth1 from mapped index")
				delete(mapped[repo], auth1)
			}

			if len(mapped[repo]) == 1 {
				log.Debug("Removing repo from mapped index")
				delete(mapped, repo)
			}
		}

		for _, prov := range pkg.Provides {
			parts := strings.Split(prov.Name, "::")
			// TODO remove from packages/idxpackages
			if ctx, ok := packages[parts[0]]; ok {
				parts = parts[1:]
				for len(parts) > 0 {
					if c, ok := ctx.Children[parts[0]]; ok {
						ctx = c
					} else {
						log.Debug("Package not found in packages: %s", parts)
						break
					}
					parts = parts[1:]
				}
				if len(parts) == 0 {
					for ctx != nil {
						for pi, p := range ctx.Packages {
							if p.Package == pkg {
								log.Debug("Removing package from packages: %s", ctx.FullName())
								ctx.Packages = append(ctx.Packages[:pi], ctx.Packages[pi+1:]...)
								break
							}
						}
						if len(ctx.Packages) == 0 {
							log.Debug("Removing PkgSpace from packages: %s", ctx.FullName())
							if ctx.Parent == nil {
								delete(packages, ctx.Namespace)
							} else {
								delete(ctx.Parent.Children, ctx.Namespace)
							}
						}

						ctx = ctx.Parent
					}
				}
			}
			parts = strings.Split(prov.Name, "::")
			if _, ok := idxpackages[repo]; ok {
				if ctx, ok := idxpackages[repo][parts[0]]; ok {
					parts = parts[1:]
					for len(parts) > 0 {
						if c, ok := ctx.Children[parts[0]]; ok {
							ctx = c
						} else {
							log.Debug("PkgSpace not found in idxpackages")
							break
						}
						parts = parts[1:]
					}
					if len(parts) == 0 {
						for ctx != nil {
							for pi, p := range ctx.Packages {
								if p.Package == pkg {
									log.Debug("Removing package from idxpackages")
									ctx.Packages = append(ctx.Packages[:pi], ctx.Packages[pi+1:]...)
									break
								}
							}
							if len(ctx.Packages) == 0 {
								log.Debug("Removing PkgSpace from idxpackages: %s", ctx.FullName())
								if ctx.Parent == nil {
									delete(idxpackages, ctx.Namespace)
								} else {
									delete(ctx.Parent.Children, ctx.Namespace)
								}
							}

							ctx = ctx.Parent
						}
					}
				}
			}
		}

		if _, ok := filemap[auth1+"/"+auth2+"/"+auth3+"/"+file]; ok {
			log.Debug("Removing file from filemap")
			// FIXME filemap should be map[string][]string, so we know if
			// the file exists in multiple indexes
			delete(filemap, auth1+"/"+auth2+"/"+auth3+"/"+file)
		}

		// write remove to index
		gopan.RemoveModule(config.CacheDir+"/"+config.Index, pkg.Author.Source, pkg.Author, pkg)
	}

	log.Debug("Removing file from gopancache: %s", fname)
	// TODO move file deletion to shared gopan package
	err := os.Remove(fname)
	if err != nil {
		log.Error("Error removing file: %s", err)
	}

	// TODO maybe clean up author tree (is this smartpans responsibility?)

	nsrc, nauth, npkg, nprov := gopan.CountIndex(indexes)
	// TODO should probably be in the index - needs to udpate when index changes
	summary = &Summary{nsrc, nauth, npkg, nprov}

	session.Stash["Page"] = "Browse"
	session.Stash["Content"] = template.HTML(html)
	session.Render("layout.html")
}

//			//pkg := indexes[config.Index][reponame].Authors[auth].Packages[fn]
//			msg(" | Adding packages to index")
//			if _, ok := idxpackages[reponame]; !ok {
//				idxpackages[reponame] = make(map[string]*PkgSpace)
//			}
//			filemap[pkg.AuthorURL()] = reponame
//			for _, prov := range pkg.Provides {
//				parts := strings.Split(prov.Name, "::")
//				if _, ok := packages[parts[0]]; !ok {
//					packages[parts[0]] = &PkgSpace{
//						Namespace: parts[0],
//						Packages:  make([]*gopan.PerlPackage, 0),
//						Children:  make(map[string]*PkgSpace),
//						Parent:    nil,
//						Versions:  make(map[float64]*gopan.PerlPackage),
//					}
//				}
//				if _, ok := idxpackages[reponame][parts[0]]; !ok {
//					idxpackages[reponame][parts[0]] = &PkgSpace{
//						Namespace: parts[0],
//						Packages:  make([]*gopan.PerlPackage, 0),
//						Children:  make(map[string]*PkgSpace),
//						Parent:    nil,
//						Versions:  make(map[float64]*gopan.PerlPackage),
//					}
//				}
//				if len(parts) == 1 {
//					packages[parts[0]].Packages = append(packages[parts[0]].Packages, prov)
//					packages[parts[0]].Versions[gopan.VersionFromString(prov.Version)] = prov
//					idxpackages[reponame][parts[0]].Packages = append(idxpackages[reponame][parts[0]].Packages, prov)
//					idxpackages[reponame][parts[0]].Versions[gopan.VersionFromString(prov.Version)] = prov
//				} else {
//					packages[parts[0]].Populate(parts[1:], prov)
//					idxpackages[reponame][parts[0]].Populate(parts[1:], prov)
//				}
//			}
//
//			msg(" | Writing to index file")
//			gopan.AppendToIndex(config.CacheDir+"/"+config.Index, indexes[config.Index][reponame], indexes[config.Index][reponame].Authors[auth], indexes[config.Index][reponame].Authors[auth].Packages[fn])
//		}
//
//		msg(" | Imported module")
//	}

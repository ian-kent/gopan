package main

import (
	"github.com/ian-kent/go-log/log"
	"github.com/ian-kent/gotcha/http"
	"html/template"
	"strings"
)

// Get top level repo list
func toplevelRepo() map[string]map[string]string {
	dirs := make(map[string]map[string]string, 0)
	for pos, _ := range mapped {
		dirs[pos] = map[string]string{
			"Name": pos,
			"Path": "/" + pos,
		}
	}
	dirs["SmartPAN"] = map[string]string{
		"Name": "SmartPAN",
		"Path": "/SmartPAN",
	}
	return dirs
}

// modules/authors
func tlRepo1(idx string) (map[string]map[string]string, map[string]map[string]string) {
	dirs := make(map[string]map[string]string, 0)
	files := make(map[string]map[string]string, 0)

	dirs["modules"] = map[string]string{
		"Name": "modules",
		"Path": "/" + idx + "/modules",
	}
	dirs["authors"] = map[string]string{
		"Name": "authors",
		"Path": "/" + idx + "/authors",
	}
	return dirs, files
}

// 02packages/id
func tlRepo2(idx string, tl string) (map[string]map[string]string, map[string]map[string]string) {
	dirs := make(map[string]map[string]string, 0)
	files := make(map[string]map[string]string, 0)

	if tl == "modules" {
		files["02packages.details.txt"] = map[string]string{
			"Name":  "02packages.details.txt",
			"Path":  "/" + idx + "/modules/02packages.details.txt",
			"Glyph": "compressed",
		}
		files["02packages.details.txt.gz"] = map[string]string{
			"Name":  "02packages.details.txt.gz",
			"Path":  "/" + idx + "/modules/02packages.details.txt.gz",
			"Glyph": "compressed",
		}
		for k, _ := range packages {
			files[k] = map[string]string{
				"Name":  k,
				"Path":  "/" + idx + "/modules/" + k,
				"Glyph": "briefcase",
			}
		}
	} else if tl == "authors" {
		dirs["id"] = map[string]string{
			"Name": "id",
			"Path": "/" + idx + "/authors/id",
		}
	}
	return dirs, files
}

// Get list of author first letters
func tlAuthor1(idx string) map[string]map[string]string {
	dirs := make(map[string]map[string]string, 0)

	if idx == "SmartPAN" {
		for idx, _ := range mapped {
			for pos, _ := range mapped[idx] {
				dirs[pos] = map[string]string{
					"Name": pos,
					"Path": "/SmartPAN/authors/id/" + pos,
				}
			}
		}
	} else {
		for pos, _ := range mapped[idx] {
			dirs[pos] = map[string]string{
				"Name": pos,
				"Path": "/" + idx + "/authors/id/" + pos,
			}
		}
	}

	dirs["*"] = map[string]string{
		"Name": "*",
		"Path": "/" + idx + "/authors/id/*",
	}

	return dirs
}

// Get list of author second letters
func tlAuthor2(idx string, fl string) map[string]map[string]string {
	dirs := make(map[string]map[string]string, 0)

	if idx == "SmartPAN" {
		for idx, _ := range mapped {
			for pos, _ := range mapped[idx][fl] {
				dirs[pos] = map[string]string{
					"Name": pos,
					"Path": "/SmartPAN/authors/id/" + fl + "/" + pos,
				}
			}
		}
	} else {
		for pos, _ := range mapped[idx][fl] {
			dirs[pos] = map[string]string{
				"Name": pos,
				"Path": "/" + idx + "/authors/id/" + fl + "/" + pos,
			}
		}
	}

	dirs["**"] = map[string]string{
		"Name": "**",
		"Path": "/" + idx + "/authors/id/" + fl + "/**",
	}

	return dirs
}

// Get list of author second letters
func tlAuthor3(idx string, fl string, sl string) map[string]map[string]string {
	dirs := make(map[string]map[string]string, 0)

	if idx == "SmartPAN" {
		for idx, _ := range mapped {
			for pos, _ := range mapped[idx][fl][sl] {
				dirs[pos] = map[string]string{
					"Name": pos,
					"Path": "/SmartPAN/authors/id/" + fl + "/" + sl + "/" + pos,
				}
			}
		}
	} else {
		for pos, _ := range mapped[idx][fl][sl] {
			dirs[pos] = map[string]string{
				"Name": pos,
				"Path": "/" + idx + "/authors/id/" + fl + "/" + sl + "/" + pos,
			}
		}
	}

	dirs["***"] = map[string]string{
		"Name": "***",
		"Path": "/" + idx + "/authors/id/" + fl + "/" + sl + "/***",
	}

	return dirs
}

func tlModuleList(idx string, fl string, sl string, author string) map[string]map[string]string {
	files := make(map[string]map[string]string, 0)

	if idx == "SmartPAN" {
		for idx, _ := range mapped {
			if author == "***" {
				for author, auth := range mapped[idx][fl][sl] {
					for pos, _ := range auth.Packages {
						files[pos] = map[string]string{
							"Name":  pos,
							"Path":  "/" + idx + "/authors/id/" + author[:1] + "/" + author[:2] + "/" + author + "/" + pos,
							"Glyph": "compressed",
						}
					}
				}
			} else {
				if auth, ok := mapped[idx][fl][sl][author]; ok {
					for pos, _ := range auth.Packages {
						files[pos] = map[string]string{
							"Name":  pos,
							"Path":  "/" + idx + "/authors/id/" + author[:1] + "/" + author[:2] + "/" + author + "/" + pos,
							"Glyph": "compressed",
						}
					}
				}
			}
		}
	} else {
		if author == "***" {
			for author, _ := range mapped[idx][fl][sl] {
				for pos, _ := range mapped[idx][fl][sl][author].Packages {
					files[pos] = map[string]string{
						"Name":  pos,
						"Path":  "/" + idx + "/authors/id/" + author[:1] + "/" + author[:2] + "/" + author + "/" + pos,
						"Glyph": "compressed",
					}
				}
			}
		} else {
			for pos, _ := range mapped[idx][fl][sl][author].Packages {
				files[pos] = map[string]string{
					"Name":  pos,
					"Path":  "/" + idx + "/authors/id/" + author[:1] + "/" + author[:2] + "/" + author + "/" + pos,
					"Glyph": "compressed",
				}
			}
		}
	}

	return files
}

func browse(session *http.Session) {
	session.Stash["Title"] = "SmartPAN"

	path := ""
	repo := ""
	itype := ""
	fpath := ""
	if r, ok := session.Stash["repo"]; ok {
		repo = r.(string)
		fpath += repo + "/"

		for fname, _ := range indexes {
			if _, ok := indexes[fname][repo]; !ok && repo != "SmartPAN" {
				session.RenderNotFound()
				return
			}
		}
	}
	if i, ok := session.Stash["type"]; ok {
		itype = i.(string)
		fpath += itype + "/"
	}
	if p, ok := session.Stash["path"]; ok {
		path = p.(string)
		fpath += path + "/"
	}

	fpath = strings.TrimSuffix(fpath, "/")
	session.Stash["path"] = fpath

	bits := strings.Split(path, "/")
	fbits := strings.Split(fpath, "/")
	dirs := make(map[string]map[string]string, 0)
	files := make(map[string]map[string]string, 0)

	log.Info("Path: %s, Bits: %d", path, len(bits))

	if repo == "" {
		dirs = toplevelRepo()
	} else if itype == "" {
		dirs, files = tlRepo1(repo)
	} else {
		switch itype {
		case "authors":
			if len(path) == 0 {
				dirs, files = tlRepo2(repo, itype)
			} else {
				switch len(bits) {
				case 1:
					log.Info("tlAuthor1")
					dirs = tlAuthor1(repo)
				case 2:
					log.Info("tlAuthor2: %s", bits[1])
					dirs = tlAuthor2(repo, bits[1])
				case 3:
					log.Info("tlAuthor3: %s, %s", bits[1], bits[2])
					dirs = tlAuthor3(repo, bits[1], bits[2])
				case 4:
					log.Info("tlModuleList: %s, %s", repo, bits[1], bits[2], bits[3])
					files = tlModuleList(repo, bits[1], bits[2], bits[3])
				default:
					log.Info("Invalid path - rendering not found")
					session.RenderNotFound()
				}
			}
		case "modules":
			if path == "" {
				dirs, files = tlRepo2(repo, itype)
			} else {
				if repo == "SmartPAN" {
					rbits := bits[1:]
					ctx := packages[bits[0]]
					log.Info("Starting with context: %s", ctx.Namespace)
					for len(rbits) > 0 {
						ctx = ctx.Children[rbits[0]]
						rbits = rbits[1:]
					}
					log.Info("Stashing package context: %s", ctx.Namespace)
					session.Stash["Package"] = ctx
					for ns, _ := range ctx.Children {
						files[ns] = map[string]string{
							"Name": ns,
							"Path": "/" + repo + "/modules/" + path + "/" + ns,
						}
					}
				} else {
					rbits := bits[1:]
					ctx := idxpackages[repo][bits[0]]
					log.Info("Starting with context: %s", ctx.Namespace)
					for len(rbits) > 0 {
						ctx = ctx.Children[rbits[0]]
						rbits = rbits[1:]
					}
					session.Stash["Package"] = ctx
					log.Info("Stashing package context: %s", ctx.Namespace)
					for ns, _ := range ctx.Children {
						files[ns] = map[string]string{
							"Name": ns,
							"Path": "/" + repo + "/modules/" + path + "/" + ns,
						}
					}
				}
			}
		default:
			session.RenderNotFound()
		}
	}

	session.Stash["Dirs"] = dirs
	session.Stash["Files"] = files

	pp := make([]map[string]string, 0)
	cp := ""
	for _, b := range fbits {
		cp = cp + "/" + b
		pp = append(pp, map[string]string{
			"Name": b,
			"Path": cp,
		})
	}

	if len(pp) > 0 && len(pp[0]["Name"]) > 0 {
		session.Stash["PathBits"] = pp
	}

	html, _ := session.RenderTemplate("browse.html")

	session.Stash["Page"] = "Browse"
	session.Stash["Content"] = template.HTML(html)
	session.Render("layout.html")
}

package gopan

type Package struct {
	Author *Author
	Name   string
	URL    string
	Provides map[string]*PerlPackage
}

type PerlPackage struct {
	Package *Package
	Name string
	Version string
	File string
}

func (p *Package) String() string {
	return p.Name
}

func (p *Package) VirtualURL() string {
	return p.Author.Source.Name + "/authors/id/" + p.AuthorURL()
}

func (p *Package) AuthorURL() string {
	return p.Author.Name[:1] + "/" + p.Author.Name[:2] + "/" + p.Author.Name + "/" + p.Name
}


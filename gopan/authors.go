package gopan

type Author struct {
	Source   *Source
	Name     string
	Packages map[string]*Package
	URL      string
}

func (a *Author) String() string {
	return a.Name
}

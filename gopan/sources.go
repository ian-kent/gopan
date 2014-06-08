package gopan

type Source struct {
	Name    string
	Authors map[string]*Author
	URL     string
}

func (s *Source) String() string {
	return s.Name
}

func CPANSource() *Source {
	return &Source{
		Name:    "CPAN",
		URL:     "http://www.cpan.org/authors/id",
		Authors: make(map[string]*Author, 0),
	}
}

func BackPANSource() *Source {
	return &Source{
		Name:    "BackPAN",
		URL:     "http://backpan.cpan.org/authors/id",
		Authors: make(map[string]*Author, 0),
	}
}

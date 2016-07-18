package getpan

import (
	"flag"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/companieshouse/gopan/gopan"
	"github.com/ian-kent/go-log/layout"
	"github.com/ian-kent/go-log/log"
)

var config *Config

type TestConfig struct {
	Global  bool
	Modules map[string]bool
}

type Config struct {
	Sources     []*Source
	Test        *TestConfig
	NoDepdump   bool
	NoInstall   bool
	CPANFile    string
	LogLevel    string
	CPUs        int
	CacheDir    string
	InstallDir  string
	MetaCPAN    bool
	MetaSources []*Source
}

type sortSources []*Source

func (s sortSources) Len() int           { return len(s) }
func (s sortSources) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s sortSources) Less(i, j int) bool { return s[i].Priority < s[j].Priority }

func (c *Config) Dump() {
	log.Debug("GoPAN configuration:")

	log.Debug("=> Sources")
	for _, s := range c.Sources {
		log.Debug(" - %s", s)
	}

	log.Debug("=> Test")
	if c.Test.Global {
		log.Debug(" - Global tests are enabled")
	} else {
		log.Debug(" - Global tests are disabled")
		for m, _ := range c.Test.Modules {
			log.Debug(" - %s tests are enabled", m)
		}
	}

	log.Debug("=> NoInstall: %t", c.NoInstall)
	log.Debug("=> MetaCPAN: %t", c.MetaCPAN)
	log.Debug("=> NoDepdump: %t", c.NoDepdump)
	log.Debug("=> CPANFile: %s", c.CPANFile)
	log.Debug("=> LogLevel: %s", c.LogLevel)
	log.Debug("=> Parallelism: %d", c.CPUs)
	log.Debug("=> CacheDir: %s", c.CacheDir)
	log.Debug("=> InstallDir: %s", c.InstallDir)
}

func DefaultSources(cpan bool, backpan bool) []*Source {
	// TODO option to disable default CPAN/BackPAN sources
	sources := make([]*Source, 0)
	if cpan {
		sources = append(sources, NewSource("CPAN", "http://www.cpan.org/modules/02packages.details.txt.gz", "http://www.cpan.org"))
	}
	if backpan {
		sources = append(sources, NewSource("BackPAN", "http://gitpan.integra.net/backpan-index.gz", "http://backpan.perl.org"))
	}
	return sources
}

func DefaultConfig() *Config {
	config = &Config{
		Sources: DefaultSources(true, true),
		Test: &TestConfig{
			Global:  false,
			Modules: make(map[string]bool),
		},
		NoDepdump:  false,
		NoInstall:  false,
		LogLevel:   "INFO",
		CPUs:       runtime.NumCPU(),
		CacheDir:   "./.gopancache",
		InstallDir: "./local",
		MetaCPAN:   false,
	}
	return config
}

func Configure() *Config {
	conf := DefaultConfig()

	cpan := make([]string, 0)
	flag.Var((*gopan.AppendSliceValue)(&cpan), "cpan", "A CPAN mirror (can be specified multiple times)")

	backpan := make([]string, 0)
	flag.Var((*gopan.AppendSliceValue)(&backpan), "backpan", "A BackPAN mirror (can be specified multiple times)")

	smart := make([]string, 0)
	flag.Var((*gopan.AppendSliceValue)(&smart), "smart", "A SmartPAN mirror (can be specified multiple times)")

	test := make([]string, 0)
	flag.Var((*gopan.AppendSliceValue)(&test), "test", "A module to install with testing (can be specified multiple times)")

	tests := false
	flag.BoolVar(&tests, "tests", false, "Enables all tests during installation phase")

	nocpan := false
	flag.BoolVar(&nocpan, "nocpan", false, "Disables the default CPAN source")

	nobackpan := false
	flag.BoolVar(&nobackpan, "nobackpan", false, "Disables the default BackCPAN source")

	cpanfile := "cpanfile"
	flag.StringVar(&cpanfile, "cpanfile", "cpanfile", "The cpanfile to install")

	numcpus := runtime.NumCPU()
	flag.IntVar(&numcpus, "cpus", numcpus, "The number of CPUs to use, defaults to "+strconv.Itoa(numcpus)+" for your environment")

	nodepdump := false
	flag.BoolVar(&nodepdump, "nodepdump", nodepdump, "Disables dumping resolved dependencies listing")

	noinstall := false
	flag.BoolVar(&noinstall, "noinstall", noinstall, "Disables installation phase")

	metacpan := false
	flag.BoolVar(&metacpan, "metacpan", metacpan, "Enable resolving source via MetaCPAN")

	loglevel := "INFO"
	flag.StringVar(&loglevel, "loglevel", loglevel, "Log output level (ERROR, INFO, WARN, DEBUG, TRACE)")

	loglayout := "[%d] [%p] %m"
	flag.StringVar(&loglayout, "loglayout", loglayout, "Log layout (for github.com/ian-kent/go-log pattern layout)")

	cachedir := "./.gopancache"
	flag.StringVar(&cachedir, "cachedir", cachedir, "Cache directory for CPAN modules")

	installdir := "./local"
	flag.StringVar(&installdir, "installdir", installdir, "Install directory for CPAN modules")

	flag.Parse()

	if nocpan || nobackpan {
		conf.Sources = DefaultSources(!nocpan, !nobackpan)
	}

	// parse cpan mirrors
	for _, url := range cpan {
		mirror := strings.TrimSuffix(url, "/")
		m := NewSource("CPAN", mirror+"/modules/02packages.details.txt.gz", mirror)
		conf.Sources = append(conf.Sources, m)
	}

	// parse backpan mirrors
	for _, url := range backpan {
		mirror := strings.TrimSuffix(url, "/")
		m := NewSource("BackPAN", mirror+"/backpan-index", mirror) // FIXME
		conf.Sources = append(conf.Sources, m)
	}

	// parse smartpan mirrors
	for _, url := range smart {
		mirror := strings.TrimSuffix(url, "/")
		m := NewSource("SmartPAN", mirror, mirror)
		conf.Sources = append(conf.Sources, m)
	}

	// resolve via metacpan
	conf.MetaCPAN = metacpan
	if metacpan {
		m := NewSource("MetaCPAN", "", "")
		conf.Sources = append(conf.Sources, m)
		c := NewMetaSource("cpan", "", "http://www.cpan.org", m.ModuleList)
		conf.MetaSources = append(conf.MetaSources, c)
		b := NewMetaSource("backpan", "", "http://backpan.perl.org", m.ModuleList)
		conf.MetaSources = append(conf.MetaSources, b)
	}

	sort.Sort(sortSources(conf.Sources))

	// parse notest
	for _, n := range test {
		conf.Test.Modules[n] = true
	}
	conf.Test.Global = tests

	// parse cpanfile
	conf.CPANFile = cpanfile

	// numcpus
	conf.CPUs = numcpus

	// nodepdeump
	conf.NoDepdump = nodepdump

	// noinstall
	conf.NoInstall = noinstall

	// install dir
	conf.InstallDir = installdir

	// cache dir
	conf.CacheDir = cachedir

	// log level and layout
	log.Logger().Appender().SetLayout(layout.Pattern(loglayout))
	log.Logger().SetLevel(log.Stol(loglevel))

	// create cache dir
	os.MkdirAll(conf.CacheDir, 0777)

	config = conf

	return conf
}

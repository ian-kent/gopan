package getpan

import (
	"flag"
	"github.com/ian-kent/go-log/layout"
	"github.com/ian-kent/go-log/log"
	"github.com/ian-kent/gopan/gopan"
	"os"
	"runtime"
	"strconv"
)

var config *Config

type NoTestConfig struct {
	Global  bool
	Modules map[string]bool
}

type Config struct {
	Sources   []*Source
	NoTest    *NoTestConfig
	NoInstall bool
	CPANFile  string
	LogLevel  string
	CPUs      int
	CacheDir  string
}

func (c *Config) Dump() {
	log.Info("GoPAN configuration:")

	log.Info("=> Sources")
	for _, s := range c.Sources {
		log.Info(" - %s", s)
	}

	log.Info("=> NoTest")
	if c.NoTest.Global {
		log.Info(" - ALL tests are disabled")
	} else {
		for m, _ := range c.NoTest.Modules {
			log.Info(" - %s", m)
		}
	}

	log.Info("=> NoInstall: %t", c.NoInstall)
	log.Info("=> CPANFile: %s", c.CPANFile)
	log.Info("=> LogLevel: %s", c.LogLevel)
	log.Info("=> Parallelism: %d", c.CPUs)
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
		NoTest: &NoTestConfig{
			Global:  false,
			Modules: make(map[string]bool),
		},
		NoInstall: false,
		LogLevel:  "INFO",
		CPUs:      runtime.NumCPU(),
		CacheDir:  ".gopancache",
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

	notest := make([]string, 0)
	flag.Var((*gopan.AppendSliceValue)(&notest), "notest", "A module to install without testing (can be specified multiple times)")

	nevertest := false
	flag.BoolVar(&nevertest, "nevertest", false, "Disables all tests during installation phase")

	nocpan := false
	flag.BoolVar(&nocpan, "nocpan", false, "Disables the default CPAN source")

	nobackpan := false
	flag.BoolVar(&nobackpan, "nobackpan", false, "Disables the default BackCPAN source")

	cpanfile := "cpanfile"
	flag.StringVar(&cpanfile, "cpanfile", "cpanfile", "The cpanfile to install")

	numcpus := runtime.NumCPU()
	flag.IntVar(&numcpus, "cpus", numcpus, "The number of CPUs to use, defaults to "+strconv.Itoa(numcpus)+" for your environment")

	noinstall := false
	flag.BoolVar(&noinstall, "noinstall", noinstall, "Disables installation phase")

	loglevel := "INFO"
	flag.StringVar(&loglevel, "loglevel", loglevel, "Log output level (ERROR, INFO, WARN, DEBUG, TRACE)")

	loglayout := "[%d] [%p] %m"
	flag.StringVar(&loglayout, "loglayout", loglayout, "Log layout (for github.com/ian-kent/go-log pattern layout)")

	flag.Parse()

	log.Info("GoPAN configuration:")

	if nocpan || nobackpan {
		conf.Sources = DefaultSources(!nocpan, !nobackpan)
	}

	// parse cpan mirrors
	for _, mirror := range cpan {
		m := NewSource("CPAN", mirror+"/modules/02packages.details.txt.gz", mirror)
		conf.Sources = append(conf.Sources, m)
	}

	// parse backpan mirrors
	for _, mirror := range backpan {
		m := NewSource("BackPAN", mirror+"/backpan-index", mirror) // FIXME
		conf.Sources = append(conf.Sources, m)
	}

	// parse smartpan mirrors
	for _, mirror := range smart {
		m := NewSource("SmartPAN", mirror, mirror)
		conf.Sources = append(conf.Sources, m)
	}

	// parse notest
	for _, n := range notest {
		conf.NoTest.Modules[n] = true
	}
	conf.NoTest.Global = nevertest

	// parse cpanfile
	conf.CPANFile = cpanfile

	// numcpus
	conf.CPUs = numcpus

	// noinstall
	conf.NoInstall = noinstall

	// log level and layout
	log.Logger().Appender().SetLayout(layout.Pattern(loglayout))
	log.Logger().SetLevel(log.Stol(loglevel))

	// create cache dir
	os.MkdirAll(conf.CacheDir, 0777)

	config = conf

	return conf
}

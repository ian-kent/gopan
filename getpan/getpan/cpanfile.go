package getpan

import (
	"github.com/ian-kent/go-log/log"
	"io/ioutil"
	"regexp"
	"strings"
)

var re = regexp.MustCompile("^\\s*requires\\s+['\"]([^'\"]+)['\"](,\\s+['\"]([^'\"]+)['\"])?;\\s*(#.*)?")

type CPANFile struct {
	DependencyList
}

func ParseCPANLine(line string) (*Dependency, error) {
	if len(line) == 0 {
		return nil, nil
	}

	matches := re.FindStringSubmatch(line)
	if len(matches) == 0 {
		log.Trace("Unable to parse line: %s", line)
		return nil, nil
	}

	module := matches[1]
	version := matches[3]
	comment := matches[4]

	dependency, err := DependencyFromString(module, version)

	if strings.HasPrefix(strings.Trim(comment, " "), "# REQS: ") {
		comment = strings.TrimPrefix(strings.Trim(comment, " "), "# REQS: ")
		log.Trace("Found additional dependencies: %s", comment)
		for _, req := range strings.Split(comment, ";") {
			req = strings.Trim(req, " ")
			bits := strings.Split(req, "-")
			new_dep, err := DependencyFromString(bits[0], bits[1])
			if err != nil {
				log.Error("Error parsing REQS dependency: %s", req)
				continue
			}
			log.Trace("Added dependency: %s", new_dep)
			dependency.Additional = append(dependency.Additional, new_dep)
		}
	}

	if err != nil {
		return nil, err
	}

	log.Info("%s (%s %s)", module, dependency.Modifier, dependency.Version)

	return dependency, err
}

func ParseCPANFile(file string) (*CPANFile, error) {
	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(bytes), "\n")

	return ParseCPANLines(lines)
}

func ParseCPANLines(lines []string) (*CPANFile, error) {
	cpanfile := &CPANFile{}

	for _, l := range lines {
		if len(l) == 0 {
			continue
		}

		log.Trace("Parsing line: %s", l)
		dep, err := ParseCPANLine(l)

		if err != nil {
			log.Error("=> Error parsing line: %s", err)
			continue
		}

		if dep != nil {
			log.Info("=> Found dependency: %s", dep)
			cpanfile.AddDependency(dep)
			continue
		}

		log.Trace("=> No error and no dependency found")
	}

	log.Info("Found %d dependencies in cpanfile", len(cpanfile.Dependencies))

	return cpanfile, nil	
}

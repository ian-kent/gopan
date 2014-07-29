package gopan

import (
	"github.com/ian-kent/go-log/log"
	"os/exec"
	"strings"
	"syscall"
)

type PerlDeps struct {
	HasPerl                   bool
	PerlVersion               string
	HasParseLocalDistribution bool
	HasJSONXS                 bool
	Ok                        bool
}

func (pd *PerlDeps) Dump() {
	log.Debug("Perl dependencies:")
	log.Debug("                Found perl => %t", pd.HasPerl)
	log.Debug("              Perl version => %s", pd.PerlVersion)
	log.Debug("                  JSON::XS => %t", pd.HasJSONXS)
	log.Debug("  Parse::LocalDistribution => %t", pd.HasParseLocalDistribution)
}

func TestPerlDeps() *PerlDeps {
	pd := &PerlDeps{}
	pd.HasPerl, pd.PerlVersion = HasPerl()
	pd.HasParseLocalDistribution = HasParseLocalDistribution()
	pd.HasJSONXS = HasJSONXS()
	pd.Ok = pd.HasPerl && pd.HasParseLocalDistribution && pd.HasJSONXS
	return pd
}

func HasPerl() (bool, string) {
	cmd := exec.Command("perl", "-V::version:")

	out, err := cmd.Output()
	if err != nil {
		log.Error("Error executing command: %s", err)
		return false, ""
	}

	ps := cmd.ProcessState
	sy := ps.Sys().(syscall.WaitStatus)
	exit := sy.ExitStatus()

	if exit != 0 {
		return false, ""
	}

	s := string(out)
	s = strings.TrimSpace(s)
	l := strings.Split(s, "\n")
	v := l[0]

	return true, v
}

func HasParseLocalDistribution() bool {
	cmd := exec.Command("perl", "-MParse::LocalDistribution", "-e", "")

	_, err := cmd.Output()
	if err != nil {
		log.Error("Error executing command: %s", err)
		return false
	}

	ps := cmd.ProcessState
	sy := ps.Sys().(syscall.WaitStatus)
	exit := sy.ExitStatus()

	if exit != 0 {
		return false
	}

	return true
}

func HasJSONXS() bool {
	cmd := exec.Command("perl", "-MJSON::XS", "-e", "")

	_, err := cmd.Output()
	if err != nil {
		log.Error("Error executing command: %s", err)
		return false
	}

	ps := cmd.ProcessState
	sy := ps.Sys().(syscall.WaitStatus)
	exit := sy.ExitStatus()

	if exit != 0 {
		return false
	}

	return true
}

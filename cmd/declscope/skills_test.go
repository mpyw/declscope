package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The adoption skill used to be installed with `gh skill install`, which
// needed the skill to sit at the repository root and needed gh. The binary
// carries it now, so the tool that the skill is about installs it.
//
// Every case drives the real binary, because the subcommand is decided before
// the driver parses anything and only a separate process shows that.

func TestSkillInstallPutsTheSkillWhereItWasAsked(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "skills")

	out, code := runSkill(t, "skill", "install", "--dir", dest)
	if code != 0 {
		t.Fatalf("install exited %d:\n%s", code, out)
	}
	if !strings.Contains(out, "installed") {
		t.Errorf("install said:\n%s", out)
	}

	body, err := os.ReadFile(filepath.Join(dest, "declscope-adoption", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "Adopting declscope") {
		t.Errorf("the installed manifest is not the skill:\n%.200s", body)
	}
	// The stamp names this binary and the release it reports.
	if !strings.Contains(string(body), "x-embedded-by: declscope") {
		t.Errorf("the installed manifest carries no stamp:\n%.400s", body)
	}

	out, code = runSkill(t, "skill", "list", "--dir", dest)
	if code != 0 || !strings.Contains(out, "up-to-date") {
		t.Errorf("list exited %d:\n%s", code, out)
	}

	out, code = runSkill(t, "skill", "uninstall", "--dir", dest)
	if code != 0 || !strings.Contains(out, "removed") {
		t.Errorf("uninstall exited %d:\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(dest, "declscope-adoption")); err == nil {
		t.Error("the skill survived uninstall")
	}
}

// Three guards now read the first argument. None may swallow another, and none
// may swallow a package pattern.
func TestSkillSubcommandLeavesTheOthersAlone(t *testing.T) {
	if out, code := runSkill(t, "skill"); code != 0 || !strings.Contains(out, "declscope skill install") {
		t.Errorf("bare skill exited %d:\n%s", code, out)
	}
	if out, code := runSkill(t, "skill", "nonsense"); code == 0 {
		t.Errorf("an unknown skill subcommand exited 0:\n%s", out)
	}
	// baseline still reaches its own guard.
	if out, code := runSkill(t, "baseline", "--help"); code != 0 && !strings.Contains(out, "baseline") {
		t.Errorf("baseline --help exited %d:\n%s", code, out)
	}
	// And the driver still answers the protocol `go vet -vettool` speaks.
	if out, code := runSkill(t, "-V=full"); code != 0 || !strings.Contains(out, "declscope version") {
		t.Errorf("-V=full exited %d:\n%s", code, out)
	}
}

// runSkill drives the binary built for this package.
func runSkill(t *testing.T, args ...string) (string, int) {
	t.Helper()
	out, err := exec.Command(bin, args...).CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	exit, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("running declscope %v: %v\n%s", args, err, out)
	}
	return string(out), exit.ExitCode()
}

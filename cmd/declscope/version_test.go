package main_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// -V=full is the check the README and the adoption skill tell a reader to run
// before trusting a config file to a version of these rules, and it is what
// `go vet -vettool` reads to identify the tool it is about to run. Up to 0.6.0
// it answered "devel" on every released binary, because the -V that x/tools
// registers has no release to name.
//
// The line is asserted the way the go command parses it: "<progname> version
// <version> ... buildID=<id>", with the version in the third field and the
// buildID last.

func TestVersionFlagReportsTheStampedRelease(t *testing.T) {
	stamped := filepath.Join(t.TempDir(), "declscope")
	build := coverBuild("-ldflags", "-X main.version=v9.9.9", "-o", stamped)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building a stamped binary: %v\n%s", err, out)
	}

	fields := versionFields(t, stamped)
	if fields[2] != "9.9.9" {
		t.Errorf("version field is %q, want the stamped release 9.9.9\n(a released binary reporting devel is the defect)", fields[2])
	}
}

// TestVersionFlagKeepsTheToolIDProtocol checks the unstamped build, which is
// what a checkout produces. It has no release to name, but the line still has
// to parse: the go command rejects a -V=full whose version is "devel" without
// a buildID, and refuses to run the vettool at all.
func TestVersionFlagKeepsTheToolIDProtocol(t *testing.T) {
	fields := versionFields(t, bin)
	if fields[2] == "" {
		t.Error("version field is empty")
	}
}

// versionFields runs -V=full and returns the fields of the one line it prints,
// having checked everything the go command checks about its shape.
func versionFields(t *testing.T, path string) []string {
	t.Helper()
	out, err := coverEnv(exec.Command(path, "-V=full")).Output()
	if err != nil {
		t.Fatalf("running -V=full: %v", err)
	}
	line := strings.TrimSpace(string(out))
	fields := strings.Fields(line)
	if len(fields) < 3 {
		t.Fatalf("-V=full printed %q, want at least three fields", line)
	}
	if fields[1] != "version" {
		t.Errorf("second field is %q, want \"version\": %s", fields[1], line)
	}
	if !strings.HasPrefix(fields[len(fields)-1], "buildID=") {
		t.Errorf("last field is %q, want a buildID: %s", fields[len(fields)-1], line)
	}
	return fields
}

// TestVersionFlagRefusesAnotherValue checks the other half of the protocol.
// -V is a boolean flag whose only value is full, and the go command sends
// nothing else; a binary that accepted -V=short and printed something would
// be answering a question nobody asked.
func TestVersionFlagRefusesAnotherValue(t *testing.T) {
	out, err := coverEnv(exec.Command(bin, "-V=short")).CombinedOutput()
	if err == nil {
		t.Fatalf("-V=short was accepted:\n%s", out)
	}
	if !strings.Contains(string(out), "-V=full") {
		t.Errorf("the refusal should name the value it takes:\n%s", out)
	}
}

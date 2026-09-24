package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/debug"
	"strings"
)

// version is the release this binary was built from, written at link time with
// -X main.version=<version>; .goreleaser.yaml passes it.
//
// It is a var rather than a constant because that is what the linker can
// write, and it is empty in every build that is not a release: a checkout, a
// `go build`, a `go run`. Those fall back to what the module system recorded,
// and to "devel" when there is nothing to record.
var version string

// registerVersionFlag claims -V before the driver does.
//
// x/tools' analysisflags registers a -V that prints "devel" for every binary,
// since a plain Analyzer has nowhere to keep a release number — which is why
// every published declscope up to 0.6.0 answered `-V=full` with "devel". Its
// addVersionFlag does nothing when a -V is already registered; the case it was
// written for is cmd/internal/objabi doing exactly this, so registering ours
// first replaces it rather than colliding with it.
//
//declscope:package // main.go registers it before handing over to the driver
func registerVersionFlag() {
	flag.Var(versionFlag{}, "V", "print version and exit")
}

// versionFlag is the -V protocol `go vet` speaks to a -vettool, and the
// question a reader asks before trusting a config file to a version of these
// rules.
type versionFlag struct{}

func (versionFlag) IsBoolFlag() bool { return true }
func (versionFlag) String() string   { return "" }

func (versionFlag) Set(s string) error {
	if s != "full" {
		return fmt.Errorf("unsupported flag value: -V=%s (use -V=full)", s)
	}
	printVersion()
	os.Exit(0)
	return nil
}

// printVersion writes the line the tool-ID protocol reads.
//
// The shape is the one x/tools prints, with the release in place of its
// hardcoded "devel". The go command reads the third field as the version and,
// when it is not "devel", takes the whole line as the tool's identity, so the
// buildID stays on it: that keeps the identity tied to this exact binary and
// not merely to a version string two builds could share.
func printVersion() {
	progname, err := os.Executable()
	if err != nil {
		log.Fatalf("finding this executable: %v", err)
	}
	fmt.Printf("%s version %s comments-go-here buildID=%s\n",
		progname, versionString(), buildIDForVersion(progname))
}

// versionString is the release, as honestly as the binary can tell it. Each
// source answers for a way declscope is installed, and the order is the order
// they can be trusted:
//
//   - the stamp a release build writes, which is what a GitHub Release or a
//     mise install carries;
//   - the module version the go command records for `go install <pkg>@<v>`,
//     which no linker flag reaches;
//   - "devel", for a build from a checkout, where there is no release to name.
//
// The leading v is dropped so that the two sources read alike: goreleaser
// spells a tag without it, and the module system keeps it.
//
//declscope:package // skills.go stamps an installed skill with the same release
func versionString() string {
	if version != "" {
		return strings.TrimPrefix(version, "v")
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return strings.TrimPrefix(v, "v")
		}
	}
	return "devel"
}

// buildIDForVersion hashes the binary, which is what x/tools does and what the
// go command falls back to when the version is "devel". A build from a
// checkout has no other identity, and a stale vet cache is the cost of getting
// it wrong.
func buildIDForVersion(progname string) string {
	f, err := os.Open(progname)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		log.Fatal(err)
	}
	return fmt.Sprintf("%02x", h.Sum(nil))
}

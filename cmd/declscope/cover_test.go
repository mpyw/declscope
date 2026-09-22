package main_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Every test in this package drives the built linter as a subprocess, because
// what they assert is the handshake between a subcommand and the analyzer: a
// unit test of either half proves nothing about it. The cost is that `go test
// -coverprofile` sees this process only, and counts none of the statements the
// subprocess executed — the subcommands read as untested however thoroughly
// they are exercised here.
//
// A binary built with `go build -cover` writes its counters to the directory
// named by GOCOVERDIR instead, which `go tool covdata textfmt` turns into an
// ordinary profile. Setting DECLSCOPE_COVERDIR turns that on: TestMain builds
// the instrumented binary, every run below points at the directory, and the
// caller — the CI job, or anyone running the tests by hand — converts what
// lands there and hands it to the coverage service beside `go test`'s own
// profile. Without the variable the binary is built and run exactly as before,
// so a plain `go test ./...` pays nothing for this.

// coverDir is the absolute GOCOVERDIR every subprocess writes to, or "" when
// the run is not collecting coverage. It is absolute because each run sets its
// own working directory, to the temporary module it was given.
var coverDir string

// coverSetUp resolves DECLSCOPE_COVERDIR and creates it. TestMain calls it
// before building, since the build flags depend on the answer.
//
//declscope:package // TestMain calls it before building for every test here
func coverSetUp() error {
	dir := os.Getenv("DECLSCOPE_COVERDIR")
	if dir == "" {
		return nil
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("DECLSCOPE_COVERDIR: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return fmt.Errorf("DECLSCOPE_COVERDIR: %w", err)
	}
	coverDir = abs
	return nil
}

// coverBuildFlags are the flags that make a build write counters.
//
// -coverpkg reaches the whole module, not just package main: a subcommand's
// worth is mostly in what it drives, and internal.Collect is reached from the
// baseline subcommand alone. atomic matches what the CI job asks `go test`
// for, and go/packages runs the analyzer concurrently.
func coverBuildFlags() []string {
	if coverDir == "" {
		return nil
	}
	return []string{"-cover", "-covermode=atomic", "-coverpkg=github.com/mpyw/declscope/..."}
}

// coverEnv points one prepared command at the counter directory. A command
// that inherits the environment unchanged is left alone, so that the tests run
// identically when coverage is off.
//
// The environment is taken from cmd.Environ(), not os.Environ(): os/exec sets
// PWD to cmd.Dir, and only for a command whose Env it is still choosing.
// Copying the parent's environment instead leaves the parent's PWD in place,
// os.Getwd() in the subprocess falls back to what the kernel reports, and on
// macOS a temporary directory then comes back as /private/var/... where the
// test holds /var/... . Callers set cmd.Dir before calling this.
//
//declscope:package // every subcommand's tests point their runs at the counters
func coverEnv(cmd *exec.Cmd) *exec.Cmd {
	if coverDir != "" {
		cmd.Env = append(cmd.Environ(), "GOCOVERDIR="+coverDir)
	}
	return cmd
}

// coverBuild assembles a `go build` of this package, with the coverage flags
// in front of the caller's own.
//
//declscope:package // TestMain and version_test.go each build a binary with it
func coverBuild(args ...string) *exec.Cmd {
	argv := append([]string{"build"}, coverBuildFlags()...)
	argv = append(argv, args...)
	argv = append(argv, ".")
	return exec.Command("go", argv...)
}

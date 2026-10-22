package main_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The README shows survey and inspect run on one package of this repository.
// Those numbers move whenever that package or the rules change, and nothing
// else would notice: the samples drifted for several releases before anyone
// re-ran them. So each sample is run here, from the repository root, and
// compared with what the README shows.
//
// A sample may be an excerpt, but only of whole sections. It starts at a
// `## ` heading and ends where a section does: before the next heading, before
// a fenced block such as the Mermaid diagram, which the README describes as a
// section of its own, or at the end of the output. Leaving out a section
// before or after it is fine. Leaving out lines inside it is not, since a
// table with rows silently missing reads as the whole table. Only stdout is
// compared, so a line on stderr, such as a toolchain download, cannot break
// the match.

// readmeSample is one console block of the README that runs a measuring
// command: the command line and the output shown under it.
type readmeSample struct {
	command string
	output  string
}

func TestReadmeMeasureSamplesMatchTheOutput(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}

	samples := readmeSamples(string(readme))
	if len(samples) != 2 {
		t.Fatalf("found %d survey or inspect samples in README.md, want 2: %+v", len(samples), samples)
	}
	for _, s := range samples {
		t.Run(s.command, func(t *testing.T) {
			args := strings.Fields(strings.TrimPrefix(s.command, "declscope "))
			out := readmeStdout(t, root, args...)
			if err := s.matches(out); err != nil {
				t.Errorf("README.md shows output that `%s` no longer prints: %v.\n"+
					"Replace the sample with whole sections of this:\n%s", s.command, err, out)
			}
		})
	}
}

// readmeSamples returns each console block whose first line runs
// `declscope survey` or `declscope inspect`.
func readmeSamples(readme string) []readmeSample {
	var samples []readmeSample
	lines := strings.Split(readme, "\n")
	for i := 0; i+1 < len(lines); i++ {
		if lines[i] != "```console" {
			continue
		}
		command, ok := strings.CutPrefix(lines[i+1], "$ ")
		if !ok {
			continue
		}
		if words := strings.Fields(command); len(words) < 2 || words[0] != "declscope" || (words[1] != "survey" && words[1] != "inspect") {
			continue
		}
		var output strings.Builder
		for j := i + 2; j < len(lines) && lines[j] != "```"; j++ {
			output.WriteString(lines[j] + "\n")
		}
		samples = append(samples, readmeSample{command: command, output: output.String()})
	}
	return samples
}

// matches reports why the sample is not a run of whole sections of out.
func (s readmeSample) matches(out string) error {
	if strings.TrimSpace(s.output) == "" {
		return errors.New("the sample is empty")
	}
	if !strings.HasPrefix(s.output, "## ") {
		return errors.New("the sample does not start at a `## ` heading")
	}
	sample := strings.TrimRight(s.output, "\n") + "\n"
	found := false
	for from := 0; ; {
		i := strings.Index(("\n" + out)[from:], "\n"+sample)
		if i < 0 {
			break
		}
		found = true
		start := from + i
		rest := strings.TrimLeft(out[start+len(sample):], "\n")
		if rest == "" || strings.HasPrefix(rest, "#") || strings.HasPrefix(rest, "```") {
			return nil
		}
		from = start + 1
	}
	if found {
		return errors.New("the sample stops partway through a section")
	}
	return errors.New("the output has no such run of lines")
}

// readmeStdout runs the linter in dir and returns its stdout. A nonzero exit
// fails the test, with stderr shown.
func readmeStdout(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	coverEnv(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("declscope %v: %v\nstdout:\n%s\nstderr:\n%s", args, err, out, stderr.String())
	}
	return string(out)
}

func TestReadmeSampleMatchesOnlyWholeSections(t *testing.T) {
	out := "# pkg\n\n## A\n\n| a |\n| 1 |\n| 2 |\n\nfooter\n\n```mermaid\ngraph LR\n```\n\n## B\n\n| b |\n"
	for _, c := range []struct {
		name, sample string
		ok           bool
	}{
		{"a section before a fence", "## A\n\n| a |\n| 1 |\n| 2 |\n\nfooter\n\n", true},
		{"the last section", "## B\n\n| b |\n", true},
		{"two sections", "## A\n\n| a |\n| 1 |\n| 2 |\n\nfooter\n\n```mermaid\ngraph LR\n```\n\n## B\n\n| b |\n", true},
		{"a tail cut off", "## A\n\n| a |\n| 1 |\n", false},
		{"a footer cut off", "## A\n\n| a |\n| 1 |\n| 2 |\n", false},
		{"rows left out", "## A\n\n| a |\n| 2 |\n\nfooter\n", false},
		{"no heading first", "| a |\n| 1 |\n| 2 |\n\nfooter\n", false},
		{"empty", "\n", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := readmeSample{output: c.sample}.matches(out)
			if (err == nil) != c.ok {
				t.Errorf("matches = %v, want ok %v", err, c.ok)
			}
		})
	}
}

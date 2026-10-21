package main_test

import (
	"os"
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
// A sample may be an excerpt, but only a contiguous one. Leaving out a
// section before or after it is fine. Leaving out lines inside it is not,
// since a table with rows silently missing reads as the whole table.

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
			out, code := runIn(t, bin, root, args...)
			if code != 0 {
				t.Fatalf("%s exited %d\n%s", s.command, code, out)
			}
			if !strings.Contains("\n"+out, "\n"+s.output) {
				t.Errorf("README.md shows output that `%s` no longer prints.\n"+
					"Replace the sample with a contiguous part of this:\n%s", s.command, out)
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

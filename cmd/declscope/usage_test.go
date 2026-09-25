package main_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/mpyw/declscope"
)

// TestUsageListsEverySubcommand pins the help the driver prints: the
// subcommands before the flags, for -h, a flag error and a bare run. Every
// subcommand main.go dispatches must be listed, so adding one without its
// line fails here.
func TestUsageListsEverySubcommand(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	var subcommands []string
	for _, m := range regexp.MustCompile(`case "(\w+)":`).FindAllStringSubmatch(string(src), -1) {
		subcommands = append(subcommands, m[1])
	}
	if len(subcommands) == 0 {
		t.Fatal("no subcommand found in main.go")
	}
	subcommands = append(subcommands, "skill install", "skill uninstall", "skill list")
	for _, c := range []struct {
		args []string
		code int
	}{
		{[]string{"-h"}, 0},
		{[]string{"-nonsense"}, 2},
		{nil, 1},
	} {
		out, code := runIn(t, bin, t.TempDir(), c.args...)
		if code != c.code {
			t.Errorf("declscope %v exited %d, want %d", c.args, code, c.code)
		}
		sub, flags := strings.Index(out, "Subcommands:"), strings.Index(out, "Flags:")
		if sub < 0 || flags < sub || !strings.Contains(out[flags:], "-fix") {
			t.Errorf("declscope %v: want the subcommands, then the flags:\n%s", c.args, out)
			continue
		}
		for _, s := range subcommands {
			if !regexp.MustCompile(`declscope ` + strings.ReplaceAll(s, " ", ` `) + ` +\[flags\]`).MatchString(out[sub:flags]) {
				t.Errorf("declscope %v does not list %q", c.args, s)
			}
		}
	}
}

// TestUsageDocIsOneParagraph pins what usageInstall assumes: the help prints
// the Doc whole beside the name, where the driver prints its first paragraph.
func TestUsageDocIsOneParagraph(t *testing.T) {
	if strings.Contains(declscope.Analyzer.Doc, "\n\n") {
		t.Errorf("the Doc has more than one paragraph, which the help prints beside the name:\n%s", declscope.Analyzer.Doc)
	}
}

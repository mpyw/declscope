package main

import (
	"flag"
	"os"
	"strings"
)

// tagsUsage is the help for -tags, on the analyzer and on every subcommand
// alike. The backquoted word names the value in the flag listing.
//
//declscope:package // usage.go puts it on the analyzer's own -tags
const tagsUsage = "comma-separated `list` of build tags to consider satisfied, as for go build"

// Build tags reach every command through GOFLAGS, which the go command reads
// on each go list that go/packages and declscope shrink run. A -tags flag
// appends to it, so a tag given on the command line overrides one GOFLAGS
// already holds, as it does for the go command: the last -tags wins.
//
// The analyzer cannot take them any other way. x/tools' driver registers a
// -tags of its own, a no-op kept so that old scripts running vet do not
// break, and loads packages with no build flags (golang/go#65776 declined to
// change that). Registering -tags before it would panic on the redefinition,
// so the analyzer's -tags is taken out of the arguments before the driver
// parses them.

// tagsValueFlags are the analyzer's flags that take a value. Reading the
// arguments the way package flag does needs them: in -config c.yaml -tags x,
// c.yaml is a value and not the first package. TestTagsValueFlags checks the
// list against the flags the driver registers.
var tagsValueFlags = map[string]bool{
	"c": true, "config": true, "cpuprofile": true, "debug": true, "memprofile": true, "trace": true,
}

// tagsFromArgs applies every -tags among the analyzer's flags and returns the
// arguments without them. Flags end where package flag stops: at --, or at
// the first argument that is not a flag.
//
//declscope:package // main.go takes the analyzer's -tags before the driver runs
func tagsFromArgs(args []string) []string {
	out := []string{args[0]}
	rest := args[1:]
	for i := 0; i < len(rest); i++ {
		arg := rest[i]
		if arg == "--" || len(arg) < 2 || arg[0] != '-' {
			return append(out, rest[i:]...)
		}
		name, value, hasValue := strings.Cut(strings.TrimPrefix(arg[1:], "-"), "=")
		if name == "tags" {
			if !hasValue {
				if i+1 == len(rest) {
					// Left for the driver, which reports the missing value.
					return append(out, arg)
				}
				i++
				value = rest[i]
			}
			tagsApply(value)
			continue
		}
		out = append(out, arg)
		if !hasValue && tagsValueFlags[name] && i+1 < len(rest) {
			i++
			out = append(out, rest[i])
		}
	}
	return out
}

// tagsRegister gives a subcommand its -tags.
//
//declscope:package // every subcommand that loads packages takes -tags
func tagsRegister(fs *flag.FlagSet) {
	fs.Var(tagsValue{}, "tags", tagsUsage)
}

// tagsValue applies each -tags as it is parsed, so the last one wins.
type tagsValue struct{}

func (tagsValue) String() string { return "" }

func (tagsValue) Set(s string) error {
	tagsApply(s)
	return nil
}

// tagsApply appends the tags to GOFLAGS. The go command also reads a list
// separated by spaces, but GOFLAGS is itself separated by spaces, so the tags
// are joined with commas first. An empty list clears the tags, as -tags=
// does for the go command.
func tagsApply(list string) {
	tags := strings.FieldsFunc(list, func(r rune) bool { return r == ',' || r == ' ' })
	flags := strings.TrimSpace(os.Getenv("GOFLAGS") + " -tags=" + strings.Join(tags, ","))
	_ = os.Setenv("GOFLAGS", flags)
}

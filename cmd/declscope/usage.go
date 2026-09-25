package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/mpyw/declscope"
)

// usageSubcommands is what the help lists besides the analyzer. main.go
// dispatches each of them before the driver sees the arguments, so the
// driver's own help cannot know of them.
const usageSubcommands = `Subcommands:

   declscope baseline [flags] [packages]
   declscope inspect  [flags] <package>
   declscope survey   [flags] [packages]
   declscope shrink   [flags] [packages]

   declscope skill install   [flags] [skill...]
   declscope skill uninstall [flags] [skill...]
   declscope skill list      [flags] [skill...]

Run a subcommand with -h for its flags.
`

// usageInstall makes the help the driver prints list the subcommands.
//
// singlechecker.Main assigns flag.Usage itself, so assigning it here would be
// undone. The flag set calls CommandLine.Usage, whose default only calls
// flag.Usage, and the driver leaves it alone: replacing it is what reaches
// -h and a flag error. By then every flag is registered, so the list below
// is the driver's. The header is the driver's layout, with the subcommands
// before the flags, which run to dozens of lines.
//
// A run with no argument at all prints the same help and exits 1, as the
// driver does. The driver answers a run with flags but no package on its own,
// without the subcommands.
//
//declscope:package // main.go installs it before the driver runs
func usageInstall() {
	bare := len(os.Args) == 1
	if bare {
		os.Args = append(os.Args, "-h")
	}
	flag.CommandLine.Usage = func() {
		// The driver splits the Doc into paragraphs and prints the first beside
		// the name. declscope's Doc is one paragraph, so it is printed whole.
		a := declscope.Analyzer
		head := fmt.Sprintf("%s: %s\n\nUsage: %s [-flag] [package]\n\n", a.Name, a.Doc, a.Name)
		_, _ = io.WriteString(flag.CommandLine.Output(), head+usageSubcommands+"\nFlags:\n")
		flag.PrintDefaults()
		if bare {
			os.Exit(1)
		}
	}
}

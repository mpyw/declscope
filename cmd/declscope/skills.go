package main

import (
	skillembed "github.com/mpyw/go-skill-embed"

	"github.com/mpyw/declscope"
)

// skills is the command that puts the adoption skill where an agent reads it.
//
// It carries the release the binary reports, so an installed copy names the
// version of the rules it describes.
//
//declscope:package // main.go hands it the arguments before the driver sees them
var skills = skillembed.NewInstaller(
	declscope.Skills,
	skillembed.WithToolName("declscope"),
	skillembed.WithVersion(versionString()),
)

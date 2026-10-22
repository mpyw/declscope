//go:build !linux && !darwin && !windows

//declscope:core

package coregoossibling

// This keeps the report in a build that compiles none of the GOOS files.
func Platform() int { return fooHelper() }

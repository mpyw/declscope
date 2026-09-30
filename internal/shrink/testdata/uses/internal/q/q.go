// Package q is imported by a build-excluded file of package b. It holds a
// package-level name alone: any field or method would be held back by that
// import, which is not what this module pins.
package q

// ExclQual is named only by the build-excluded file.
var ExclQual = 1

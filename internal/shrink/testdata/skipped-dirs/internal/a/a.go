// Package a is imported only from directories ./... skips. Each of them
// still belongs to the module, and go build compiles a package there when it
// is named. As with a name only external tests use, such a use is no use, so
// the name is reported, but the fix is withheld: it would break that package.
package a

// Greeting is called by testdata/prog, which a test runs with go run.
func Greeting() string { return "hello" } // want: func Greeting is exported.*\(no fix: a file in a directory \./\.\.\. skips uses it\)$

// FromHidden is named by a package in .hidden.
func FromHidden() int { return 1 } // want: func FromHidden is exported.*\(no fix: a file in a directory \./\.\.\. skips uses it\)$

// FromUnderscore is named by a package in _skipped.
func FromUnderscore() int { return 2 } // want: func FromUnderscore is exported.*\(no fix: a file in a directory \./\.\.\. skips uses it\)$

// FromIgnored is named by gen, which an ignore directive names at the root.
func FromIgnored() int { return 3 } // want: func FromIgnored is exported.*\(no fix: a file in a directory \./\.\.\. skips uses it\)$

// FromIgnoredAnywhere is named by web/static, which an ignore directive
// names at any depth.
func FromIgnoredAnywhere() int { return 4 } // want: func FromIgnoredAnywhere is exported.*\(no fix: a file in a directory \./\.\.\. skips uses it\)$

// FromVendor is named by internal/a/vendor/w. The go command uses only the
// vendor directory at the module root, so this one holds an ordinary
// package.
func FromVendor() int { return 7 } // want: func FromVendor is exported.*\(no fix: a file in a directory \./\.\.\. skips uses it\)$

// FromAssembly is read only by the assembly of testdata/asmprog.
var FromAssembly int64 = 6 // want: var FromAssembly is exported.*\(no fix: a file in a directory \./\.\.\. skips uses it\)$

// Spare is named only by vendor/example.com/v, in the vendor directory at
// the module root, which builds no package. It is fixed.
func Spare() int { return 5 } // want: func Spare is exported, but nothing outside \S+ uses it$

// T is handed to testdata/prog by New.
type T struct{} // want: type T is exported.*\(no fix: an exported declaration that keeps its name hands it out\)$

// New is called by testdata/prog.
func New() T { return T{} } // want: func New is exported.*\(no fix: a file in a directory \./\.\.\. skips uses it\)$

// Method is not named, but testdata/prog holds a T, and is not type-checked.
func (T) Method() {} // want: method Method is exported.*\(no fix: a build-excluded file of another package may use it\)$

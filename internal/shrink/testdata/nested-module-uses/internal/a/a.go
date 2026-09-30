// Package a is imported by the nested module in tools, which is loaded, so
// each way it uses a declaration counts, and only what it leaves is reported.
package a

// Called is called from tools.
func Called() {}

func Unused() {} // want: func Unused is exported, but nothing outside example.com/nu/internal/a uses it$

// Runner has Run called through tools' own interface, which names no method
// of this package.
type Runner struct{}

func (Runner) Run() {}

// Printed reaches fmt.Println in tools, so reflection may read its fields.
type Printed struct {
	Field int
}

// NewPrinted is how tools gets a Printed without naming its field.
func NewPrinted() Printed { return Printed{} }

// Tagged is written only in a file of tools that a build tag excludes.
func Tagged() {}

// InTest is called only from a test of tools.
func InTest() {}

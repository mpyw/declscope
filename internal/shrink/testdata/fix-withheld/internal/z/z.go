// Package z holds declarations named only by example functions in a test
// file the build leaves out. go vet checks the names when it builds the
// file, so the fix is withheld, and the report reads as it does in that
// configuration.
package z

// Widget is named by ExampleWidget, and nothing else names it.
type Widget struct{} // want: type Widget is exported.*no fix: an example function names it\)$

// Gadget and Spin are named by ExampleGadget_Spin. Any build-excluded file
// of the package already withholds the fix of a method.
type Gadget struct{} // want: type Gadget is exported.*no fix: an example function names it\)$

func (Gadget) Spin() {} // want: method Spin is exported.*no fix: a build-excluded file of its package may use it\)$

// Spare is named by nothing, and is fixed.
func Spare() int { return 1 } // want: func Spare is exported, but nothing outside \S+ uses it$

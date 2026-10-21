package i

import "testing"

// A test function is no candidate, since go test finds it by name, but its
// ignores are still one binding, as they are for the analyzer: the ignore
// naming unused answers the doc's report.
//
//declscope:ignore overexported
func TestX(t *testing.T) {} //declscope:ignore unused

// A repeat is the analyzer's to report, as a duplicate, and no second site
// here.
//
//declscope:ignore overexported // want: unused //declscope:ignore overexported
func TestY(t *testing.T) {} //declscope:ignore overexported

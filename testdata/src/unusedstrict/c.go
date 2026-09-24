//declscope:private // want `unused file-level //declscope:private: every declaration it reaches already has private scope`

package unusedstrict

// A declaration restating the file's directive is reported, and so is the
// file's, which restates the default for everything here.
//
//declscope:private // want `unused //declscope:private on cFile: it already has private scope`
func cFile() int { return 1 }

func cOther() int { return 2 }

var _ = cFile() + cOther()

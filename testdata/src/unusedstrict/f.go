//declscope:private // want `unused file-level //declscope:private: every declaration it reaches already has private scope`

package unusedstrict

// The spec's private narrows its block's package, so strict keeps it too. The
// block reaches no declaration that takes its scope. The file's private is
// what fNarrowed would have from the default anyway.
//
//declscope:package // want `unused //declscope:package: every declaration it reaches states its own scope`
var (
	//declscope:private
	fNarrowed = 1
)

var _ = fNarrowed

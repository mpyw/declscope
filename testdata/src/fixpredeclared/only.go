package fixpredeclared

// Dropping the prefix would leave a predeclared name. The declaration itself
// would compile, shadowing the builtin for the whole package, so the fix is
// withheld and the violation reported alone.
var onlyLen = 3 // want `var onlyLen carries the prefix of namespace "only", which is not required here; rename it to len`

type onlyError struct{} // want `type onlyError carries the prefix of namespace "only", which is not required here; rename it to error`

var _ onlyError

// Free, so renamed.
var onlyHelper = 1 // want `var onlyHelper carries the prefix of namespace "only", which is not required here; rename it to helper`

func Exported(s string) int { return len(s) + onlyLen + onlyHelper }

package ifacemembers

// Satisfying an interface is not a use of its method names. A method set is
// resolved, not written, so this file crosses nothing — and if it did, the
// idiom would be a violation in every codebase that uses it, repairable only
// by a directive that changes nothing about Go semantics.
type conc struct{}

func (conc) sealed() bool { return true }
func (conc) Open() int    { return 1 }

var _ Sealed = conc{}

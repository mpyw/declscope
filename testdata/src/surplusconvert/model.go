package surplusconvert

type model struct {
	ID int
	//declscope:package
	rev int
}

//declscope:package
func modelNew() model { return model{ID: 1, rev: 2} }

// genModel is generic. A conversion from its instantiation pairs the
// instantiated fields, which are mapped back to the ones declared here.
type genModel[T any] struct {
	ID int
	//declscope:package
	rev T
}

//declscope:package
func genModelNew() genModel[int] { return genModel[int]{ID: 1, rev: 2} }

// Ledger is converted to its own type from snapshot.go. That conversion is the
// identity: it pairs no field with another type's, so it keeps nothing wide.
type Ledger struct {
	//declscope:package // want `//declscope:package on Ledger.seq: no use from another namespace is visible to declscope`
	seq int
}

var _ = Ledger{seq: 1}

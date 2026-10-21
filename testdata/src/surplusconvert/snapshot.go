package surplusconvert

// snapshot mirrors model for persistence. The conversion pairs every field by
// name and spells none of them, so rev's directive is load-bearing without a
// visible reference — the rule must stay quiet.
type snapshot struct {
	ID  int
	rev int
}

func snapshotOf() snapshot { return snapshot(modelNew()) }

var _ = snapshotOf

type genSnapshot struct {
	ID  int
	rev int
}

func snapshotOfGen() genSnapshot { return genSnapshot(genModelNew()) }

var _ = snapshotOfGen

func snapshotLedger(l Ledger) Ledger { return Ledger(l) }

var _ = snapshotLedger

// A conversion to a type parameter allowing only *ParamModel pairs its fields
// with paramRow's, as (*ParamModel)(r) does. The source side alone was read
// before, which is why the directive in model.go sits on the target side.
type paramRow struct {
	ID  int
	rev int
}

func snapshotOfParam[P ~*ParamModel](r *paramRow) P { return P(r) }

var _ = snapshotOfParam[*ParamModel]

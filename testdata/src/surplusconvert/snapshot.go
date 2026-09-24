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

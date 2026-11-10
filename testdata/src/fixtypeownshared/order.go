package fixtypeownshared

func orderRun() string {
	e := entry{key: "x", note: "y"}
	return e.key + e.note + pair{}.Label
}

var _ = orderRun

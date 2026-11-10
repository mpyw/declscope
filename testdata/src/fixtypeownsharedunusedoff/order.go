package fixtypeownsharedunusedoff

func orderRun() string {
	e := entry{key: "x", note: "y"}
	return e.key + e.note
}

var _ = orderRun

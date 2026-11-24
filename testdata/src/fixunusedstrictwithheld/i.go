package fixunusedstrictwithheld

// g_other.go writes iItems{{1}}. The inner literal leaves out iItem and fills
// n by position, so the file names only iItems, a named slice. The crossing
// into n may be there.
type iItem struct {
	//declscope:private // want `unused //declscope:private on iItem.n: it already has private scope`
	n int
}

type iItems []iItem

var _ = iItem{n: 1}.n

package b

import "example.com/constsets/internal/e"

var (
	_ = e.StClaimed
	_ = e.StDone
	_ e.ItemStatus
	_ e.Due
	_ = e.Mixed
	_ = e.Week
	_ = e.Current()
)

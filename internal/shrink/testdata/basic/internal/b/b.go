package b

import "example.com/basic/internal/a"

var _ = a.Used() + a.Ignored() + a.Mixed()

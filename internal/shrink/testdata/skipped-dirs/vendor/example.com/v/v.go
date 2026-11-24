// Package v sits in the vendor directory at the module root, the one the go
// command vendors into. It builds no package there, so what this file names
// is no use: a.Spare is still fixed.
package v

import "example.com/skipped/internal/a"

var _ = a.Spare()

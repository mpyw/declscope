//go:build never

package u

import . "example.com/withheld/internal/w"

var _ = Dotted

func pick(p interface{ Pick() }) { p.Pick() }

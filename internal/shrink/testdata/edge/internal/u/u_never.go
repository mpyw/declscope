//go:build never

package u

import . "example.com/edge/internal/m"

var _ = Dotted

func pick(p interface{ Pick() }) { p.Pick() }

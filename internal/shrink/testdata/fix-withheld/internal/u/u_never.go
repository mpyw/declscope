//go:build never

package u

import . "example.com/withheld/internal/y"

var _ = Dotted

func pick(p interface{ Pick() }) { p.Pick() }

var _ = DotFilled{1}

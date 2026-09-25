// Package pub hands out a value of a type that embeds m.Promoted, so the
// promoted method and field reach another module.
package pub

import "example.com/edge/internal/m"

type Wrapper struct{ m.Promoted }

func Get() Wrapper { return Wrapper{} }

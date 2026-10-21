// Package q has a test, so the variant read is q with its test, not the one
// cmd/gv imports.
package q

import "encoding/json"

func Encode[V any](v V) string {
	b, _ := json.Marshal(v)
	return string(b)
}

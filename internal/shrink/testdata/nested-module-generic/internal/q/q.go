package q

import "encoding/json"

func Encode[V any](v V) string {
	b, _ := json.Marshal(v)
	return string(b)
}

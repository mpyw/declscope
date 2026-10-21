package tools

import (
	"example.com/ng/internal/a"
	"example.com/ng/internal/q"
)

func Use() string { return q.Encode(a.Make()) }

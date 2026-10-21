package main

import (
	"fmt"

	"example.com/gv/internal/p"
	"example.com/gv/internal/q"
	"example.com/gv/internal/s"
)

func main() {
	fmt.Println(q.Encode(p.Make()))
	fmt.Println(s.Wrap(p.MakeU()))
}

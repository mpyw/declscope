package tools

import (
	"fmt"

	"example.com/nu/internal/a"
)

type runner interface{ Run() }

func Use() {
	a.Called()
	var r runner = a.Runner{}
	r.Run()
	fmt.Println(a.NewPrinted())
}

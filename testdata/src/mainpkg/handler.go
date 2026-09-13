package main

func handlerDo() { // want `func handlerDo is private to namespace "handler", but is used from namespace "main"`
	c := ServerNew()
	c.Addr = ":80"
	_ = c
}

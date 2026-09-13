//declscope:private

package main

// Nothing can import a main package, but plugin.Lookup reaches its exported
// symbols by name, so exportedness keeps its meaning and its default. A main
// package that is not a plugin says otherwise in one line, and the file-level
// directive reaches the exported declarations too.
type Config struct {
	Addr string // want `field Config.Addr is declared private by the file's //declscope:private, but is used from namespace "handler"`
}

func ServerNew() *Config { return &Config{} } // want `func ServerNew is declared private by the file's //declscope:private, but is used from namespace "handler"`

// func main is the one name the toolchain requires, so the naming rules pass
// it by however they are set.
func main() { handlerDo() }

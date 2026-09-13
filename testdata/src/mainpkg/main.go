//declscope:private

package main

// Exportedness keeps its meaning here, so Config and ServerNew are held back
// only because this file says so: a file-level directive reaches exported
// declarations too.
type Config struct {
	Addr string // want `field Config.Addr is declared private by the file's //declscope:private, but is used from namespace "handler"`
}

func ServerNew() *Config { return &Config{} } // want `func ServerNew is declared private by the file's //declscope:private, but is used from namespace "handler"`

// func main is the one name the toolchain requires, so the naming rules pass
// it by however they are set.
func main() { handlerDo() }

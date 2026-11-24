//declscope:private

package fixwithheldinherited

// Every kind takes private from the file alone.
func fileHelper() int { return 1 } // want `func fileHelper is declared private by the file's //declscope:private, but is used from namespace "order"`

type fileBox struct {
	n int // want `field fileBox.n is declared private by the file's //declscope:private, but is used from namespace "order"`
}

func (b fileBox) fileGet() int { return b.n } // want `method fileBox.fileGet is declared private by the file's //declscope:private, but is used from namespace "order"`

//declscope:shared
func FileMake() fileBox { return fileBox{} }

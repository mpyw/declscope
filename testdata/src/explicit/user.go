package explicit

// userForced carries the namespace prefix but is forced private, so the
// name and the directive disagree with the use site. Both are deliberate
// statements by the author, so no fix is offered: -fix must not silently
// overwrite a directive that was written on purpose, and renaming alone would
// leave the directive keeping it private while the new name claims otherwise.
//
//declscope:private
func userForced() int { return 1 } // want `func userForced is declared private by //declscope:private, but is used from namespace "order"`

//declscope:private
type userShape struct{} // want `type userShape is declared private by //declscope:private, but is used from namespace "order"`

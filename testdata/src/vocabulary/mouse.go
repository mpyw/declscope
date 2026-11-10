package vocabulary

// wheel is listed under the mouse namespace, so it carries the namespace the
// way mouse itself would.
func wheelDelta() int { return 1 }

// A vocabulary word must still open a word of the name: pinwheel spells wheel
// with no boundary before it, the monkey/key shape.
func pinwheel() int { return 2 } // want `func pinwheel does not carry namespace "mouse" anywhere in its name; give it a name that carries "mouse", such as mousePinwheel, or move it to the file it belongs to`

// A name outside the vocabulary is asked as usual.
func scrollSpeed() int { return 3 } // want `func scrollSpeed does not carry namespace "mouse" anywhere in its name; give it a name that carries "mouse", such as mouseScrollSpeed, or move it to the file it belongs to`

// The namespace's own spelling still counts, vocabulary or not.
func mouseButton() int { return 4 }

var _, _, _, _ = wheelDelta, pinwheel, scrollSpeed, mouseButton

package replace

// scroll is this level's word for mouse.
func scrollSpeed() int { return 1 }

// wheel was the root's word, and the nearer list replaced it.
func wheelDelta() int { return 2 } // want `func wheelDelta does not carry namespace "mouse" anywhere in its name`

var _, _ = scrollSpeed, wheelDelta

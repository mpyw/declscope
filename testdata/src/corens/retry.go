package corens

// Not core, so an ordinary namespace: the prefix is required here, and carries
// the exportedness of the name it joins.
func WithBackoff() int { return 1 } // want `func WithBackoff does not carry namespace "retry" anywhere in its name; give it a name that carries "retry", such as RetryWithBackoff, or move it to the file it belongs to`

// helper is private to the core, so naming it from here crosses a boundary:
// //declscope:core carries no scope of its own. acquire does not, because
// pool.go states one.
func attempt() int { return helper() + acquire() } // want `func attempt does not carry namespace "retry" anywhere in its name; give it a name that carries "retry", such as retryAttempt, or move it to the file it belongs to`

var _ = attempt

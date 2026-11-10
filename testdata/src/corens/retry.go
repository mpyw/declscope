package corens

// Not core, so an ordinary namespace: the prefix is required here, and carries
// the exportedness of the name it joins.
func WithBackoff() int { return 1 } // want `func WithBackoff does not carry namespace "retry" anywhere in its name; move it to the file it belongs to, or give it a name that carries "retry" \(RetryWithBackoff is one candidate, not necessarily the best\)`

// helper is private to the core, so naming it from here crosses a boundary:
// //declscope:core carries no scope of its own. acquire does not, because
// pool.go states one.
func attempt() int { return helper() + acquire() } // want `func attempt does not carry namespace "retry" anywhere in its name; move it to the file it belongs to, or give it a name that carries "retry" \(retryAttempt is one candidate, not necessarily the best\)`

var _ = attempt

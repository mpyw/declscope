package corens

// Not core, so an ordinary namespace: the prefix is required here, and carries
// the exportedness of the name it joins.
func WithBackoff() int { return 1 } // want `func WithBackoff does not carry namespace "retry" anywhere in its name; rename it to RetryWithBackoff, or to another name that carries "retry"`

// helper is private to the core, so naming it from here crosses a boundary:
// //declscope:core carries no scope of its own. acquire does not, because
// pool.go states one.
func attempt() int { return helper() + acquire() } // want `func attempt does not carry namespace "retry" anywhere in its name; rename it to retryAttempt, or to another name that carries "retry"`

var _ = attempt

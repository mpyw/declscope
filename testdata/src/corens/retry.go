package corens

// Not core, so an ordinary namespace: the prefix is required here, and carries
// the exportedness of the name it joins.
func WithBackoff() int { return 1 } // want `func WithBackoff does not carry the prefix of namespace "retry"; rename it to RetryWithBackoff`

// helper is private to the core, so naming it from here crosses a boundary:
// //declscope:core carries no scope of its own. acquire does not, because
// pool.go states one.
func attempt() int { return helper() + acquire() } // want `func attempt does not carry the prefix of namespace "retry"; rename it to retryAttempt`

var _ = attempt

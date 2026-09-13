package corens

// Not core, so an ordinary namespace: the label is required here, and carries
// the exportedness of the name it joins.
func WithBackoff() int { return 1 } // want `func WithBackoff does not carry the prefix of namespace "retry"; rename it to RetryWithBackoff`

func attempt() int { return 2 } // want `func attempt does not carry the prefix of namespace "retry"; rename it to retryAttempt`

var _ = attempt

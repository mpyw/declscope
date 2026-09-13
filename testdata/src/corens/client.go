//declscope:core

package corens

// The core namespace has no name, so there is no prefix to require or to drop.
// These are the package's API and are asked for nothing.
func New() int { return 1 }

type ClientConfig struct{ retries int }

func helper() int { return 2 }

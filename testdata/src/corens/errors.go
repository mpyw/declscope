//declscope:core

package corens

// A second core file shares the one core namespace, the way //declscope:namespace
// merges files under a name — the core simply has none. Sharing it is what makes
// "unlabeled" still name exactly one unit, and lets this file reach client.go's
// private helper.
var ErrClosed = helper()

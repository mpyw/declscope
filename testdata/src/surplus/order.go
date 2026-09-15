// The file's directive widens everything the file declares, and is judged as
// the one comment it is.
//
//declscope:package // want `the file's //declscope:package: no use from another namespace is visible to declscope`

package surplus

func orderQuiet() int { return 5 }

var _ = orderQuiet

//declscope:namespace _foo

package uncapitalizable

// _foo is a valid namespace, but _ has no capital form either, so Save is not
// asked to become _fooSave.
func Save() int { return 1 }

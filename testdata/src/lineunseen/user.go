//line gen/user.tmpl:1
package lineunseen

// A //line directive above the package clause renames the file itself. The
// file is still the one the pass holds, so nothing is withheld.
//
//declscope:shared // want `//declscope:shared on userQuiet: no use from another namespace is visible to declscope`
func userQuiet() int { return 1 }

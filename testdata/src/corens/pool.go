//declscope:core
//declscope:shared

package corens

// The two directives are orthogonal. //declscope:core decides the namespace;
// //declscope:shared decides the scope. A file may carry both, and then what
// it declares is core and package-wide at once.
func acquire() int { return 3 }

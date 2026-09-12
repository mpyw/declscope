// Package nsindoc checks the placement Go's own doc comment convention
// prescribes for directives: at the bottom of the package comment, preceded by
// a blank comment line. go/doc strips directive lines, so the rendered package
// documentation is unaffected.
//
//declscope:namespace shared
package nsindoc

func helper() int { return 1 }

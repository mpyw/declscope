//declscope:namespace shared

// Package nsdetached checks that a namespace directive is recognised when a
// blank line separates it from the package clause, which is how a file that
// already carries a package doc comment keeps the two apart.
package nsdetached

func helper() int { return 1 }

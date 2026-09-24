//declscope:ignore directive

package ignoredirective

// A report about a directive hangs on a comment, not on a declaration, so a
// declaration-level ignore can never answer one written where no declaration
// is. The file-level ignore above stands the rule down for the whole file,
// which is why nothing below is reported.
func fileLoose() int {
	//declscope:package
	return 1
}

// The unused-ignore report is the unused rule's, not the directive rule's, so
// the file-level ignore above does not answer it.
//
//declscope:ignore boundary // want `unused //declscope:ignore boundary on fileLoud`
func fileLoud() int { return 2 }

var _ = fileLoose() + fileLoud()

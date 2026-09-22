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

// The same ignore answers the unused-ignore report, which is itself a report
// about a directive. Silencing it is the job it did, so it is not turned
// around and called unused.
//
//declscope:ignore boundary
func fileQuiet() int { return 2 }

var _ = fileLoose() + fileQuiet()

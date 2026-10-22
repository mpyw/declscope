package coretestsubject

// client_test.go is in the core, but this file is not its test, so it keeps
// its own namespace. Joining the core through its test file made the test
// variant report this crossing as private to the core, and a baseline record
// the one declaration under two keys.
func clientHelper() int { return 1 } // want `func clientHelper is private to namespace "client", but is used from namespace "order"`

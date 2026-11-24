package camel

// account is listed under userId, which names this file's namespace userID.
func accountName() string { return "a" }

// A name outside the vocabulary is asked as usual.
func displayName() string { return "d" } // want `func displayName does not carry namespace "userID" anywhere in its name`

var _, _ = accountName, displayName

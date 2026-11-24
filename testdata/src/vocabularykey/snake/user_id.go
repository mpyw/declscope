package snake

// member is listed under user_id, which names this file's namespace userID.
func memberName() string { return "m" }

// A name outside the vocabulary is asked as usual.
func displayName() string { return "d" } // want `func displayName does not carry namespace "userID" anywhere in its name`

var _, _ = memberName, displayName

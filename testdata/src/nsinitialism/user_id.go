package nsinitialism

// The namespace of user_id.go is userID, spelled the way Go spells the
// initialism, and the prefix is matched ignoring case: every spelling below
// carries it.
func userIDCache() int { return 1 }
func userIdCache() int { return 2 }
func userIdcache() int { return 3 }

// The name is the namespace, in another spelling, and qualify accepts it.
var userId = 4

// The suggestion spells the namespace and the first word of the name the way
// Go does: not userIdLookup, and not userIDUrlPath.
func lookup() int { return 5 } // want `func lookup does not carry namespace "userID" anywhere in its name; rename it to userIDLookup, or to another name that carries "userID"`

func urlPath() string { return "" } // want `func urlPath does not carry namespace "userID" anywhere in its name; rename it to userIDURLPath, or to another name that carries "userID"`

// The namespace opens the name and the match may run on into the rest of the
// word, so this carries userID even though userid is one word here.
func useridentity() int { return 6 }

func UserExported() int {
	return userIDCache() + userIdCache() + userIdcache() + userId + lookup() + len(urlPath()) + useridentity()
}

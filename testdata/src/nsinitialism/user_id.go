package nsinitialism

// The namespace of user_id.go is userID, spelled the way Go spells the
// initialism, and the prefix is matched ignoring case: every spelling below
// carries it.
func userIDCache() int { return 1 }
func userIdCache() int { return 2 }
func userIdcache() int { return 3 }

// The name is the namespace, in another spelling. It carries no prefix to
// drop and qualify accepts it.
var userId = 4

// The suggestion spells the namespace and the first word of the name the way
// Go does: not userIdLookup, and not userIDUrlPath.
func lookup() int { return 5 } // want `func lookup does not carry the prefix of namespace "userID"; rename it to userIDLookup`

func urlPath() string { return "" } // want `func urlPath does not carry the prefix of namespace "userID"; rename it to userIDURLPath`

// Not a prefix: userid is one word here and lowercase continues it.
func useridentity() int { return 6 } // want `func useridentity does not carry the prefix of namespace "userID"; rename it to userIDUseridentity`

func UserExported() int {
	return userIDCache() + userIdCache() + userIdcache() + userId + lookup() + len(urlPath()) + useridentity()
}

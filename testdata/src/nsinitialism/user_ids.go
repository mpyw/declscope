package nsinitialism

// The namespace of user_ids.go is userIDs, spelled the way Go spells a plural
// initialism, not userIds.
func load() int { return 1 } // want `func load does not carry namespace "userIDs" anywhere in its name; rename it to userIDsLoad, or to another name that carries "userIDs"`

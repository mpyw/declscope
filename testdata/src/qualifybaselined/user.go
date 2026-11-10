package qualifybaselined

// orderA.help is recorded in the baseline, so it stays quiet.
func (orderA) help() int { return 1 }

// orderB.help shares its name and namespace but not its receiver, so the
// entry for orderA.help does not absorb it.
func (orderB) help() int { return 2 } // want `method help does not carry namespace "user" anywhere in its name; give it a name that carries "user", such as userHelp, or move it to the file it belongs to`

var _, _ = orderA{}.help, orderB{}.help

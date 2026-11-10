package fixforeignmethod

// help satisfies orderHelper. Renamed, orderT would no longer implement it,
// and the assignment in order.go would not compile.
func (o orderT) help() int { return 1 } // want `method help does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userHelp is one candidate, not necessarily the best\)`

// assert is asked for only by a type assertion. Renamed, the program would
// still compile, but the assertion would fail where it now succeeds.
func (o orderT) assert() int { return 2 } // want `method assert does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userAssert is one candidate, not necessarily the best\)`

// check would become userCheck, which orderChecker asks for. Renamed, orderT
// would start satisfying it, and the assertion would succeed where it now fails.
func (o orderT) check() bool { return true } // want `method check does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userCheck is one candidate, not necessarily the best\)`

// size would become userSize, the name of a field orderT already has.
func (o orderT) size() int { return 3 } // want `method size does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userSize is one candidate, not necessarily the best\)`

// depth would become userDepth, a field of orderOuter, which promotes depth.
func (o orderT) depth() int { return 4 } // want `method depth does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userDepth is one candidate, not necessarily the best\)`

// width would become userWidth, a field of the unnamed struct orderAnon has.
func (o *orderT) width() int { return 5 } // want `method width does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userWidth is one candidate, not necessarily the best\)`

// spare is free on orderT, on every type promoting it, and in every interface,
// so it is renamed.
func (o orderT) spare() int { return 6 } // want `method spare does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userSpare is one candidate, not necessarily the best\)`

func userCall(o orderOuter) int { return o.help() + o.spare() }

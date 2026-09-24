package generatedowner

// A method on a type declared in a generated file has no second file to be
// foreign to: the generated file is not read, so the naming rule leaves the
// method alone, as it does a method beside its type.
func (Gen) run() int { return 1 }

var _ = Gen{}.run

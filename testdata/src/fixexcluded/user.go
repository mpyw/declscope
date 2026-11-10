package fixexcluded

// load is named from excluded.go, which this build configuration leaves out.
// Renaming it here would leave that file calling a name that no longer
// exists, so the rename is withheld and only the violation is reported.
func load() int { return 1 } // want `func load does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userLoad is one candidate, not necessarily the best\)`

// userSpare is already declared in excluded.go, so claiming that name would
// declare it twice in the configuration that includes the file.
func spare() int { return 2 } // want `func spare does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userSpare is one candidate, not necessarily the best\)`

// Named nowhere outside this file, so nothing the excluded file writes is
// disturbed and the rename is offered.
func free() int { return 3 } // want `func free does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userFree is one candidate, not necessarily the best\)`

func Exported() int { return load() + spare() + free() }

package fixexcluded

// load is named from excluded.go, which this build configuration leaves out.
// Renaming it here would leave that file calling a name that no longer
// exists, so the rename is withheld and only the violation is reported.
func load() int { return 1 } // want `func load does not carry namespace "user" anywhere in its name; rename it to userLoad`

// userSpare is already declared in excluded.go, so claiming that name would
// declare it twice in the configuration that includes the file.
func spare() int { return 2 } // want `func spare does not carry namespace "user" anywhere in its name; rename it to userSpare`

// Named nowhere outside this file, so nothing the excluded file writes is
// disturbed and the rename is offered.
func free() int { return 3 } // want `func free does not carry namespace "user" anywhere in its name; rename it to userFree`

func Exported() int { return load() + spare() + free() }

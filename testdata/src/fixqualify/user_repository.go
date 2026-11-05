package fixqualify

// Both names merge to userRepositoryLoad. The first in source order takes it,
// and the second is offered the plain prefix instead of no fix.
func load() int { return 6 } // want `func load does not carry namespace "userRepository" anywhere in its name; rename it to userRepositoryLoad, or to another name that carries "userRepository"`

func repositoryLoad() int { return 8 } // want `func repositoryLoad does not carry namespace "userRepository" anywhere in its name; rename it to userRepositoryRepositoryLoad, or to another name that carries "userRepository"`

// The name opens with the word that opens the namespace, so it is prefixed
// whole: putting the rest of the namespace after it would change what it says.
func userLoad() int { return 9 } // want `func userLoad does not carry namespace "userRepository" anywhere in its name; rename it to userRepositoryUserLoad, or to another name that carries "userRepository"`

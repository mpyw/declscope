package fixqualify

// Both names merge to userRepositoryLoad. The first in source order takes it,
// and the second is offered the plain prefix instead of no fix.
func load() int { return 6 } // want `func load does not carry namespace "userRepository" anywhere in its name; move it to the file it belongs to, or give it a name that carries "userRepository" \(userRepositoryLoad is one candidate, not necessarily the best\)`

func repositoryLoad() int { return 8 } // want `func repositoryLoad does not carry namespace "userRepository" anywhere in its name; move it to the file it belongs to, or give it a name that carries "userRepository" \(userRepositoryRepositoryLoad is one candidate, not necessarily the best\)`

// The name opens with the word that opens the namespace, so it is prefixed
// whole: putting the rest of the namespace after it would change what it says.
func userLoad() int { return 9 } // want `func userLoad does not carry namespace "userRepository" anywhere in its name; move it to the file it belongs to, or give it a name that carries "userRepository" \(userRepositoryUserLoad is one candidate, not necessarily the best\)`

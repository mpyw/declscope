package surplusstrictdefaults

// Under defaults.unexported: package the field would be package-scoped with
// no directive at all, so the type's directive widened nothing and the rule
// has nothing to say.
//
//declscope:package
type userCard struct {
	id   int
	note int
}

func userRead(c userCard) int { return c.note }

var _ = userRead

package surplusstrictdefaults

// Under defaults.unexported: shared the field would be shared with
// no directive at all, so the type's directive widened nothing and the rule
// has nothing to say.
//
//declscope:shared
type userCard struct {
	id   int
	note int
}

func userRead(c userCard) int { return c.note }

var _ = userRead

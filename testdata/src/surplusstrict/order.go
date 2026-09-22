package surplusstrict

func orderRun(a userAccount, p userPair, m userMixed, n userNeedless, q userQuiet, h userHushed) int {
	var s userSaver
	_ = s
	return a.id + len(a.Name) + p.shared + m.top + n.read + len([]any{q, h})
}

var _ = orderRun

func orderHelpers() int { return helperShared() + helperC + helperMakePair().x + blockUsed }

var _ = orderHelpers

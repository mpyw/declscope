package surplusstrictreach

type orderOuter struct{ userInner }

type orderSnap struct{ rev int }

type orderGenSnap struct{ gen int }

func (b *userBox) orderBump() { b.v++ }

func orderRun() int {
	r := userRec{1, 2}
	o := orderOuter{}
	l := userList[int]{}
	s := orderSnap(userModel{})
	p := userPair[string]{1, "x"}
	g := orderGenSnap(userGenModel[int]{})
	var b userBox
	b.orderBump()
	return len([]any{r, p}) + o.n + len(l.items) + s.rev + g.gen
}

var _ = orderRun

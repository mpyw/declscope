package surplusstrictreach

type orderOuter struct{ userInner }

type orderSnap struct{ rev int }

type orderGenSnap struct{ gen int }

type orderPtrSnap struct{ rev int }

func (b *userBox) orderBump() { b.v++ }

func orderRun() int {
	r := userRec{1, 2}
	o := orderOuter{}
	l := userList[int]{}
	s := orderSnap(userModel{})
	p := userPair[string]{1, "x"}
	g := orderGenSnap(userGenModel[int]{})
	pr := []*userPtrRec{{1}}
	ps := (*orderPtrSnap)(&userPtrModel{})
	var b userBox
	b.orderBump()
	return len([]any{r, p}) + o.n + len(l.items) + s.rev + g.gen + len(pr) + ps.rev
}

var _ = orderRun

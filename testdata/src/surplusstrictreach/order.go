package surplusstrictreach

type orderOuter struct{ userInner }

type orderSnap struct{ rev int }

func (b *userBox) orderBump() { b.v++ }

func orderRun() int {
	r := userRec{1, 2}
	o := orderOuter{}
	l := userList[int]{}
	s := orderSnap(userModel{})
	var b userBox
	b.orderBump()
	return len([]any{r}) + o.n + len(l.items) + s.rev
}

var _ = orderRun

package ifacemembers

func orderRun(s Sealed, w Widened, m Mixed, q Quiet, st Store[int]) int {
	if s.sealed() {
		return w.shared() + m.ok() + m.nope() + q.hushed() + st.fetch(1) + s.Open()
	}
	return 0
}

var _ = orderRun

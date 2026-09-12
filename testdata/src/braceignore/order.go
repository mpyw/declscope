package braceignore

func orderRun(t *UserT, v *UserV, w *UserW, m *UserM) int {
	return t.x + v.z + w.w + w.v + m.m + userA + userB + userC + userHelper() + userLoose()
}

var _ = orderRun

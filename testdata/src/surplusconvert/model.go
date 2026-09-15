package surplusconvert

type model struct {
	ID int
	//declscope:package
	rev int
}

//declscope:package
func modelNew() model { return model{ID: 1, rev: 2} }

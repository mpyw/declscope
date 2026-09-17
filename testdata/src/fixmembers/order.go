package fixmembers

func orderRun() string {
	e := entry{key: "x"}
	return e.key
}

func orderRead() bool { return UserMake().flag }

var _, _ = orderRun, orderRead

//declscope:shared

package surplusdefaultstrict

func userShared() int { return 1 }

func userLocal() int { return 2 } // want `func userLocal takes shared scope from the file's //declscope:shared, but no use from another namespace is visible to declscope`

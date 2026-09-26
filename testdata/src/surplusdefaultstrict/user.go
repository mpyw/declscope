//declscope:package

package surplusdefaultstrict

func userShared() int { return 1 }

func userLocal() int { return 2 } // want `func userLocal takes package scope from the file's //declscope:package, but no use from another namespace is visible to declscope`

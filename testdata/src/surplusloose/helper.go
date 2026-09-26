// helperShared is called from order.go, so the directive is in use, and under
// loose, that is all the rule asks. helperLocal is not reported;
// strict is what judges it on its own.
//
//declscope:package

package surplusloose

func helperShared() int { return helperLocal() }

func helperLocal() int { return 1 }

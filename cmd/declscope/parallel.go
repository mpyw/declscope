package main

import (
	"runtime"
	"sync"
)

// inParallel calls f once for every index below n, on up to GOMAXPROCS
// goroutines at a time, and returns once every call has.
//
// Loading type-checks in parallel already, and on a large module the analysis
// that follows is a quarter of the run: 0.7s of 2.5s over the standard
// library. Each caller writes one package's result to that package's own slot
// and folds the slots in order afterwards, so no call shares state with
// another and the output does not depend on scheduling. The analysis is safe
// to run this way: singlechecker already runs it on many packages at once.
//
//declscope:shared // survey and baseline analyze every package through it
func inParallel(n int, f func(i int)) {
	slots := make(chan struct{}, runtime.GOMAXPROCS(0))
	var wg sync.WaitGroup
	for i := range n {
		slots <- struct{}{}
		wg.Go(func() {
			defer func() { <-slots }()
			f(i)
		})
	}
	wg.Wait()
}

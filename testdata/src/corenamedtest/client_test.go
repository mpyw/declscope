//declscope:namespace probe

package corenamedtest

// This file shares client.go's stem but names its own namespace. Only a file
// with no namespace directive joins the core, so the directive holds and
// probeHelper is private to probe. The report sits in a test file, which only
// the test variant reads, so it is pinned there alone.
func probeHelper() int { return New() } // want `func probeHelper is private to namespace "probe", but is used from the core namespace`

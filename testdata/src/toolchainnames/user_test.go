package toolchainnames

import "testing"

// The toolchain finds each of these by name. Prefixing one leaves a function
// nothing runs, so none is asked for a prefix.
func TestLoad(t *testing.T)      { _ = userLoad() }
func BenchmarkLoad(b *testing.B) { _ = userLoad() }
func FuzzLoad(f *testing.F)      { _ = userLoad() }
func ExampleTestHelper()         { _ = TestHelper() }

package surplussatisfies

type readCloser interface {
	read() string
	close() string
}

type intGetter interface{ get() int }

type runner interface{ run() }

// An anonymous struct satisfies through embedded methods, spelling none of
// them.
func useBoth() string {
	var rc readCloser = struct {
		reader
		closer
	}{}
	return rc.read() + rc.close()
}

// A type declared inside a function does the same, and appears in no package
// scope.
func useLocal() string {
	type both struct {
		reader
		closer
	}
	var rc readCloser = both{}
	return rc.read() + rc.close()
}

// An instantiated generic type satisfies with an instantiated method, which is
// a different object from the one the source declares.
func useGet() int {
	var g intGetter = box[int]{v: 42}
	return g.get()
}

// A promoted instantiation satisfies through the embedding.
type outer struct{ box[int] }

var _ intGetter = outer{}

// A pointer-receiver method promotes only through the address.
type holder struct{ inner }

var _ runner = &holder{}

// An anonymous interface in a type assertion is a contract too.
func useRun(v any) {
	if r, ok := v.(interface{ run() }); ok {
		r.run()
	}
}

// A constraint carrying a type term is an interface contract as well.
func useFormat[T interface {
	~int
	str() string
}](t T) string {
	return t.str()
}

var (
	_ = useBoth
	_ = useLocal
	_ = useGet
	_ = useRun
	_ = useFormat[myInt]
)

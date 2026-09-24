package directiveloose

// rules.directive is loose by default. A private field of a type that states
// no scope names the scope defaults.unexported gives it today, but another
// configuration could give it package, so the directive is not reported.
type implicit struct {
	//declscope:private
	x int
}

// Restating the type's directive decides nothing under any configuration, and
// loose reports it.
//
//declscope:private
type explicit struct {
	//declscope:private // want `unused //declscope:private on explicit.x: nothing it reaches takes a scope`
	x int
}

var _ = implicit{}.x + explicit{}.x

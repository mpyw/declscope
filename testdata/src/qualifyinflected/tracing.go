package qualifyinflected

// tracing is trace + ing with the e dropped, but generation never runs back
// from tracing to trace: the stem would be a guess. The message points at the
// vocabulary instead, since the rename would stutter.
func traceValue() int { return 1 } // want `func traceValue does not carry namespace "tracing" anywhere in its name; rename it to tracingTraceValue, or to another name that carries "tracing"; if the name spells "tracing" in another form, list that form under rules.naming.vocabulary.tracing`

// The free right edge runs only from the namespace, so tracer does not carry
// tracing either.
type tracerType struct{} // want `type tracerType does not carry namespace "tracing" anywhere in its name; rename it to tracingTracerType, or to another name that carries "tracing"; if the name spells "tracing" in another form, list that form under rules.naming.vocabulary.tracing`

// The namespace's own spelling carries it as usual.
func tracingStart() int { return 2 }

var _, _ = traceValue, tracingStart
var _ tracerType

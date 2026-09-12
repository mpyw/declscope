package generated

// Generated files are excluded entirely, so referencing genSecret from another
// namespace produces no diagnostic.
func useRun() int { return genSecret() }

var _ = useRun

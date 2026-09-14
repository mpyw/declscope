package exportedscope

// An exported declaration has no boundary by default. Asking instead whether it
// is reachable from outside the package is a question a single-package analysis
// cannot answer — a type escapes through an exported signature, an embedding,
// an alias, or an interface it satisfies — and guessing reported boundaries on
// API that every importer reaches.
type Config struct{ Timeout int }

func Open() *Config { return nil }

// An author may still state the boundary the analysis will not guess. This is a
// statement, not a guess, so it binds and it is enforced.
//
//declscope:private
func Seal() int { return 1 } // want `func Seal is declared private by //declscope:private, but is used from namespace "use"`

// A DTO whose fields are capitalized for a serializer rather than for an
// audience says so on the type, and the directive reaches the fields.
//
//declscope:private
type entry struct { // want `type entry is declared private by //declscope:private, but is used from namespace "use"` `type entry does not carry namespace "api" anywhere in its name; rename it to apiEntry`
	Key   string `json:"key"` // want `field entry.Key is declared private by //declscope:private on entry, but is used from namespace "use"`
	Value []byte `json:"value"`
}

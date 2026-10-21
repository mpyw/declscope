package rule

import (
	"strings"
)

// QualifyMode says when a name must carry its namespace: always, never, or
// only once a package has a second namespace. Only rules.naming.qualify reads
// one.
type QualifyMode int

const (
	// QualifyModeNever disables the rule.
	QualifyModeNever QualifyMode = iota

	// QualifyModeAlways applies the rule to every package. For the naming
	// rule this means a package gaining its second namespace is not a mass
	// rename.
	QualifyModeAlways

	// QualifyModeOnDemand applies the rule only to a package with more than
	// one namespace. In a package with one there is no boundary for a prefix to
	// mark: every other rule is structurally inert there, since every
	// reference is already inside the single namespace, and a prefix repeated
	// on every declaration would distinguish nothing.
	QualifyModeOnDemand
)

// String returns the spelling the settings use.
func (m QualifyMode) String() string {
	switch m {
	case QualifyModeNever:
		return "never"
	case QualifyModeAlways:
		return "always"
	case QualifyModeOnDemand:
		return "ondemand"
	default:
		return "unknown"
	}
}

// Applies reports whether the rule applies to a package with the given number
// of namespaces.
func (m QualifyMode) Applies(namespaces int) bool {
	switch m {
	case QualifyModeAlways:
		return true
	case QualifyModeOnDemand:
		return namespaces > 1
	default:
		return false
	}
}

// ModeSet is the values one setting accepts, in the order an error message
// names them. Each setting declares its own, so that a rejected value is
// answered with what that setting accepts rather than with everything its mode
// type can hold.
//
// It is generic over the mode types, which stay separate: parsing by spelling
// and listing the spellings is all they share.
type ModeSet[M interface {
	~int
	String() string
}] []M

// Parse reads a setting's value, of the members of the set only.
func (s ModeSet[M]) Parse(value string) (M, bool) {
	for _, m := range s {
		if value == m.String() {
			return m, true
		}
	}
	return 0, false
}

// String lists the accepted spellings the way an error message names them:
// "always, never or ondemand".
func (s ModeSet[M]) String() string {
	names := make([]string, len(s))
	for i, m := range s {
		names[i] = m.String()
	}
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " or " + names[len(names)-1]
}

// SurplusMode says how much the surplus rule reports: nothing, a whole
// directive nothing needs, or also each declaration a directive widens for
// nothing. Only rules.surplus reads one.
//
// It is QualifyMode's sibling rather than three more QualifyMode values.
// QualifyMode carries a predicate, Applies, that asks how many namespaces a
// package has, and that question means nothing here; a QualifyMode that could
// hold loose would need an answer for it anyway. What the two share — parse by
// spelling, and list the spellings in an error — is small, and ModeSet is where
// it is shared.
type SurplusMode int

const (
	// SurplusModeLoose reports a //declscope:shared when nothing that takes
	// its scope from it is reached from another namespace.
	SurplusModeLoose SurplusMode = iota

	// SurplusModeOff reports nothing.
	SurplusModeOff

	// SurplusModeStrict reports what loose does, and also each declaration
	// that takes shared scope from an enclosing directive which is otherwise
	// in use, when nothing reaches that declaration from another namespace. It
	// is the default.
	SurplusModeStrict
)

// String returns the spelling the settings use.
func (m SurplusMode) String() string {
	switch m {
	case SurplusModeOff:
		return "off"
	case SurplusModeLoose:
		return "loose"
	case SurplusModeStrict:
		return "strict"
	default:
		return "unknown"
	}
}

// Reports reports whether the rule says anything at all.
func (m SurplusMode) Reports() bool { return m == SurplusModeLoose || m == SurplusModeStrict }

// ReportsDeclarations reports whether the rule also judges each declaration a
// directive in use widens, not only the directive as a whole.
func (m SurplusMode) ReportsDeclarations() bool { return m == SurplusModeStrict }

// BoundaryMode says whether the boundary rule reports: on or off. Only
// rules.boundary reads one.
//
// A mode rather than a boolean, so that the key spells the rule's own name:
// boundary: true would read as "boundary: yes please", which is why the
// switch used to be named for what it did (allowBoundary). With a mode, the
// rule's name, its diagnostic category, its baseline key, its ignore target
// and its configuration key are one word, as they are for surplus.
type BoundaryMode int

const (
	// BoundaryModeOn reports a private declaration used from outside its
	// namespace. It is the default.
	BoundaryModeOn BoundaryMode = iota

	// BoundaryModeOff reports nothing, leaving only the naming rule.
	BoundaryModeOff
)

// String returns the spelling the settings use.
func (m BoundaryMode) String() string {
	switch m {
	case BoundaryModeOn:
		return "on"
	case BoundaryModeOff:
		return "off"
	default:
		return "unknown"
	}
}

// Reports reports whether the rule says anything at all.
func (m BoundaryMode) Reports() bool { return m == BoundaryModeOn }

// UnusedMode says when the unused rule reports a directive: never, only when
// no configuration could make a scope directive bind, or also when it names
// the scope the declaration would have without it under this one. Only
// rules.unused reads one.
//
// Its off reaches the unused rule alone. Malformed, unknown, conflicting and
// misplaced directives are the directive rule's, which has no switch.
type UnusedMode int

const (
	// UnusedModeLoose reports an ignore that silenced nothing, and a scope
	// directive only when the scope it names is one every declaration it
	// reaches would have under every configuration.
	UnusedModeLoose UnusedMode = iota

	// UnusedModeOff reports nothing.
	UnusedModeOff

	// UnusedModeStrict also reports a scope directive that names, for
	// everything it reaches, the scope that declaration would have without it
	// under the configuration in force, and offers to delete it. It is the
	// default.
	UnusedModeStrict
)

// String returns the spelling the settings use.
func (m UnusedMode) String() string {
	switch m {
	case UnusedModeOff:
		return "off"
	case UnusedModeLoose:
		return "loose"
	case UnusedModeStrict:
		return "strict"
	default:
		return "unknown"
	}
}

// Reports reports whether the rule says anything at all.
func (m UnusedMode) Reports() bool { return m == UnusedModeLoose || m == UnusedModeStrict }

// ReportsRedundant reports whether a directive restating the scope the
// configuration already gives is reported.
func (m UnusedMode) ReportsRedundant() bool { return m == UnusedModeStrict }

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

// QualifyModeSet is the values one setting accepts, in the order an error
// message names them. Each setting declares its own, so that a rejected value
// is answered with what that setting accepts rather than with everything
// QualifyMode can hold.
type QualifyModeSet []QualifyMode

// Parse reads a setting's value: always, never or ondemand, and of those only
// the members of the set.
func (s QualifyModeSet) Parse(value string) (QualifyMode, bool) {
	for _, m := range s {
		if value == m.String() {
			return m, true
		}
	}
	return 0, false
}

// String lists the accepted spellings the way an error message names them:
// "always, never or ondemand".
func (s QualifyModeSet) String() string {
	names := make([]string, len(s))
	for i, m := range s {
		names[i] = m.String()
	}
	return modeJoin(names)
}

// modeJoin spells a list of accepted values the way an error message names
// them: "a, b or c".
func modeJoin(names []string) string {
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
// hold loose would need an answer for it anyway. What the two share — parse by spelling, and list the
// spellings in an error — is small, and modeJoin is where it is shared.
type SurplusMode int

const (
	// SurplusModeLoose reports a //declscope:package when nothing that takes
	// its scope from it is reached from another namespace. It is the default.
	SurplusModeLoose SurplusMode = iota

	// SurplusModeOff reports nothing.
	SurplusModeOff

	// SurplusModeStrict reports what loose does, and also each declaration
	// that takes package scope from an enclosing directive which is otherwise
	// in use, when nothing reaches that declaration from another namespace.
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

// SurplusModeSet is the values rules.surplus accepts, in the order an error
// message names them.
type SurplusModeSet []SurplusMode

// Parse reads a setting's value, of the members of the set only.
func (s SurplusModeSet) Parse(value string) (SurplusMode, bool) {
	for _, m := range s {
		if value == m.String() {
			return m, true
		}
	}
	return 0, false
}

// String lists the accepted spellings: "off, loose or strict".
func (s SurplusModeSet) String() string {
	names := make([]string, len(s))
	for i, m := range s {
		names[i] = m.String()
	}
	return modeJoin(names)
}

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

// BoundaryModeSet is the values rules.boundary accepts, in the order an error
// message names them.
type BoundaryModeSet []BoundaryMode

// Parse reads a setting's value, of the members of the set only.
func (s BoundaryModeSet) Parse(value string) (BoundaryMode, bool) {
	for _, m := range s {
		if value == m.String() {
			return m, true
		}
	}
	return 0, false
}

// String lists the accepted spellings: "off or on".
func (s BoundaryModeSet) String() string {
	names := make([]string, len(s))
	for i, m := range s {
		names[i] = m.String()
	}
	return modeJoin(names)
}

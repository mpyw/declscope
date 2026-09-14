package internal

import (
	"strings"
)

// Mode says when a name must carry its namespace: always, never, or only once
// a package has a second namespace. Only rules.qualify reads one.
type Mode int

const (
	// ModeNever disables the rule.
	ModeNever Mode = iota

	// ModeAlways applies the rule to every package. For the naming rule this
	// means a package gaining its second namespace is not a mass rename.
	ModeAlways

	// ModeOnDemand applies the rule only to a package with more than one
	// namespace. In a package with one there is no boundary for a prefix to
	// mark: every other rule is structurally inert there, since every
	// reference is already inside the single namespace, and a prefix repeated
	// on every declaration would distinguish nothing.
	ModeOnDemand
)

// String returns the spelling the settings use.
func (m Mode) String() string {
	switch m {
	case ModeNever:
		return "never"
	case ModeAlways:
		return "always"
	case ModeOnDemand:
		return "ondemand"
	default:
		return "unknown"
	}
}

// Applies reports whether the rule applies to a package with the given number
// of namespaces.
func (m Mode) Applies(namespaces int) bool {
	switch m {
	case ModeAlways:
		return true
	case ModeOnDemand:
		return namespaces > 1
	default:
		return false
	}
}

// ModeSet is the values one setting accepts, in the order an error message
// names them. Each setting declares its own, so that a rejected value is
// answered with what that setting accepts rather than with everything Mode
// can hold.
type ModeSet []Mode

// Parse reads a setting's value: always, never or ondemand, and of those only
// the members of the set.
func (s ModeSet) Parse(value string) (Mode, bool) {
	for _, m := range s {
		if value == m.String() {
			return m, true
		}
	}
	return 0, false
}

// String lists the accepted spellings the way an error message names them:
// "always, never or ondemand".
func (s ModeSet) String() string {
	names := make([]string, len(s))
	for i, m := range s {
		names[i] = m.String()
	}
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " or " + names[len(names)-1]
}

// Options is the resolved configuration for a run.

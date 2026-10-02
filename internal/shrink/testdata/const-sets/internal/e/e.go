// Package e exercises const sets: a const block of two or more constants,
// every one of them of one exported type of the package, is left alone or
// unexported as a whole.
package e

import "time"

// RequestStatus is used through its values. The values nothing outside uses
// stay exported with them.
type RequestStatus string

const (
	StUnclaimed RequestStatus = "unclaimed"
	StClaimed   RequestStatus = "claimed"
	StDone      RequestStatus = "done"
	StWithdrawn RequestStatus = "withdrawn"
)

// ItemStatus is named outside, and none of its values is: they stay exported
// with it, implicit types included.
type ItemStatus int

const (
	_ ItemStatus = iota
	ItemInFridge
	ItemUsedUp
)

// Due is named outside. A lone constant of it is an internal choice, not a
// value of the set, and a block mixing in another constant is not a set.
type Due string

const (
	DueSoon Due = "soon"
	DueWeek Due = "week"
)

const DefaultDue Due = DueWeek // want: const DefaultDue is exported, but nothing

const (
	MixedDue Due = "mixed" // want: const MixedDue is exported, but nothing
	Mixed        = 10
)

// A block of one type from another package is not a set.
const (
	Day  time.Duration = 24 * time.Hour // want: const Day is exported, but nothing
	Week time.Duration = 7 * Day
)

// Lonely is reported, and so is every value of its set: they go together.
type Lonely int // want: type Lonely is exported, but nothing

const (
	LonelyA Lonely = iota // want: const LonelyA is exported, but nothing outside \S+ uses it$
	LonelyB               // want: const LonelyB is exported, but nothing outside \S+ uses it$
)

// Held is reported, and one of its values keeps its name, so every other one
// does too, and so does the type the value hands out.
type Held int // want: type Held is exported.*no fix: an exported declaration that keeps its name hands it out

const (
	//declscope:ignore overexported // kept on purpose
	HeldA Held = iota
	HeldB // want: const HeldB is exported.*no fix: another value of its const block keeps its name
)

// The type is silenced, so its set is left alone, and an ignore over a value
// of it silences nothing.
//
//declscope:ignore overexported // kept on purpose
type Silent int

const (
	SilentA Silent = iota
	//declscope:ignore overexported // want: unused //declscope:ignore overexported
	SilentB
)

// Excluded is reported, since nothing names it. A build-excluded file uses one
// of its values, which keeps the set exported, and keeps the type that the
// value hands out.
type Excluded int // want: type Excluded is exported.*no fix: an exported declaration that keeps its name hands it out

const (
	ExcludedA Excluded = iota
	ExcludedB
)

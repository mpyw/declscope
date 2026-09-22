// Package measure holds what `declscope survey` and `declscope inspect`
// report: the shape of a package, rather than its violations alone.
//
// The analyzer answers one question per declaration and reports what fails.
// That leaves two things it never says out loud. One is the crossing itself —
// which namespace reached which, how much of it, and whether anyone decided
// that it was allowed. The other is the state of the checks: a count of zero
// means nothing until the reader knows the rule was in force, the build
// succeeded and no baseline was absorbing the answer.
//
// Nothing here defines a violation. Every value is an aggregation of what the
// analyzer already found, which is why the model carries the state a finding
// ended in rather than re-judging it.
//
// The file joins the core namespace for the reason subject.go does: every
// renderer reads this model, and in a named namespace each type would have to
// spell that namespace into its own name. It states no scope: everything here
// is exported, which already resolves to package scope, so a
// //declscope:package would decide nothing. The core is about the naming rule,
// which asks nothing of a namespace that has no name.
//
//declscope:core

package measure

import (
	"cmp"
	"maps"
	"slices"

	"github.com/mpyw/declscope/internal/rule"
)

// EdgeState is what became of one declaration reached from another namespace.
//
// The four states that matter are Declared, Baselined, Reported and Open: a
// decision recorded, a decision deferred, a decision outstanding, and no
// decision asked for. Ignored and Unchecked exist because the data has them —
// a crossing can be silenced by a directive, and rules.allowBoundary switches
// the question off entirely — and folding either into one of the four would
// report a state nobody is in.
type EdgeState string

const (
	// EdgeDeclared is a crossing whose declaration was widened by a
	// //declscope:package directive. The decision is recorded in the source.
	EdgeDeclared EdgeState = "declared"

	// EdgeBaselined is a crossing the baseline suppresses. The decision is
	// deferred, not made.
	EdgeBaselined EdgeState = "baselined"

	// EdgeReported is a crossing the analyzer reports. Undecided and visible.
	EdgeReported EdgeState = "reported"

	// EdgeIgnored is a crossing a //declscope:ignore boundary directive
	// silences. It is a decision, recorded in the source like EdgeDeclared,
	// but it withholds the report rather than widening the declaration.
	EdgeIgnored EdgeState = "ignored"

	// EdgeOpen is a crossing of a declaration that is package-scoped without
	// any directive saying so — an exported declaration, or one widened by
	// defaults.unexported. It is a default, not a decision.
	EdgeOpen EdgeState = "open"

	// EdgeUnchecked is a crossing in a package where rules.allowBoundary
	// switched the rule off. The crossing is real and nobody asked about it.
	EdgeUnchecked EdgeState = "unchecked"
)

// FindingState is what became of one finding: silenced by a directive,
// absorbed by a baseline, or reported.
//
// It is deliberately not EdgeState. A finding is not a crossing, and the three
// states a crossing can be in without ever having been a finding — declared,
// open, unchecked — have no meaning here; typing them the same made the
// translation between them partial, with a default arm quietly answering for
// three values that cannot occur.
type FindingState string

const (
	FindingIgnored   FindingState = "ignored"
	FindingBaselined FindingState = "baselined"
	FindingReported  FindingState = "reported"
)

// NameState is what became of one declaration the naming rule examined.
type NameState string

const (
	// NameExempt is a declaration the rule reaches but a directive excuses.
	NameExempt NameState = "exempt"

	// NameBaselined is a finding the baseline suppresses.
	NameBaselined NameState = "baselined"

	// NameReported is a finding the analyzer reports.
	NameReported NameState = "reported"
)

// Finding is one violation the analyzer found, together with what became of
// it. The analyzer keeps the outcome only long enough to decide whether to
// print; this is the same decision, kept.
type Finding struct {
	Rule rule.Rule

	// Declaration is the name the baseline would key on.
	Declaration string

	// State is the outcome, in the order the report settles it: an ignore
	// directive is consulted before the baseline, so a suppression the
	// baseline would also have absorbed still counts as the directive doing
	// its job.
	State FindingState

	// Fixable records whether the analyzer offered a fix for this finding.
	// It is the analyzer's own answer, not a second opinion: the reasons a
	// rename is withheld are recorded nowhere, so only the outcome is known.
	Fixable bool
}

// Namespace is one unit of privacy within a package, with the denominators
// every ratio in the report divides by.
//
// The two counts are kept apart because the rules do not ask the same
// question of the same declarations: every declaration can be crossed, while
// the naming rule reaches package-level declarations only, and only when it
// is in force at all.
type Namespace struct {
	// Name is the namespace as the baseline spells it, so that a row here and
	// a baseline entry can be compared by eye. The core is "(core)".
	Name string

	// Files are the files making up the namespace, in source order.
	Files []string

	// Core marks the package's one unnamed namespace.
	Core bool

	// Declarations counts package-level declarations and members. It is the
	// denominator of a crossing's reach.
	Declarations int

	// QualifyTargets counts the declarations the naming rule examines here.
	// It is the denominator of saturation, and it is zero wherever the rule
	// is not in force, which is not the same as every declaration passing.
	QualifyTargets int
}

// Edge is one declaration of one namespace, reached from one other namespace.
//
// The unit is the declaration rather than the use site: "how much of this
// namespace does that one reach into" is the question the shape of a package
// turns on, and a single helper called two hundred times answers it once.
// Uses carries the depth separately.
type Edge struct {
	// From is the namespace doing the reaching, To the one being reached.
	From, To string

	// Declaration is the name reached, spelled Type.member for a member, as
	// the baseline spells it.
	Declaration string

	// Kind is what the declaration is: func, type, var, const, method, field.
	Kind string

	// Uses counts the reference sites in From. It is never the sum of the
	// state counts, which are per declaration.
	Uses int

	// State is what became of this crossing.
	State EdgeState
}

// NameFinding is one declaration the naming rule examined and was not
// satisfied by.
type NameFinding struct {
	Namespace   string
	File        string
	Declaration string
	Kind        string

	// Exported marks a declaration the rule reached only because
	// rules.naming.exported is on. No rename is ever offered for one, so a
	// consumer sorting the work by what -fix can do reads it here; no table
	// splits the count.
	Exported bool

	State NameState

	// Fixable records whether -fix would actually offer the rename, which is
	// the analyzer's own answer rather than a second judgment: the reasons a
	// rename is withheld are not recorded anywhere, so only the outcome is
	// reported.
	Fixable bool
}

// Count is one rule's tally for one package.
//
// Found is before any suppression, and Found = Ignored + Baselined + Reported
// always holds. A run where it does not is a bug.
type Count struct {
	Found     int
	Ignored   int
	Baselined int
	Reported  int

	// Asked is false when the rule was not in force: rules.allowBoundary, or
	// a qualify mode that does not apply to this package. Every number above
	// is then zero for a reason that has nothing to do with the code, so a
	// renderer prints them as "-" rather than as a clean result.
	Asked bool

	// Keyable is false for a rule whose findings carry no baseline key, which
	// is the directive rule and the filter rule. Their Baselined is zero
	// because nothing could ever suppress them.
	Keyable bool
}

// Package is the measured shape of one analyzed package.
type Package struct {
	// Path is the import path.
	Path string

	// Config is the chain of config files that governed the analysis,
	// outermost first. The producer leaves it empty: which files were
	// consulted is known to the caller that resolved them, not to the pass.
	Config []string

	// Namespaces are sorted by name, the core first.
	Namespaces []Namespace

	// Edges are sorted by From, To, then Declaration.
	Edges []Edge

	// Names are sorted by Namespace, then Declaration.
	Names []NameFinding

	// Findings is one Count per rule in rule.All.
	Findings map[rule.Rule]Count

	// AllCore marks a package whose every file joined the core namespace. The
	// boundary and naming rules then have nothing to check, though the
	// surplus, directive and filter rules still run.
	AllCore bool
}

// Checks is the state of the checks themselves, assembled by the caller that
// resolved them rather than by the analysis: which config files governed which
// packages, what each of them switched on, what a baseline is absorbing, and
// whether the code compiled at all.
//
// It is reported before any count, because a count means nothing until the
// reader knows the rule was in force. A zero from a rule that was switched
// off, from a package that did not build, or from a baseline that absorbed
// everything reads exactly like a zero from clean code.
type Checks struct {
	Configs   []ConfigUse
	Baselines []BaselineUse
	TypeCheck TypeCheck
}

// ConfigUse is one set of rules and the packages it governed.
//
// The key is the chain of files, not the resolved options: config files
// compose key by key, so naming only the nearest one is lossy, and the
// resolved options hold compiled patterns and a pointer, which cannot be
// compared.
type ConfigUse struct {
	// Chain is the config files that applied, outermost first. Empty means
	// the built-in defaults governed these packages.
	Chain []string

	// Packages is how many packages this chain governed.
	Packages int

	Boundary bool

	// Surplus is the mode as the config spells it: off, loose, strict.
	Surplus string

	// Qualify is the mode as the config spells it: always, ondemand, never.
	Qualify string

	// Exported says whether the naming rule reached exported declarations.
	Exported bool
}

// BaselineUse is one baseline file and what it holds.
type BaselineUse struct {
	Path string

	// Entries is how many violations the file records.
	Entries int
}

// TypeCheck is whether the packages compiled.
type TypeCheck struct {
	Packages int

	// Failed names the packages that did not type-check, with the first error
	// of each. A package that does not compile yields no findings, which is
	// indistinguishable from a package with nothing wrong.
	Failed []string
}

// Sorted returns the package with every slice in the one order the renderers
// and the goldens rely on.
//
// It returns a copy rather than sorting in place. Every other method on
// Package takes a value receiver, and one that took a pointer would be the
// odd one out twice over: mixing receiver kinds on a type, and reordering
// slices its caller still holds.
func (p Package) Sorted() Package {
	p.Namespaces = slices.Clone(p.Namespaces)
	p.Edges = slices.Clone(p.Edges)
	p.Names = slices.Clone(p.Names)
	// The map goes too. A copy that shared it would let a caller holding the
	// original change what the copy reports, which is the surprise this
	// method exists to avoid.
	p.Findings = maps.Clone(p.Findings)

	slices.SortFunc(p.Namespaces, func(a, b Namespace) int {
		if a.Core != b.Core {
			// The core answers for the package's own subject, so it reads
			// first rather than wherever "(core)" happens to sort.
			if a.Core {
				return -1
			}
			return 1
		}
		return cmp.Compare(a.Name, b.Name)
	})
	slices.SortFunc(p.Edges, func(a, b Edge) int {
		return cmp.Or(
			cmp.Compare(a.From, b.From),
			cmp.Compare(a.To, b.To),
			cmp.Compare(a.Declaration, b.Declaration),
		)
	})
	slices.SortFunc(p.Names, func(a, b NameFinding) int {
		return cmp.Or(
			cmp.Compare(a.Namespace, b.Namespace),
			cmp.Compare(a.Declaration, b.Declaration),
		)
	})
	return p
}

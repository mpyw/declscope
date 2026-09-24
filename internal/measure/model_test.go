package measure

import "github.com/mpyw/declscope/internal/rule"

// fixturePackage is a package shaped to exercise every state a renderer has a
// column, a note or a dash for: a core namespace the naming rule asks nothing
// of, a mutual pair, a crossing in each state, and a namespace whose names all
// fail.
//
// It is built by hand rather than analyzed, which is the point of the model
// being its own package: a renderer can be tested without building a pass.
//
//declscope:package // the goldens are rendered from it
func fixturePackage() Package {
	return Package{
		Path:   "example.com/x/internal/cmd",
		Config: []string{".declscope.yaml"},
		Namespaces: []Namespace{
			{Name: "(core)", Files: []string{"cmd.go"}, Core: true, Declarations: 4},
			{Name: "completion", Files: []string{"completion.go", "comp_zsh.go"}, Declarations: 6, QualifyTargets: 4},
			{Name: "flags", Files: []string{"flags.go"}, Declarations: 5, QualifyTargets: 2},
		},
		Edges: []Edge{
			{From: "completion", To: "flags", Declaration: "writeFlag", Kind: "func", Uses: 3, State: EdgeReported},
			{From: "completion", To: "flags", Declaration: "Flags.parsed", Kind: "field", Uses: 2, State: EdgeBaselined},
			// A pair carrying nothing but deferred crossings, so the diagram
			// draws the plain arrow. Sharing a pair with a reported crossing
			// would hide it behind the precedence.
			{From: "(core)", To: "completion", Declaration: "compBuf", Kind: "var", Uses: 2, State: EdgeBaselined},
			{From: "flags", To: "completion", Declaration: "compName", Kind: "func", Uses: 1, State: EdgeDeclared},
			// Reached from two namespaces, so the note below the table can be
			// told apart from one counting edges: one declaration silenced,
			// not two crossings.
			{From: "(core)", To: "flags", Declaration: "flagSet", Kind: "var", Uses: 4, State: EdgeIgnored},
			{From: "completion", To: "flags", Declaration: "flagSet", Kind: "var", Uses: 1, State: EdgeIgnored},
			{From: "completion", To: "(core)", Declaration: "Run", Kind: "func", Uses: 9, State: EdgeOpen},
		},
		Names: []NameFinding{
			{Namespace: "flags", File: "flags.go", Declaration: "parseAll", Kind: "func", State: NameReported, Fixable: true},
			{Namespace: "flags", File: "flags.go", Declaration: "Reset", Kind: "func", Exported: true, State: NameReported},
			{Namespace: "completion", File: "comp_zsh.go", Declaration: "zshHeader", Kind: "func", State: NameBaselined},
			{Namespace: "completion", File: "completion.go", Declaration: "compLine", Kind: "func", State: NameExempt},
		},
		Findings: map[rule.Rule]Count{
			rule.Boundary:  {Asked: true, Keyable: true, Found: 5, Ignored: 1, Baselined: 2, Reported: 2},
			rule.Qualify:   {Asked: true, Keyable: true, Found: 4, Ignored: 1, Baselined: 1, Reported: 2},
			rule.Surplus:   {Asked: true, Keyable: true},
			rule.Unused:    {Asked: true, Found: 2, Ignored: 1, Reported: 1},
			rule.Directive: {Asked: true, Found: 1, Reported: 1},
			rule.Filter:    {Asked: true},
		},
	}
}

//declscope:package // the goldens are rendered from it
func fixtureSummary() Summary {
	return SummaryOf([]Package{fixturePackage(), {
		Path:       "example.com/x/internal/legacy",
		AllCore:    true,
		Namespaces: []Namespace{{Name: "(core)", Files: []string{"a.go", "b.go"}, Core: true, Declarations: 7}},
		Findings: map[rule.Rule]Count{
			rule.Boundary: {Asked: true, Keyable: true},
			rule.Qualify:  {Keyable: true},
		},
	}, {
		// No file of this package was read at all: every one is generated, or
		// the filter removed them. Its row has to say so in every format, and
		// the two disagreed about it while no fixture held one.
		Path: "example.com/x/internal/generated",
		Findings: map[rule.Rule]Count{
			rule.Boundary: {Asked: true, Keyable: true},
			rule.Qualify:  {Keyable: true},
		},
	}}, Checks{
		Configs: []ConfigUse{{
			Chain: []string{".declscope.yaml"}, Packages: 2,
			Boundary: true, Surplus: "loose", Unused: "strict", Qualify: "ondemand", Exported: true,
		}},
		Baselines: []BaselineUse{{Path: ".declscope-baseline.yaml", Entries: 12}},
		TypeCheck: TypeCheck{Packages: 2},
	})
}

// fixtureUncheckedPackage is a package with both rules switched off: the
// crossings are real and nobody was asked about them. Every count a renderer
// prints for it has to be a dash rather than a zero.
//
//declscope:package // the goldens are rendered from it
func fixtureUncheckedPackage() Package {
	return Package{
		Path: "example.com/x/internal/legacy",
		Namespaces: []Namespace{
			{Name: "api", Files: []string{"api.go"}, Declarations: 3},
			{Name: "store", Files: []string{"store.go"}, Declarations: 2},
		},
		Edges: []Edge{
			{From: "api", To: "store", Declaration: "rows", Kind: "var", Uses: 5, State: EdgeUnchecked},
		},
		Findings: map[rule.Rule]Count{
			rule.Boundary: {Keyable: true},
			rule.Qualify:  {Keyable: true},
		},
	}
}

// fixtureQuietPackage is a package where every file joined the core: nothing
// crosses, and no name is asked to carry a namespace that has none.
//
//declscope:package // the goldens are rendered from it
func fixtureQuietPackage() Package {
	return Package{
		Path:       "example.com/x/internal/only",
		AllCore:    true,
		Namespaces: []Namespace{{Name: "(core)", Files: []string{"a.go", "b.go"}, Core: true, Declarations: 9}},
		Findings: map[rule.Rule]Count{
			rule.Boundary: {Asked: true, Keyable: true},
			rule.Qualify:  {Keyable: true},
		},
	}
}

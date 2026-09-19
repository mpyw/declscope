package measure

import "github.com/mpyw/declscope/internal/rule"

// fixturePackage is a package shaped to exercise every state a renderer has a
// column, a note or a dash for: a core namespace the naming rule asks nothing
// of, a mutual pair, a crossing in each state, and a namespace whose names all
// fail.
//
// It is built by hand rather than analyzed, which is the point of the model
// being its own package: a renderer can be tested without building a pass.
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
			{From: "flags", To: "completion", Declaration: "compName", Kind: "func", Uses: 1, State: EdgeDeclared},
			{From: "(core)", To: "flags", Declaration: "flagSet", Kind: "var", Uses: 4, State: EdgeIgnored},
			{From: "completion", To: "(core)", Declaration: "Run", Kind: "func", Uses: 9, State: EdgeOpen},
		},
		Names: []NameFinding{
			{Namespace: "flags", File: "flags.go", Declaration: "parseAll", Kind: "func", State: NameReported, Fixable: true},
			{Namespace: "flags", File: "flags.go", Declaration: "Reset", Kind: "func", Exported: true, State: NameReported},
			{Namespace: "completion", File: "comp_zsh.go", Declaration: "zshHeader", Kind: "func", State: NameBaselined},
		},
		Findings: map[rule.Rule]Count{
			rule.Boundary:  {Asked: true, Keyable: true, Found: 4, Ignored: 1, Baselined: 1, Reported: 2},
			rule.Qualify:   {Asked: true, Keyable: true, Found: 3, Baselined: 1, Reported: 2},
			rule.Surplus:   {Asked: true, Keyable: true},
			rule.Directive: {Asked: true, Found: 1, Reported: 1},
			rule.Filter:    {Asked: true},
		},
	}
}

func fixtureSummary() Summary {
	return SummaryOf([]Package{fixturePackage(), {
		Path:       "example.com/x/internal/legacy",
		AllCore:    true,
		Namespaces: []Namespace{{Name: "(core)", Files: []string{"a.go", "b.go"}, Core: true, Declarations: 7}},
		Findings: map[rule.Rule]Count{
			rule.Boundary: {Asked: true, Keyable: true},
			rule.Qualify:  {Keyable: true},
		},
	}}, Checks{
		Configs: []ConfigUse{{
			Chain: []string{".declscope.yaml"}, Packages: 2,
			Boundary: true, Surplus: true, Qualify: "ondemand", Exported: true,
		}},
		Baselines: []BaselineUse{{Path: ".declscope-baseline.yaml", Entries: 12}},
		TypeCheck: TypeCheck{Packages: 2},
	})
}

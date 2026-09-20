package measure

import (
	"encoding/json"
	"io"

	"github.com/mpyw/declscope/internal/rule"
)

// The JSON is the interface the adoption skill reads, so the shapes are
// declared here rather than tagged onto the model: a field renamed for a
// terminal column would otherwise be a silent break for every consumer. Keys
// are added, never removed or renamed.
//
// Edges and names stay flat, one row each. Every table either command prints
// is a fold of those two arrays, and a consumer that wants a different fold —
// which is what an agent reading this usually wants — should not have to
// unpick somebody else's grouping first.

type jsonPackage struct {
	Package string   `json:"package"`
	Config  []string `json:"config,omitempty"`
	AllCore bool     `json:"allCore"`

	// Findings carries the same asked flag the tables print a dash for.
	// Without it a consumer reads qualifyTargets as a live number where the
	// text form says the rule was never in force — a zero from a switched-off
	// rule reading exactly like a zero from clean code, in the format the
	// adoption skill tells an agent to use.
	Findings map[string]jsonCount `json:"findings"`

	Namespaces []jsonNamespace `json:"namespaces"`
	Edges      []jsonEdge      `json:"edges"`
	Names      []jsonName      `json:"names"`
}

type jsonNamespace struct {
	Namespace      string   `json:"namespace"`
	Files          []string `json:"files"`
	Core           bool     `json:"core"`
	Declarations   int      `json:"declarations"`
	QualifyTargets int      `json:"qualifyTargets"`
}

type jsonEdge struct {
	From        string `json:"from"`
	To          string `json:"to"`
	Declaration string `json:"declaration"`
	Kind        string `json:"kind"`
	Uses        int    `json:"uses"`
	State       string `json:"state"`
}

type jsonName struct {
	Namespace   string `json:"namespace"`
	File        string `json:"file"`
	Declaration string `json:"declaration"`
	Kind        string `json:"kind"`
	Exported    bool   `json:"exported"`
	State       string `json:"state"`
	Fixable     bool   `json:"fixable"`
}

// writeJSON renders one package.
//
//declscope:package // format.go dispatches to it
func (p Package) writeJSON(w io.Writer) error {
	out := jsonPackage{
		Package:  p.Path,
		Config:   p.Config,
		AllCore:  p.AllCore,
		Findings: jsonCounts(p.Findings),
		// Never nil: an empty array says "none", where null says "this
		// command does not report that", and a consumer has to branch.
		Namespaces: make([]jsonNamespace, 0, len(p.Namespaces)),
		Edges:      make([]jsonEdge, 0, len(p.Edges)),
		Names:      make([]jsonName, 0, len(p.Names)),
	}
	for _, ns := range p.Namespaces {
		out.Namespaces = append(out.Namespaces, jsonNamespace{
			Namespace:      ns.Name,
			Files:          ns.Files,
			Core:           ns.Core,
			Declarations:   ns.Declarations,
			QualifyTargets: ns.QualifyTargets,
		})
	}
	for _, e := range p.Edges {
		out.Edges = append(out.Edges, jsonEdge{
			From:        e.From,
			To:          e.To,
			Declaration: e.Declaration,
			Kind:        e.Kind,
			Uses:        e.Uses,
			State:       string(e.State),
		})
	}
	for _, n := range p.Names {
		out.Names = append(out.Names, jsonName{
			Namespace:   n.Namespace,
			File:        n.File,
			Declaration: n.Declaration,
			Kind:        n.Kind,
			Exported:    n.Exported,
			State:       string(n.State),
			Fixable:     n.Fixable,
		})
	}
	return writeJSONValue(w, out)
}

type jsonSummary struct {
	Checks   jsonChecks           `json:"checks"`
	Totals   map[string]jsonCount `json:"totals"`
	Packages []jsonSummaryPackage `json:"packages"`
}

type jsonChecks struct {
	TypeCheck jsonTypeCheck  `json:"typeCheck"`
	Configs   []jsonConfig   `json:"configs"`
	Baselines []jsonBaseline `json:"baselines"`
}

type jsonTypeCheck struct {
	Packages int      `json:"packages"`
	Failed   []string `json:"failed"`
}

type jsonConfig struct {
	Chain    []string  `json:"chain"`
	Packages int       `json:"packages"`
	Rules    jsonRules `json:"rules"`
}

type jsonRules struct {
	Boundary bool   `json:"boundary"`
	Qualify  string `json:"qualify"`
	Exported bool   `json:"exported"`
	Surplus  bool   `json:"surplus"`
}

type jsonBaseline struct {
	Path    string `json:"path"`
	Entries int    `json:"entries"`
}

type jsonCount struct {
	// Asked says whether the rule was in force anywhere. The counts below are
	// zero either way when it is false, and the reason is not the code.
	Asked     bool `json:"asked"`
	Found     int  `json:"found"`
	Ignored   int  `json:"ignored"`
	Baselined int  `json:"baselined"`
	Reported  int  `json:"reported"`
}

type jsonSummaryPackage struct {
	Package    string            `json:"package"`
	Namespaces int               `json:"namespaces"`
	CoreFiles  int               `json:"coreFiles"`
	AllCore    bool              `json:"allCore"`
	Boundary   jsonBoundaryState `json:"boundary"`
	Qualify    jsonQualifyState  `json:"qualify"`
	Largest    *jsonLargest      `json:"largest,omitempty"`
}

type jsonBoundaryState struct {
	Reported  int `json:"reported"`
	Baselined int `json:"baselined"`
	Declared  int `json:"declared"`
}

type jsonQualifyState struct {
	Asked     bool       `json:"asked"`
	Reported  int        `json:"reported"`
	Baselined int        `json:"baselined"`
	Exempt    int        `json:"exempt"`
	Worst     *jsonWorst `json:"worst,omitempty"`
}

// jsonWorst names a namespace inside an object that names its package, which
// is the only way a namespace may be reported: two packages can spell one the
// same way and mean nothing in common.
type jsonWorst struct {
	Namespace string `json:"namespace"`
	Failing   int    `json:"failing"`
	Targets   int    `json:"targets"`
}

type jsonLargest struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Reached int    `json:"reached"`
	Uses    int    `json:"uses"`
}

// writeJSON renders a whole run.
//
//declscope:package // format.go dispatches to it
func (s Summary) writeJSON(w io.Writer) error {
	out := jsonSummary{
		Checks: jsonChecks{
			TypeCheck: jsonTypeCheck{
				Packages: s.Checks.TypeCheck.Packages,
				Failed:   jsonStrings(s.Checks.TypeCheck.Failed),
			},
			Configs:   make([]jsonConfig, 0, len(s.Checks.Configs)),
			Baselines: make([]jsonBaseline, 0, len(s.Checks.Baselines)),
		},
		Packages: make([]jsonSummaryPackage, 0, len(s.Rows)),
	}
	for _, c := range s.Checks.Configs {
		out.Checks.Configs = append(out.Checks.Configs, jsonConfig{
			Chain:    jsonStrings(c.Chain),
			Packages: c.Packages,
			Rules: jsonRules{
				Boundary: c.Boundary,
				Qualify:  c.Qualify,
				Exported: c.Exported,
				Surplus:  c.Surplus,
			},
		})
	}
	for _, b := range s.Checks.Baselines {
		// The conversion holds only while the two shapes agree: give the model
		// a field the JSON does not carry, and this stops compiling, which is
		// where the explicit mapping goes back in.
		out.Checks.Baselines = append(out.Checks.Baselines, jsonBaseline(b))
	}
	out.Totals = jsonCounts(s.Totals)
	for _, row := range s.Rows {
		pkg := jsonSummaryPackage{
			Package:    row.Package,
			Namespaces: row.Namespaces,
			CoreFiles:  row.CoreFiles,
			AllCore:    row.AllCore,
			Boundary: jsonBoundaryState{
				Reported:  row.BoundaryReported,
				Baselined: row.BoundaryBaselined,
				Declared:  row.BoundaryDeclared,
			},
			Qualify: jsonQualifyState{
				Asked:     row.QualifyAsked,
				Reported:  row.QualifyReported,
				Baselined: row.QualifyBaselined,
				Exempt:    row.QualifyExempt,
			},
		}
		if row.HasLargest {
			pkg.Largest = &jsonLargest{
				From:    row.Largest.From,
				To:      row.Largest.To,
				Reached: row.Largest.Reached,
				Uses:    row.Largest.Uses,
			}
		}
		if row.HasWorst {
			pkg.Qualify.Worst = &jsonWorst{
				Namespace: row.Worst.Namespace,
				Failing:   row.Worst.Saturation(),
				Targets:   row.Worst.Targets,
			}
		}
		out.Packages = append(out.Packages, pkg)
	}
	return writeJSONValue(w, out)
}

// jsonCounts renders one tally per rule, in rule.All's order, leaving out a
// rule the model holds nothing for.
func jsonCounts(counts map[rule.Rule]Count) map[string]jsonCount {
	out := make(map[string]jsonCount, len(counts))
	for _, r := range rule.All {
		count, ok := counts[r]
		if !ok {
			continue
		}
		out[string(r)] = jsonCount{
			Asked:     count.Asked,
			Found:     count.Found,
			Ignored:   count.Ignored,
			Baselined: count.Baselined,
			Reported:  count.Reported,
		}
	}
	return out
}

func writeJSONValue(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// jsonStrings turns a nil slice into an empty one, so that a consumer reads
// "nothing" rather than having to tell null from [].
func jsonStrings(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

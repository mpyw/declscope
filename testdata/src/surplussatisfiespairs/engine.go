package surplussatisfiespairs

// engine is embedded by pointer in use.go, which spells it and keeps its
// directive. spin is promoted from there into a contract.
//
//declscope:package
type engine struct{}

//declscope:package
func (*engine) spin() {}

// idle is in no contract, so its directive is reported. It shows the rule is
// on in this package, and that the pairs kept are no wider than they need be.
//
//declscope:package // want `//declscope:package on engine.idle: no use from another namespace is visible to declscope`
func (*engine) idle() {}

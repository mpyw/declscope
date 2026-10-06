//declscope:ignore unused

package fixrenamed

// ReportName's directive restates its scope, but the file silences the
// deletion, so the rename is what is reported and fixed.
//
//declscope:package // want `//declscope:package is renamed //declscope:shared; it still means shared`
func ReportName() string { return "" }

// TODO(#185): delete this fixture with the //declscope:package alias.

package pub

import "example.com/more/internal/r"

// Iface hands out an r.Out from its method.
type Iface interface{ Get() r.Out }

// Var is an exported variable of an internal type's pointer.
var Var *r.Out

// K hands out an r.Kept.
var K r.Kept

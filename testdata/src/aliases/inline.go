package aliases

// An alias whose right-hand side is written inline does contain its fields,
// for the same reason any other declaration does.
//declscope:private
type entry = struct { // want `type entry is declared private by //declscope:private, but is used from namespace "use"`
	key string // want `field entry.key is declared private by //declscope:private on entry, but is used from namespace "use"`
}

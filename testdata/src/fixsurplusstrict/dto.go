package fixsurplusstrict

// The directive decides only for the unexported fields: UserDTO and Name are
// shared by exportedness. Narrowing both fields would leave it binding
// nothing, so the fields are reported without a fix, and this file takes no
// edit. The directive is the thing to delete, which no fix does.
//
//declscope:shared
type UserDTO struct {
	Name string
	seq  int // want `field UserDTO.seq takes shared scope`
	rev  int // want `field UserDTO.rev takes shared scope`
}

func dtoLocal(d UserDTO) int { return d.seq + d.rev }

var _ = dtoLocal

#include "textflag.h"

// func Fast() int64
TEXT ·Fast(SB), NOSPLIT, $0-8
	MOVQ example·com∕asmuses∕internal∕cpu·HasFast(SB), AX
	MOVQ AX, ret+0(FP)
	RET

#include "textflag.h"

// func Fast() int64
TEXT ·Fast(SB), NOSPLIT, $0-8
	MOVD example·com∕asmuses∕internal∕cpu·HasFast(SB), R0
	MOVD R0, ret+0(FP)
	RET

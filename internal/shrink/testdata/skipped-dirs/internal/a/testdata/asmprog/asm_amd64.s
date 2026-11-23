#include "textflag.h"

// func read() int64
TEXT ·read(SB), NOSPLIT, $0-8
	MOVQ example·com∕skipped∕internal∕a·FromAssembly(SB), AX
	MOVQ AX, ret+0(FP)
	RET

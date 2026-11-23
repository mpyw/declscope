#include "textflag.h"

// func read() int64
TEXT ·read(SB), NOSPLIT, $0-8
	MOVD example·com∕skipped∕internal∕a·FromAssembly(SB), R0
	MOVD R0, ret+0(FP)
	RET

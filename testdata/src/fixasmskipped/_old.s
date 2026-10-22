#include "textflag.h"

TEXT ·quadruple(SB), NOSPLIT, $0-16
	CALL ·plain(SB)
	RET

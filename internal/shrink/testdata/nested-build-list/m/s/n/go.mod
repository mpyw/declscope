module example.com/nbl/s/n

go 1.24

require (
	example.com/nbl v0.0.0
	example.com/nbl/s/t v0.0.0
)

replace (
	example.com/nbl => ../../
	example.com/nbl/s/t => ../../../t-copy
)

module example.com/nbl/n

go 1.24

require (
	example.com/nbl v0.0.0
	example.com/nbl/t v0.0.0
)

replace (
	example.com/nbl => ../
	example.com/nbl/t => ../../t-copy
)

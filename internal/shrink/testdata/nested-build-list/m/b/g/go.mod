module example.com/nbl/b/g

go 1.24

require (
	example.com/nbl v0.0.0
	example.com/nbl/b/u v0.0.0
)

replace (
	example.com/nbl => ../../
	example.com/nbl/b/u => ../../../u-copy
)

module example.com/nbl/k

go 1.24

require (
	example.com/nbl v0.0.0
	example.com/nbl/c/v v0.0.0
)

replace (
	example.com/nbl => ../
	example.com/nbl/c/v => ../../v-copy
)

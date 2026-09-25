module github.com/mpyw/declscope

go 1.27.0

require (
	github.com/mpyw/go-skill-embed v0.2.1
	golang.org/x/mod v0.41.0
	golang.org/x/tools v0.50.0
	gopkg.in/yaml.v3 v3.0.1
)

require golang.org/x/sync v0.23.0 // indirect

retract v0.12.0 // go install fails: a testdata file name breaks the module zip

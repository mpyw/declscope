// A nested module with an unrelated path cannot import internal/a, so it
// keeps nothing unjudged.
module example.com/unrelated

go 1.25

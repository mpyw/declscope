// The test reads config.go's private sections map, and config_test.go is
// taken by the external tests, so it joins config.go's namespace by name.
//
//declscope:namespace config

package config

import (
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestSectionsNameEveryStruct checks that every struct the config decodes into
// has an entry in sections. go-yaml names the Go type when a key is unknown,
// and a section missing from the map leaves that raw complaint in place of
// the keys the section takes. filter was missing once, so a stale
// filter.exclude was answered with "type config.filterSection".
func TestSectionsNameEveryStruct(t *testing.T) {
	unmarshaler := reflect.TypeFor[yaml.Unmarshaler]()
	var walk func(reflect.Type)
	walk = func(typ reflect.Type) {
		switch typ.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Map:
			walk(typ.Elem())
			return
		case reflect.Struct:
		default:
			return
		}
		// A type that decodes itself never reaches go-yaml's field lookup.
		if reflect.PointerTo(typ).Implements(unmarshaler) {
			return
		}
		if _, ok := sections[typ.String()]; !ok {
			t.Errorf("sections has no entry for %s", typ)
		}
		for i := range typ.NumField() {
			if typ.Field(i).IsExported() {
				walk(typ.Field(i).Type)
			}
		}
	}
	walk(reflect.TypeFor[File]())
}

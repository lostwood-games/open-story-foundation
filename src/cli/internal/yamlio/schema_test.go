package yamlio

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

// object is the part of a JSON Schema object definition the loader mirrors.
type object struct {
	Required   []string           `json:"required"`
	Properties map[string]*object `json:"properties"`
	Additional json.RawMessage    `json:"additionalProperties"` // false or a schema
}

// checkerRequired are the texts the schema requires but the loader leaves
// to the checker (OSF110), so YAML and .osf report them alike.
var checkerRequired = map[string][]string{
	"Story":         {"name"},
	"Character":     {"name"},
	"Document":      {"name"},
	"Content":       {"text"},
	"OptionContent": {"text"},
}

// TestShapesMatchSchema keeps the loader's field table equal to
// schema/osf-v1.schema.json.
func TestShapesMatchSchema(t *testing.T) {
	src, err := os.ReadFile("../../../../schema/osf-v1.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		object
		Defs map[string]*object `json:"$defs"`
	}
	if err := json.Unmarshal(src, &schema); err != nil {
		t.Fatal(err)
	}
	defs := map[string]*object{"resource": &schema.object}
	for name, d := range schema.Defs {
		defs[name] = d
	}
	if o := defs["Option"]; o != nil {
		defs["OptionContent"] = o.Properties["content"]
	}
	if c := defs["I18nConfiguration"]; c != nil && c.Properties["locales"] != nil {
		var l object
		if err := json.Unmarshal(c.Properties["locales"].Additional, &l); err == nil {
			defs["I18nLocale"] = &l
		}
	}
	for name, sh := range shapes {
		d := defs[name]
		if d == nil {
			t.Errorf("%s: not in the schema", name)
			continue
		}
		var props []string
		for p := range d.Properties {
			props = append(props, p)
		}
		if !sameSet(props, sh.allowed) {
			t.Errorf("%s: schema properties %v, loader allows %v", name, sorted(props), sorted(sh.allowed))
		}
		want := append(slices.Clone(sh.required), checkerRequired[name]...)
		if !sameSet(d.Required, want) {
			t.Errorf("%s: schema requires %v, loader and checker require %v", name, sorted(d.Required), sorted(want))
		}
	}
}

func sorted(s []string) []string { s = slices.Clone(s); slices.Sort(s); return s }

func sameSet(a, b []string) bool { return slices.Equal(sorted(a), sorted(b)) }

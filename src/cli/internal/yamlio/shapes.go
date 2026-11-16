package yamlio

// shape is the keys a YAML mapping may have and the ones it must have.
type shape struct{ allowed, required []string }

// shapes mirrors the objects of schema/osf-v1.schema.json, keyed by their
// $defs name; schema_test.go keeps the two in step. Texts the checker
// requires (a Story name, a node text) are left out of required so that YAML
// and .osf projects report them with the same rule, OSF110.
var shapes = map[string]shape{
	"resource":             {[]string{"apiVersion", "kind", "data"}, []string{"apiVersion", "kind", "data"}},
	"Story":                {[]string{"id", "name", "description", "start"}, []string{"id"}},
	"Character":            {[]string{"id", "name", "description"}, []string{"id"}},
	"Document":             {[]string{"id", "name"}, []string{"id"}},
	"Variable":             {[]string{"id", "type", "default", "description"}, []string{"id", "type", "default"}},
	"Dialogue":             {[]string{"id", "name", "startNode", "nodes"}, []string{"id", "startNode", "nodes"}},
	"Node":                 {[]string{"content", "set", "next", "options"}, []string{"content"}},
	"Content":              {[]string{"speaker", "title", "text"}, nil},
	"Option":               {[]string{"content", "requires", "set", "next"}, []string{"content"}},
	"Branch":               {[]string{"to", "requires"}, []string{"to"}},
	"OptionContent":        {[]string{"text"}, nil},
	"SetOperation":         {[]string{"operation", "value"}, []string{"operation", "value"}},
	"Condition":            {[]string{"operator", "value"}, []string{"operator", "value"}},
	"Locale":               {[]string{"language", "values"}, []string{"language", "values"}},
	"I18nConfiguration":    {[]string{"source", "locales"}, []string{"locales"}},
	"I18nLocale":           {[]string{"id", "enforce"}, []string{"id"}},
	"PluginsConfiguration": {[]string{"plugins"}, []string{"plugins"}},
}

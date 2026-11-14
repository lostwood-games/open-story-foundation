package lang

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/lostwood-games/open-story-foundation/src/cli/internal/diag"
	"github.com/lostwood-games/open-story-foundation/src/cli/internal/osf"
	"github.com/lostwood-games/open-story-foundation/src/cli/internal/source"
)

// builtIn plugins need no plugins/v1 Configuration entry.
var builtIn = map[string]bool{"story": true, "dialogue": true}

// LoadDir parses every .osf file under dir into one project. The project is
// returned even when there are diagnostics, holding everything that parsed,
// so the checker can still run; it is nil only when there are no files.
func LoadDir(dir string) (*osf.Project, []diag.Diagnostic) {
	files, err := source.Files(dir, ".osf")
	if err != nil {
		return nil, []diag.Diagnostic{diag.ReadError.At(diag.Pos{}, diag.Pos{}, "%v", err)}
	}
	if len(files) == 0 {
		return nil, []diag.Diagnostic{diag.NoProject.At(diag.Pos{File: dir}, diag.Pos{}, "no .osf files found")}
	}
	u := NewUnit()
	var diags []diag.Diagnostic
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			diags = append(diags, diag.ReadError.At(diag.Pos{File: path}, diag.Pos{}, "%v", err))
			continue
		}
		rel, _ := filepath.Rel(dir, path)
		diags = append(diags, ParseFile(u, path, filepath.ToSlash(rel), src)...)
	}
	diags = append(diags, ResolveVersions(u)...)
	return u.Project, diags
}

// ResolveVersions applies the project's Versions block (language.md §3.1): it
// sets the apiVersion of every resource and the plugin list.
func ResolveVersions(u *Unit) []diag.Diagnostic {
	var diags []diag.Diagnostic
	errf := func(r diag.Rule, pos, end diag.Pos, format string, args ...any) {
		diags = append(diags, r.At(pos, end, format, args...))
	}
	p := u.Project
	p.APIVersions = map[string]string{}
	if len(u.Versions) == 0 {
		errf(diag.NoVersions, diag.Pos{}, diag.Pos{}, `no Versions block found; add versions.osf with Versions { language = "1"  story = "v1"  dialogue = "v1" }`)
		return diags
	}
	for _, extra := range u.Versions[1:] {
		errf(diag.DuplicateVersions, extra.Pos, diag.Pos{}, "a project has one Versions block; the first is at %s", u.Versions[0].Pos)
	}
	v := u.Versions[0]
	p.PluginsPos = v.Pos
	p.PluginsComments = v.Comments
	p.PluginComments = map[string]osf.Comments{}
	for _, e := range v.Entries {
		versions, known := osf.Supported[e.Name]
		switch {
		case !known:
			errf(diag.UnknownPlugin, e.Value.Pos, e.Value.End, "unknown plugin %q; this version of osf knows language, story, dialogue and i18n", e.Name)
		case !contains(versions, e.Value.Value):
			errf(diag.UnsupportedVersion, e.Value.Pos, e.Value.End, "%s version %q is not supported by this version of osf (supported: %s)", e.Name, e.Value.Value, strings.Join(versions, ", "))
		case e.Name == "language":
		default:
			p.APIVersions[e.Name] = e.Value.Value
			if !builtIn[e.Name] {
				p.Plugins = append(p.Plugins, e.Name+"/"+e.Value.Value)
				p.PluginComments[e.Name+"/"+e.Value.Value] = e.Value.Comments
			}
		}
	}
	for _, name := range []string{"language", "story", "dialogue"} {
		if !hasEntry(v, name) {
			errf(diag.MissingVersion, v.Pos, diag.Pos{}, "Versions must set %s", name)
		}
	}
	p.PluginsSet = len(p.Plugins) > 0
	if !hasEntry(v, "i18n") {
		var first *diag.Pos
		if len(p.I18n) > 0 {
			first = &p.I18n[0].Pos
		} else if len(p.Locales) > 0 {
			first = &p.Locales[0].Pos
		}
		if first != nil {
			errf(diag.PluginNotEnabled, *first, diag.Pos{}, `I18n and Locale blocks need the i18n plugin: add i18n = "v1" to Versions`)
		}
	}
	return diags
}

func hasEntry(v *Versions, name string) bool {
	for _, e := range v.Entries {
		if e.Name == name {
			return true
		}
	}
	return false
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

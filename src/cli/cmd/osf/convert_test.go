package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/lostwood-games/open-story-foundation/src/cli/internal/check"
	"github.com/lostwood-games/open-story-foundation/src/cli/internal/lang"
	"github.com/lostwood-games/open-story-foundation/src/cli/internal/source"
	"github.com/lostwood-games/open-story-foundation/src/cli/internal/yamlio"
)

// resources indexes YAML resources by Kind.id, merging each language's Locale
// values, so two projects can be compared regardless of file layout.
func resources(t *testing.T, files map[string][]byte) map[string]any {
	out := map[string]any{}
	for name, src := range files {
		var r map[string]any
		if err := yaml.Unmarshal(src, &r); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		data := r["data"].(map[string]any)
		switch r["kind"] {
		case "Locale":
			key := fmt.Sprintf("Locale.%v", data["language"])
			if out[key] == nil {
				out[key] = map[string]any{}
			}
			for k, v := range data["values"].(map[string]any) {
				out[key].(map[string]any)[k] = v
			}
		case "Configuration":
			out[fmt.Sprint(r["apiVersion"])] = data
		default:
			out[fmt.Sprintf("%v.%v", r["kind"], data["id"])] = r
		}
	}
	return out
}

func TestConvertMatchesYAMLExample(t *testing.T) {
	p, diags := lang.LoadDir(osfExample)
	if len(diags) > 0 {
		t.Fatalf("load: %v", diags)
	}
	if r := check.Project(p); len(r.Errors) > 0 {
		t.Fatalf("check: %v", r.Errors)
	}
	built, err := yamlio.Render(p)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]byte{}
	for _, f := range built {
		got[f.Path] = f.Content
	}
	want := map[string][]byte{}
	paths, _ := source.Files(yamlExample, ".yaml")
	for _, path := range paths {
		want[path], _ = os.ReadFile(path)
	}
	g, w := resources(t, got), resources(t, want)
	for k := range w {
		if !reflect.DeepEqual(g[k], w[k]) {
			gy, _ := yaml.Marshal(g[k])
			wy, _ := yaml.Marshal(w[k])
			t.Errorf("%s differs\n--- built\n%s--- yaml/example\n%s", k, gy, wy)
		}
	}
	for k := range g {
		if _, ok := w[k]; !ok {
			t.Errorf("built %s, which yaml/example does not have", k)
		}
	}
}

func TestConvertWritesAndReplaces(t *testing.T) {
	dir := osfProject(t, map[string]string{"s.osf": baseStory + `Dialogue d { Node a { text: "x" } }`})
	out := filepath.Join(dir, "out")
	if code := run([]string{"convert", "-out", out, dir}, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("convert exit %d", code)
	}
	if _, err := os.Stat(filepath.Join(out, "d.json")); err != nil {
		t.Fatal(err)
	}
	// Rename the dialogue: the old file must go, other files must stay.
	os.WriteFile(filepath.Join(dir, "s.osf"), []byte(strings.Replace(baseStory, "Dialogue.d", "Dialogue.e", 1)+`Dialogue e { Node a { text: "x" } }`), 0o644)
	os.WriteFile(filepath.Join(out, "mine.txt"), []byte("keep"), 0o644)
	if code := run([]string{"convert", dir, "-out", out}, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("reconvert exit %d", code)
	}
	if _, err := os.Stat(filepath.Join(out, "d.json")); !os.IsNotExist(err) {
		t.Error("stale d.json was not removed")
	}
	for _, f := range []string{"e.json", "mine.txt"} {
		if _, err := os.Stat(filepath.Join(out, f)); err != nil {
			t.Error(err)
		}
	}
	// A folder osf convert did not make is never written to.
	other := filepath.Join(dir, "other")
	os.MkdirAll(other, 0o755)
	os.WriteFile(filepath.Join(other, "x.txt"), nil, 0o644)
	var stderr bytes.Buffer
	if code := run([]string{"convert", "-out", other, dir}, &bytes.Buffer{}, &stderr); code != 1 || !strings.Contains(stderr.String(), "was not made by osf;") {
		t.Errorf("exit %d: %s", code, stderr.String())
	}
}

func TestDocuments(t *testing.T) {
	files := map[string]string{}
	files["s.osf"] = baseStory + `I18n { source = Eng Locale Rus "Russian" { enforce = true } }
Dialogue d { Node a { text: "At {{ Document.lighthouse }}" } }`
	files["documents/documents.osf"] = `Document lighthouse { name = "The Lighthouse" }`
	files["locales/Rus/s.osf"] = `Locale Rus { Story s { name = "С" } Character hero { name = "Герой" } Dialogue d { Node a { text: "У {{ Document.lighthouse }}" } } }`
	files["locales/Rus/documents/documents.osf"] = `Locale Rus { Document lighthouse { name = "Маяк" } }`
	dir := osfProject(t, files)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"validate", dir}, &stdout, &stderr); code != 0 {
		t.Fatalf("validate exit %d:\n%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "1 document") {
		t.Errorf("summary should count the document:\n%s", stdout.String())
	}

	out := filepath.Join(t.TempDir(), "build")
	if code := run([]string{"convert", "-o", "yaml", "-out", out, dir}, &bytes.Buffer{}, &stderr); code != 0 {
		t.Fatalf("convert exit %d:\n%s", code, stderr.String())
	}
	built, err := os.ReadFile(filepath.Join(out, "documents", "lighthouse.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"apiVersion: story/v1", "kind: Document", "id: lighthouse", "name: The Lighthouse"} {
		if !strings.Contains(string(built), want) {
			t.Errorf("documents/lighthouse.yaml has no %q:\n%s", want, built)
		}
	}
	if code := run([]string{"validate", out}, &bytes.Buffer{}, &stderr); code != 0 {
		t.Fatalf("validate of the YAML convert exit %d:\n%s", code, stderr.String())
	}
}

func convert(t *testing.T, format, out, dir string) {
	t.Helper()
	var stderr bytes.Buffer
	if code := run([]string{"convert", "-o", format, "-out", out, dir}, &bytes.Buffer{}, &stderr); code != 0 {
		t.Fatalf("convert -o %s %s: exit %d:\n%s", format, dir, code, stderr.String())
	}
}

func readFiles(t *testing.T, dir, ext string) map[string]string {
	t.Helper()
	paths, err := source.Files(dir, ext)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, p := range paths {
		rel, _ := filepath.Rel(dir, p)
		src, _ := os.ReadFile(p)
		out[filepath.ToSlash(rel)] = string(src)
	}
	return out
}

func sameFiles(t *testing.T, what string, got, want map[string]string) {
	t.Helper()
	for rel, w := range want {
		g, ok := got[rel]
		if !ok {
			t.Errorf("%s: %s is missing", what, rel)
		} else if g != w {
			t.Errorf("%s: %s differs\n--- got\n%s--- want\n%s", what, rel, g, w)
		}
	}
	for rel := range got {
		if _, ok := want[rel]; !ok {
			t.Errorf("%s: unexpected file %s", what, rel)
		}
	}
}

// TestRoundTripThroughYAML: .osf -> YAML -> .osf gives the source back, file
// for file and byte for byte, comments included.
func TestRoundTripThroughYAML(t *testing.T) {
	tmp := t.TempDir()
	yamlDir, osfDir := filepath.Join(tmp, "yaml"), filepath.Join(tmp, "osf")
	convert(t, "yaml", yamlDir, osfExample)
	convert(t, "osf", osfDir, yamlDir)
	sameFiles(t, "osf -> yaml -> osf", readFiles(t, osfDir, ".osf"), readFiles(t, osfExample, ".osf"))
}

// TestRoundTripThroughJSON: JSON has no comments and does not say which .osf
// file a resource came from, so the .osf written from it has the standard
// layout. Converting that back gives the same JSON, and it is valid.
func TestRoundTripThroughJSON(t *testing.T) {
	tmp := t.TempDir()
	json1, osfDir, json2 := filepath.Join(tmp, "json1"), filepath.Join(tmp, "osf"), filepath.Join(tmp, "json2")
	convert(t, "json", json1, osfExample)
	convert(t, "osf", osfDir, json1)
	var stderr bytes.Buffer
	if code := run([]string{"validate", osfDir}, &bytes.Buffer{}, &stderr); code != 0 {
		t.Fatalf("the .osf written from JSON does not validate:\n%s", stderr.String())
	}
	convert(t, "json", json2, osfDir)
	got, want := map[string]string{}, map[string]string{}
	for _, m := range []struct {
		dir string
		out map[string]string
	}{{json2, got}, {json1, want}} {
		raw := map[string][]byte{}
		for rel, src := range readFiles(t, m.dir, ".json") {
			raw[rel] = []byte(src)
		}
		for k, v := range resources(t, raw) {
			b, _ := yaml.Marshal(v)
			m.out[k] = string(b)
		}
	}
	sameFiles(t, "json -> osf -> json", got, want)
}

// TestConvertOSFToOSF: an .osf project converted to .osf keeps its files and
// content, since the example is already in the standard formatting.
func TestConvertOSFToOSF(t *testing.T) {
	out := filepath.Join(t.TempDir(), "osf")
	convert(t, "osf", out, osfExample)
	sameFiles(t, "osf -> osf", readFiles(t, out, ".osf"), readFiles(t, osfExample, ".osf"))
}

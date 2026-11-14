package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const yamlExample = "../../../../example/yaml"

func writeProject(t *testing.T, files map[string]string) string {
	dir := t.TempDir()
	for name, src := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestYAMLExampleIsValid(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"validate", yamlExample}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d:\n%s", code, stderr.String())
	}
}

func TestYAMLErrors(t *testing.T) {
	dir := writeProject(t, map[string]string{
		"a.yaml": "apiVersion: story/v1\nkind: Character\ndata:\n  id: a\n  name: A\n  name: B\n",
		"b.yaml": "apiVersion: story/v1\nkind: Character\ndata:\n  id: b\n  nickname: B\n",
	})
	var stderr bytes.Buffer
	if code := run([]string{"validate", dir}, &bytes.Buffer{}, &stderr); code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
	for _, want := range []string{`a.yaml:6:3: error: OSF014 Character: key "name" is repeated`, `OSF013 Character: unknown field "nickname"`, `b.yaml:1:1: error: OSF110 Character b has no name`} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("want %q in:\n%s", want, stderr.String())
		}
	}
}

func TestNoProject(t *testing.T) {
	var stderr bytes.Buffer
	if code := run([]string{"validate", t.TempDir()}, &bytes.Buffer{}, &stderr); code == 0 {
		t.Error("an empty folder passed validation")
	}
	if !strings.Contains(stderr.String(), "OSF090") {
		t.Errorf("want OSF090 in:\n%s", stderr.String())
	}
}

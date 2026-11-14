package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lostwood-games/open-story-foundation/src/cli/internal/diag"
	"github.com/lostwood-games/open-story-foundation/src/cli/internal/project"
	"github.com/lostwood-games/open-story-foundation/src/cli/internal/source"
	"github.com/lostwood-games/open-story-foundation/src/cli/internal/yamlio"
)

// manifest lists the files a build wrote, so the next build can replace them
// without touching anything else in the output folder.
const manifest = ".osf-build"

func runBuild(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("out", "", "output folder (default: build/ inside the project)")
	output := fs.String("o", "json", "json or yaml")
	fs.Usage = func() {
		fmt.Fprint(stderr, `Usage: osf build [-o json|yaml] [-out dir] [dir]

Validates the .osf project in dir (default: the current folder) and, if it has
no errors, writes one OSF resource file per Story, Character, Document, Variable,
Dialogue and translation file, mirroring the project's folders.

Options:
  -o json     Output format: json (default) or yaml. Both hold the same
              resources in the same layout; YAML also keeps the comments.
  -out dir    Output folder. Default: build/ inside the project. A folder that
              is not empty and was not made by osf build is never overwritten.
`)
	}
	if err := parseArgs(fs, args); err != nil {
		return flagExit(err)
	}
	dir, ok := oneDir(fs, stderr)
	if !ok {
		return 2
	}
	if *out == "" {
		*out = filepath.Join(dir, "build")
	}
	if *output != "json" && *output != "yaml" {
		fmt.Fprintf(stderr, "osf: -o must be json or yaml, not %q\n", *output)
		return 2
	}

	if files, _ := source.Files(dir, ".osf"); len(files) == 0 {
		fmt.Fprintf(stderr, "osf: %s holds no .osf files to build\n", dir)
		return 1
	}
	p, diags := project.Analyze(dir)
	printDiags(stderr, diags)
	if errors, warnings := diag.Count(diags); errors > 0 || p == nil {
		fmt.Fprintf(stderr, "\n%s, %s.\nNothing was built.\n", plural(errors, "error"), plural(warnings, "warning"))
		return 1
	}
	files, err := yamlio.RenderFormat(p, *output)
	if err != nil {
		fmt.Fprintf(stderr, "osf: %v\n", err)
		return 1
	}
	outFiles := make([]outFile, len(files))
	for i, f := range files {
		outFiles[i] = outFile{f.Path, f.Content}
	}
	if err := writeFiles(*out, manifest, "osf build", outFiles); err != nil {
		fmt.Fprintf(stderr, "osf: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Built %s into %s (%s).\n", plural(len(files), "resource"), *out, summary(p))
	return 0
}

// outFile is one file to write: a /-separated path relative to the output
// folder and its content.
type outFile struct {
	path    string
	content []byte
}

// writeFiles writes files into out, first removing what the previous run of
// tool (osf build or osf import) listed in its manifest there.
func writeFiles(out, manifestName, tool string, files []outFile) error {
	previous, err := readManifest(out, manifestName, tool)
	if err != nil {
		return err
	}
	for _, rel := range previous {
		if err := os.Remove(filepath.Join(out, filepath.FromSlash(rel))); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	var written []string
	for _, f := range files {
		path := filepath.Join(out, filepath.FromSlash(f.path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, f.content, 0o644); err != nil {
			return err
		}
		written = append(written, f.path)
	}
	text := fmt.Sprintf("# Files written by %s. The next %s replaces them.\n%s\n", tool, strings.TrimPrefix(tool, "osf "), strings.Join(written, "\n"))
	return os.WriteFile(filepath.Join(out, manifestName), []byte(text), 0o644)
}

// readManifest returns the files the last run of tool wrote to out. It
// refuses an existing, non-empty folder that tool did not make.
func readManifest(out, manifestName, tool string) ([]string, error) {
	entries, err := os.ReadDir(out)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	files, err := manifestFiles(filepath.Join(out, manifestName))
	if files == nil && err == nil && len(entries) > 0 {
		return nil, fmt.Errorf("%s is not empty and was not made by %s; choose another folder with -out", out, tool)
	}
	return files, err
}

// manifestFiles reads the file list in a manifest, nil when there is none.
func manifestFiles(path string) ([]string, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var files []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if filepath.IsAbs(line) || strings.HasPrefix(filepath.Clean(line), "..") {
			continue // never delete outside the output folder
		}
		files = append(files, line)
	}
	if files == nil {
		files = []string{}
	}
	return files, sc.Err()
}

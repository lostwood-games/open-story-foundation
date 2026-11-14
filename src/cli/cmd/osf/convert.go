package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lostwood-games/open-story-foundation/src/cli/internal/diag"
	"github.com/lostwood-games/open-story-foundation/src/cli/internal/osf"
	"github.com/lostwood-games/open-story-foundation/src/cli/internal/osfwrite"
	"github.com/lostwood-games/open-story-foundation/src/cli/internal/project"
	"github.com/lostwood-games/open-story-foundation/src/cli/internal/source"
	"github.com/lostwood-games/open-story-foundation/src/cli/internal/yamlio"
)

// manifest lists the files osf convert wrote, in source order, so the next
// run can replace them without touching anything else in the
// output folder, and a conversion back to .osf can restore that order.
const manifest = ".osf-files"

func runConvert(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("convert", flag.ContinueOnError)
	fs.SetOutput(stderr)
	output := fs.String("o", "json", "json, yaml or osf")
	out := fs.String("out", "", "output folder (default: <dir>-<format> next to the project)")
	fs.Usage = func() {
		fmt.Fprint(stderr, `Usage: osf convert [-o json|yaml|osf] [-out dir] [dir]

Validates the project in dir (default: the current folder) and, if it has no
errors, writes it in the form named by -o. dir is read as .osf source if it
holds any .osf file, otherwise as OSF YAML or JSON resources. Any form converts
to any form; converting to the form a project already has rewrites it in the
standard formatting.

  -o json   One JSON resource file per Story, Character, Document, Variable,
            Dialogue and translation file (default).
  -o yaml   The same resources as YAML, with the comments of the .osf source.
  -o osf    .osf source for writers. .osf input keeps its files. YAML files
            written by osf go back into the .osf file named in their first
            line. Anything else is laid out as versions.osf, story.osf,
            characters.osf, documents.osf, variables.osf, one file per
            dialogue, and locales/<Locale>/ for translations.

Options:
  -out dir  Output folder. Default: <dir>-<format> next to the project, such
            as story-json. A folder that is not empty and was not made by
            osf is never overwritten.
`)
	}
	if err := parseArgs(fs, args); err != nil {
		return flagExit(err)
	}
	dir, ok := oneDir(fs, stderr)
	if !ok {
		return 2
	}
	if *output != "json" && *output != "yaml" && *output != "osf" {
		fmt.Fprintf(stderr, "osf: -o must be json, yaml or osf, not %q\n", *output)
		return 2
	}
	if *out == "" {
		abs, err := filepath.Abs(dir)
		if err != nil {
			fmt.Fprintf(stderr, "osf: %v\n", err)
			return 1
		}
		*out = abs + "-" + *output
	}

	return convertProject(dir, *out, *output, stdout, stderr)
}

// convertProject validates the project in dir and writes it into out as
// format: json, yaml or osf.
func convertProject(dir, out, format string, stdout, stderr io.Writer) int {
	osfFiles, _ := source.Files(dir, ".osf")
	isOSF := len(osfFiles) > 0
	p, diags := project.Analyze(dir)
	printDiags(stderr, diags)
	if errors, warnings := diag.Count(diags); errors > 0 || p == nil {
		fmt.Fprintf(stderr, "\n%s, %s.\nNothing was converted.\n", plural(errors, "error"), plural(warnings, "warning"))
		return 1
	}

	var files []outFile
	if format == "osf" {
		written, err := osfSource(dir, p, isOSF)
		if err != nil {
			fmt.Fprintf(stderr, "osf: %v\n", err)
			return 1
		}
		for _, f := range written {
			files = append(files, outFile{f.Path, f.Content})
		}
	} else {
		if !isOSF {
			// Resources keep the order of the files they were read from.
			if err := restoreOrder(dir, p); err != nil {
				fmt.Fprintf(stderr, "osf: %v\n", err)
				return 1
			}
		}
		written, err := yamlio.RenderFormat(p, format)
		if err != nil {
			fmt.Fprintf(stderr, "osf: %v\n", err)
			return 1
		}
		for _, f := range written {
			files = append(files, outFile{f.Path, f.Content})
		}
	}
	if err := writeFiles(out, files); err != nil {
		fmt.Fprintf(stderr, "osf: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Wrote %s: %s in %s (%s).\n", format, plural(len(files), "file"), out, summary(p))
	return 0
}

// osfSource renders the project loaded from dir as .osf files. A project
// that was .osf already keeps its files. Otherwise the head comment of each
// YAML file restores which .osf file it came from.
func osfSource(dir string, p *osf.Project, isOSF bool) ([]osfwrite.File, error) {
	if isOSF {
		return osfwrite.Project(p, func(file string) string { return file })
	}
	paths, err := yamlio.FindFiles(dir)
	if err != nil {
		return nil, err
	}
	origins := map[string]string{}
	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		rel, _ := filepath.Rel(dir, path)
		origins[filepath.ToSlash(rel)] = yamlio.GeneratedFrom(src)
	}
	if err := restoreOrder(dir, p); err != nil {
		return nil, err
	}
	return osfwrite.Project(p, func(file string) string { return origins[file] })
}

// restoreOrder sorts the project's resources by the position of their file in
// the manifest an earlier run of osf left in dir, which lists files in
// source order. Files not listed keep their place after the listed ones.
func restoreOrder(dir string, p *osf.Project) error {
	built, err := manifestFiles(filepath.Join(dir, manifest))
	if err != nil || len(built) == 0 {
		return err
	}
	rank := map[string]int{}
	for i, f := range built {
		rank[f] = i
	}
	byFile := func(file string) int {
		if r, ok := rank[file]; ok {
			return r
		}
		return len(built)
	}
	sort.SliceStable(p.Characters, func(i, j int) bool { return byFile(p.Characters[i].File) < byFile(p.Characters[j].File) })
	sort.SliceStable(p.Documents, func(i, j int) bool { return byFile(p.Documents[i].File) < byFile(p.Documents[j].File) })
	sort.SliceStable(p.Variables, func(i, j int) bool { return byFile(p.Variables[i].File) < byFile(p.Variables[j].File) })
	sort.SliceStable(p.Dialogues, func(i, j int) bool { return byFile(p.Dialogues[i].File) < byFile(p.Dialogues[j].File) })
	sort.SliceStable(p.Locales, func(i, j int) bool { return byFile(p.Locales[i].File) < byFile(p.Locales[j].File) })
	return nil
}

// outFile is one file to write: a /-separated path relative to the output
// folder and its content.
type outFile struct {
	path    string
	content []byte
}

// writeFiles writes files into out, first removing what the previous run
// of osf listed in its manifest there.
func writeFiles(out string, files []outFile) error {
	previous, err := readManifest(out)
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
	text := fmt.Sprintf("# Files written by osf convert. The next convert into this folder replaces them.\n%s\n", strings.Join(written, "\n"))
	return os.WriteFile(filepath.Join(out, manifest), []byte(text), 0o644)
}

// readManifest returns the files the last run of osf wrote to out. It
// refuses an existing, non-empty folder that osf did not make.
func readManifest(out string) ([]string, error) {
	entries, err := os.ReadDir(out)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	files, err := manifestFiles(filepath.Join(out, manifest))
	if files == nil && err == nil && len(entries) > 0 {
		return nil, fmt.Errorf("%s is not empty and was not made by osf; choose another folder with -out", out)
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

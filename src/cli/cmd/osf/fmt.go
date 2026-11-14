package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/lostwood-games/open-story-foundation/src/cli/internal/format"
	"github.com/lostwood-games/open-story-foundation/src/cli/internal/source"
)

func runFmt(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("fmt", flag.ContinueOnError)
	fs.SetOutput(stderr)
	check := fs.Bool("check", false, "")
	diff := fs.Bool("diff", false, "")
	write := fs.Bool("write", true, "")
	list := fs.Bool("list", true, "")
	fs.Usage = func() {
		fmt.Fprint(stderr, `Usage: osf fmt [options] [path ...]

Rewrites .osf files in the standard formatting. Paths may be files or folders
(searched recursively); the default is the current folder.

Options:
  -check        Change nothing; exit with status 3 if any file is not formatted.
  -diff         Show the changes.
  -write=false  Do not rewrite files.
  -list=false   Do not print the names of files that changed.
`)
	}
	if err := parseArgs(fs, args); err != nil {
		return flagExit(err)
	}
	paths := fs.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}
	var files []string
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			fmt.Fprintf(stderr, "osf: %v\n", err)
			return 2
		}
		if !info.IsDir() {
			files = append(files, p)
			continue
		}
		found, err := source.Files(p, ".osf")
		if err != nil {
			fmt.Fprintf(stderr, "osf: %v\n", err)
			return 2
		}
		files = append(files, found...)
	}

	failed, changed := false, 0
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(stderr, "osf: %v\n", err)
			failed = true
			continue
		}
		out, err := format.Source(path, src)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			failed = true
			continue
		}
		if bytes.Equal(src, out) {
			continue
		}
		changed++
		if *list {
			fmt.Fprintln(stdout, path)
		}
		if *diff {
			fmt.Fprint(stdout, unifiedDiff(path, string(src), string(out)))
		}
		if *write && !*check {
			info, _ := os.Stat(path)
			if err := os.WriteFile(path, out, info.Mode().Perm()); err != nil {
				fmt.Fprintf(stderr, "osf: %v\n", err)
				failed = true
			}
		}
	}
	switch {
	case failed:
		return 2
	case *check && changed > 0:
		return 3
	}
	return 0
}

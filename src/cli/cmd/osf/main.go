package main

import (
	"fmt"
	"io"
	"os"
)

// version is set at release time with -ldflags "-X main.version=<x.y.z>".
var version = "0.1.0-dev"

type command struct {
	name    string
	summary string
	run     func(args []string, stdout, stderr io.Writer) int
}

var commands = []command{
	{"validate", "Check a project against the Open Story spec", runValidate},
	{"version", "Print the osf version", runVersion},
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	name := args[0]
	switch name {
	case "help", "-h", "-help", "--help":
		usage(stdout)
		return 0
	case "-v", "-version", "--version":
		name = "version"
	}
	for _, c := range commands {
		if c.name == name {
			return c.run(args[1:], stdout, stderr)
		}
	}
	fmt.Fprintf(stderr, "osf: unknown command %q\n\n", name)
	usage(stderr)
	return 2
}

func usage(w io.Writer) {
	fmt.Fprint(w, "Usage: osf <command> [options] [args]\n\nCommands:\n")
	for _, c := range commands {
		fmt.Fprintf(w, "  %-9s  %s\n", c.name, c.summary)
	}
	fmt.Fprint(w, "\nRun osf <command> -help for a command's options.\n")
}

func runVersion(args []string, stdout, stderr io.Writer) int {
	fmt.Fprintf(stdout, "osf v%s\n", version)
	fmt.Fprintf(stdout, "language 1, plugins: story/v1, dialogue/v1, i18n/v1\n")
	return 0
}

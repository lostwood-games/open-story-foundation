// Package project loads a folder as an Open Story project and checks it. The
// osf command and, later, the language server share it.
package project

import (
	"os"

	"github.com/lostwood-games/open-story-foundation/src/cli/internal/check"
	"github.com/lostwood-games/open-story-foundation/src/cli/internal/diag"
	"github.com/lostwood-games/open-story-foundation/src/cli/internal/lang"
	"github.com/lostwood-games/open-story-foundation/src/cli/internal/osf"
	"github.com/lostwood-games/open-story-foundation/src/cli/internal/source"
	"github.com/lostwood-games/open-story-foundation/src/cli/internal/yamlio"
)

// Load reads dir as an .osf project, or as a project of OSF YAML or JSON
// resources when it holds no .osf files. The project is nil only when nothing
// could be read.
func Load(dir string) (*osf.Project, []diag.Diagnostic) {
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return nil, []diag.Diagnostic{diag.NoProject.At(diag.Pos{File: dir}, diag.Pos{}, "not a folder")}
	}
	if files, _ := source.Files(dir, ".osf"); len(files) > 0 {
		return lang.LoadDir(dir)
	}
	return yamlio.LoadDir(dir)
}

// Analyze loads and checks a project. Everything that parsed is checked, so
// one broken file does not hide problems elsewhere. Diagnostics come back
// sorted by file and position.
func Analyze(dir string) (*osf.Project, []diag.Diagnostic) {
	p, diags := Load(dir)
	if p != nil {
		diags = append(diags, check.Project(p).All()...)
	}
	diag.Sort(diags)
	return p, diags
}

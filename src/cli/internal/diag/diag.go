// Package diag holds what every part of osf uses to report a problem: a place
// in a source file, the diagnostic itself, and the table of rules with their
// stable codes. It depends on nothing else in this module.
package diag

import (
	"fmt"
	"sort"
)

// Pos is a place in a source file. Line and Col start at 1; zero means unknown.
type Pos struct {
	File string
	Line int
	Col  int
}

func (p Pos) String() string {
	switch {
	case p.File == "":
		return ""
	case p.Line == 0:
		return p.File
	case p.Col == 0:
		return fmt.Sprintf("%s:%d", p.File, p.Line)
	}
	return fmt.Sprintf("%s:%d:%d", p.File, p.Line, p.Col)
}

type Severity int

const (
	Error Severity = iota
	Warning
)

func (s Severity) String() string {
	if s == Warning {
		return "warning"
	}
	return "error"
}

// Diagnostic is an error or warning tied to a place in the project. Pos is where the
// problem starts; End is the position just after its last character (the same
// line and column as Pos when unknown). Code is the rule code from
// Rules, such as OSF112. Suggestion, when set, is the text a quick-fix
// would put at the range (a close id for a misspelled reference).
type Diagnostic struct {
	Pos        Pos
	End        Pos
	Code       string
	Severity   Severity
	Msg        string
	Suggestion string
}

// EndPos returns End, or Pos when no end was recorded.
func (d Diagnostic) EndPos() Pos {
	if d.End.Line == 0 {
		return d.Pos
	}
	return d.End
}

// Message is the message with the suggestion appended:
// `speaker: unknown character "strnger"; did you mean "stranger"?`.
func (d Diagnostic) Message() string {
	if d.Suggestion == "" {
		return d.Msg
	}
	return fmt.Sprintf("%s; did you mean %q?", d.Msg, d.Suggestion)
}

// String is the terminal form: `file:line:col: error: OSF112 message`.
func (d Diagnostic) String() string {
	s := d.Severity.String() + ": "
	if d.Code != "" {
		s += d.Code + " "
	}
	s += d.Message()
	if p := d.Pos.String(); p != "" {
		return p + ": " + s
	}
	return s
}

func (d Diagnostic) Error() string { return d.String() }

// Sort orders diagnostics by file, line and column, keeping the emission
// order of diagnostics at the same place.
func Sort(diags []Diagnostic) {
	sort.SliceStable(diags, func(i, j int) bool {
		a, b := diags[i].Pos, diags[j].Pos
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Col < b.Col
	})
}

func Count(diags []Diagnostic) (errors, warnings int) {
	for _, d := range diags {
		if d.Severity == Warning {
			warnings++
		} else {
			errors++
		}
	}
	return
}

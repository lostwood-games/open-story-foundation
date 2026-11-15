package diag

import "encoding/json"

// JSONPos is a 1-based line and column, as printed on the terminal.
type JSONPos struct {
	Line int `json:"line"`
	Col  int `json:"col"`
}

type JSONRange struct {
	Start JSONPos `json:"start"`
	End   JSONPos `json:"end"`
}

// JSONDiag is the shape of one diagnostic in `osf validate -o json`.
type JSONDiag struct {
	File       string    `json:"file"`
	Range      JSONRange `json:"range"`
	Severity   string    `json:"severity"`
	Code       string    `json:"code"`
	Message    string    `json:"message"`
	Suggestion string    `json:"suggestion,omitempty"`
}

type JSONSummary struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
}

type JSONReport struct {
	Diagnostics []JSONDiag  `json:"diagnostics"`
	Summary     JSONSummary `json:"summary"`
}

// Report converts diagnostics to their JSON form, in the given order.
func Report(diags []Diagnostic) JSONReport {
	r := JSONReport{Diagnostics: []JSONDiag{}}
	for _, d := range diags {
		end := d.EndPos()
		r.Diagnostics = append(r.Diagnostics, JSONDiag{
			File:       d.Pos.File,
			Range:      JSONRange{JSONPos{d.Pos.Line, d.Pos.Col}, JSONPos{end.Line, end.Col}},
			Severity:   d.Severity.String(),
			Code:       d.Code,
			Message:    d.Message(),
			Suggestion: d.Suggestion,
		})
	}
	r.Summary.Errors, r.Summary.Warnings = Count(diags)
	return r
}

// Marshal renders diagnostics as the JSON `osf validate -o json`
// prints, with a trailing newline.
func Marshal(diags []Diagnostic) ([]byte, error) {
	b, err := json.Marshal(Report(diags))
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

package check

import "testing"

func TestSuggest(t *testing.T) {
	ids := []string{"hero", "stranger", "villain", "leave"}
	tests := []struct{ name, want string }{
		{"strnger", "stranger"},
		{"hreo", "hero"},
		{"Hero", "hero"},
		{"leav", "leave"},
		{"stanrger", "stranger"}, // one swap
		{"x", ""},                // too far from everything
		{"hero", ""},             // exact match is not a suggestion
		{"", ""},
	}
	for _, tt := range tests {
		if got := Suggest(tt.name, ids); got != tt.want {
			t.Errorf("Suggest(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
	if d := Distance("kitten", "sitting"); d != 3 {
		t.Errorf("Distance(kitten, sitting) = %d, want 3", d)
	}
	if d := Distance("ab", "ba"); d != 1 {
		t.Errorf("Distance(ab, ba) = %d, want 1 (one swap)", d)
	}
}

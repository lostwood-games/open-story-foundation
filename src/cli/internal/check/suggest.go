package check

import (
	"sort"
	"strings"
)

// Suggest returns the candidate closest to name when it is close enough to be
// a likely typo, else "". Closeness is the Damerau-Levenshtein distance
// (insert, delete, replace, swap adjacent): at most 2, or a third of the
// name's length for long names. A candidate that differs only in case always
// wins. Ties go to the candidate that sorts first.
func Suggest(name string, candidates []string) string {
	if name == "" {
		return ""
	}
	sorted := append([]string(nil), candidates...)
	sort.Strings(sorted)
	limit := max(2, len(name)/3)
	best, bestD := "", limit+1
	for _, c := range sorted {
		if c == name {
			continue
		}
		if strings.EqualFold(c, name) {
			return c
		}
		d := Distance(name, c)
		if d < bestD && d < len(c) {
			best, bestD = c, d
		}
	}
	return best
}

// Distance is the optimal string alignment distance between a and b, on runes.
func Distance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev2 := make([]int, len(rb)+1)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
			if i > 1 && j > 1 && ra[i-1] == rb[j-2] && ra[i-2] == rb[j-1] {
				cur[j] = min(cur[j], prev2[j-2]+1)
			}
		}
		prev2, prev, cur = prev, cur, prev2
	}
	return prev[len(rb)]
}

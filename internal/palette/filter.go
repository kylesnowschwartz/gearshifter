package palette

import (
	"sort"
	"strings"

	"github.com/kylesnowschwartz/gearshifter/internal/catalog"
)

// matchScore rates how well query matches text: higher is better, -1 is no
// match. Exact prefix beats substring beats scattered subsequence, so short
// queries surface the command you started typing.
func matchScore(text, query string) int {
	text = strings.ToLower(text)
	query = strings.ToLower(query)
	switch {
	case query == "":
		return 0
	case strings.HasPrefix(text, query):
		return 100
	case strings.Contains(text, query):
		return 50
	}
	// subsequence: every query rune appears in order
	i := 0
	for _, r := range text {
		if i < len(query) && r == rune(query[i]) {
			i++
		}
	}
	if i == len(query) {
		return 10
	}
	return -1
}

// filterCommands returns indices into commands ranked by match quality
// against query. Name is the primary field; description is a weak fallback
// so e.g. "side question" still finds /btw. Empty query returns all indices
// in catalog order.
func filterCommands(commands []catalog.Command, query string) []int {
	idx := make([]int, 0, len(commands))
	if query == "" {
		for i := range commands {
			idx = append(idx, i)
		}
		return idx
	}
	scores := make(map[int]int, len(commands))
	for i, c := range commands {
		s := matchScore(c.Name, query)
		if s < 0 {
			// description is a weak fallback; require a literal substring —
			// subsequence over long prose would match almost anything
			if strings.Contains(strings.ToLower(c.Description), strings.ToLower(query)) {
				s = 1
			}
		}
		if s >= 0 {
			idx = append(idx, i)
			scores[i] = s
		}
	}
	sort.SliceStable(idx, func(a, b int) bool { return scores[idx[a]] > scores[idx[b]] })
	return idx
}

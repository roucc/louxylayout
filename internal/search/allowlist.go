package search

import (
	"fmt"
	"strings"
)

// AllowlistedSubstrings validates explicitly accepted searches against the
// target's recipe-group names, while allowing any extra recipe matches.
// This does not change filtering for other searches or other goals.
func (search *Search) AllowlistedSubstrings(targetItem string, allowed []string) ([]string, error) {
	if len(allowed) == 0 {
		return nil, nil
	}
	candidates := make(map[string]bool)
	forEachCandidateSub(search.candidateNamesFor(targetItem), func(_, lower string) { candidates[lower] = true })
	seen := make(map[string]bool, len(allowed))
	var result []string
	for _, sub := range allowed {
		lower := strings.ToLower(sub)
		if !candidates[lower] {
			return nil, fmt.Errorf("allowed junk search %q for %q is not a target candidate", sub, targetItem)
		}
		if !seen[lower] {
			result = append(result, sub)
			seen[lower] = true
		}
	}
	return result, nil
}

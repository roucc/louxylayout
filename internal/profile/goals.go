package profile

import (
	"fmt"
	"strings"

	"louxylayout/internal/layout"
	"louxylayout/internal/search"
)

// BuildGoals applies this profile's junk policy, explicit exceptions and search
// restrictions consistently for the CLI, tests, and library callers.
func (p Profile) BuildGoals(finder *search.Search) ([]layout.Goal, error) {
	known := make(map[string]bool, len(p.Goals))
	for _, item := range p.Goals {
		known[item] = true
	}
	for item, allowed := range p.AllowedJunkSearches {
		if len(allowed) > 0 && !known[item] {
			return nil, fmt.Errorf("allowed junk searches refer to unknown goal %q", item)
		}
	}
	goals := make([]layout.Goal, 0, len(p.Goals))
	for _, item := range p.Goals {
		goal := layout.Goal{Item: item}
		candidates := finder.JunklessSubstrings(item)
		if p.AllowGoodJunk {
			candidates = finder.ShortestUniqueSubstringWithJunk(item)
		}
		seen := make(map[string]bool, len(candidates))
		for _, candidate := range candidates {
			goal.Substrings = append(goal.Substrings, candidate.Sub)
			seen[strings.ToLower(candidate.Sub)] = true
		}
		extra, err := finder.AllowlistedSubstrings(item, p.AllowedJunkSearches[item])
		if err != nil {
			return nil, err
		}
		for _, sub := range extra {
			if !seen[strings.ToLower(sub)] {
				goal.Substrings = append(goal.Substrings, sub)
				seen[strings.ToLower(sub)] = true
			}
		}
		goals = append(goals, goal)
	}
	return layout.WithPreferredSearches(goals, p.PreferredSearches)
}

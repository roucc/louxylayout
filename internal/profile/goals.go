package profile

import (
	"louxylayout/internal/layout"
	"louxylayout/internal/search"
)

// BuildGoals applies the selected profile's junk policy and search restrictions.
func (p Profile) BuildGoals(finder *search.Search) ([]layout.Goal, error) {
	var goals []layout.Goal
	for _, item := range p.Goals {
		goal := layout.Goal{Item: item}
		candidates := finder.JunklessSubstrings(item)
		if p.AllowGoodJunk {
			candidates = finder.ShortestUniqueSubstringWithJunk(item)
		}
		for _, candidate := range candidates {
			goal.Substrings = append(goal.Substrings, candidate.Sub)
		}
		goals = append(goals, goal)
	}
	return layout.WithPreferredSearches(goals, p.PreferredSearches)
}

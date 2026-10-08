package layout

import (
	"louxylayout/internal/config"
	"louxylayout/internal/data"
	"louxylayout/internal/search"
	"testing"
)

func BenchmarkOptimizeCraftGroups(b *testing.B) {
	groups, items, err := data.Load(config.Language)
	if err != nil {
		b.Fatal(err)
	}
	finder := search.New(groups, items, config.Inventory, config.Goals)
	var goals []Goal
	for _, item := range config.Goals {
		goal := Goal{Item: item}
		candidates := finder.JunklessSubstrings(item)
		if config.AllowGoodJunk {
			candidates = finder.ShortestUniqueSubstringWithJunk(item)
		}
		for _, candidate := range candidates {
			goal.Substrings = append(goal.Substrings, candidate.Sub)
		}
		goals = append(goals, goal)
	}
	goals, err = WithPreferredSearches(goals, config.PreferredSearches)
	if err != nil {
		b.Fatal(err)
	}
	minimum, err := MinimumCharacters(goals)
	if err != nil {
		b.Fatal(err)
	}
	options := OptimizeOptions{Seed: 42, Fixed: Layout{' ': "Space"}, InitialCharacters: minimum.Characters, Restarts: 1, Iterations: 200, InitialBindings: PreferredBindings}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Optimize(goals, KeyboardWeights, options); err != nil {
			b.Fatal(err)
		}
	}
}

func TestNorwegianBedPreferencesAreValid(t *testing.T) {
	groups, items, err := data.Load("no_no")
	if err != nil {
		t.Fatal(err)
	}
	finder := search.New(groups, items, config.Inventory, config.Goals)
	goal := Goal{Item: "block.minecraft.white_bed"}
	for _, candidate := range finder.JunklessSubstrings(goal.Item) {
		goal.Substrings = append(goal.Substrings, candidate.Sub)
	}
	if _, err := WithPreferredSearches([]Goal{goal}, map[string][]string{goal.Item: {"l ", "n "}}); err != nil {
		t.Fatal(err)
	}
}

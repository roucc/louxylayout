package main

import (
	"fmt"
	"log"
	"louxylayout/internal/config"
	"louxylayout/internal/data"
	"louxylayout/internal/layout"
	"louxylayout/internal/search"
	"sort"
)

func main() {
	groups, items, err := data.Load(config.Language)
	if err != nil {
		log.Fatal(err)
	}
	finder := search.New(groups, items, config.Inventory, config.Goals)
	fmt.Printf("Loaded %d recipe groups\n", len(groups))
	fmt.Printf("Found %d craftable items\n", finder.CraftableCount())
	fmt.Print("\n----------------------------------------------\n\n")

	// find best crafts for specified goals
	// for _, item := range config.Goals {
	// 	var pure, junk []search.SubResult
	// 	for _, r := range finder.ShortestUniqueSubstringWithJunk(item) {
	// 		if len(r.Also) == 0 {
	// 			pure = append(pure, r)
	// 		} else {
	// 			junk = append(junk, r)
	// 		}
	// 	}
	//
	// 	var p []string
	// 	for _, r := range pure[:min(len(pure), 3)] {
	// 		p = append(p, strings.ReplaceAll(r.Sub, " ", "_"))
	// 	}
	//
	// 	var j []string
	// 	for _, r := range junk[:min(len(junk), 3)] {
	// 		var names []string
	// 		for _, a := range r.Also {
	// 			names = append(names, items[a])
	// 		}
	// 		j = append(j, fmt.Sprintf("%s (+ %s)", strings.ReplaceAll(r.Sub, " ", "_"), strings.Join(names, ", ")))
	// 	}
	//
	// 	fmt.Printf("%s\n  pure:      [%s]\n  good junk: [%s]\n", item, strings.Join(p, ", "), strings.Join(j, "; "))
	// 	fmt.Println()
	// }

	var goals []layout.Goal
	for _, item := range config.Goals {
		goal := layout.Goal{Item: item}
		candidates := finder.JunklessSubstrings(item)
		if config.AllowGoodJunk {
			candidates = finder.ShortestUniqueSubstringWithJunk(item)
		}
		for _, candidate := range candidates {
			goal.Substrings = append(goal.Substrings, candidate.Sub)
		}
		goals = append(goals, goal)
	}
	goals, err = layout.WithPreferredSearches(goals, config.PreferredSearches)
	if err != nil {
		log.Fatal(err)
	}
	result, err := layout.MinimumCharacters(goals)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Minimum characters (%d): %s\n", len(result.Characters), string(result.Characters))

	// Optimize all original candidates, allowing the character set to change.
	keys := make([]string, 0, len(layout.KeyboardWeights.Keys))
	for key := range layout.KeyboardWeights.Keys {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	optimized, err := layout.Optimize(goals, layout.KeyboardWeights, layout.OptimizeOptions{
		Seed:              42,
		Fixed:             layout.Layout{' ': "Space"},
		InitialCharacters: result.Characters,
		InitialBindings:   layout.PreferredBindings,
	})
	if err != nil {
		log.Fatal(err)
	}

	bindings := optimized.Bindings
	keyCharacters := make(map[string]rune, len(bindings))
	for ch, key := range bindings {
		keyCharacters[key] = ch
	}
	fmt.Println("Rebindings (physical key → typed character):")
	for _, key := range keys {
		if ch, assigned := keyCharacters[key]; assigned {
			if ch == ' ' {
				fmt.Printf("  %s → Space\n", key)
			} else {
				fmt.Printf("  %s → %c\n", key, ch)
			}
		} else {
			fmt.Printf("  %s → unassigned\n", key)
		}
	}
	fmt.Printf("Optimized characters: %d\n", len(bindings))
	fmt.Println("Cost:", optimized.Cost)
	fmt.Println("Selected searches:")
	for _, goal := range optimized.Searches {
		fmt.Printf("  %s: %q\n", goal.Item, goal.Substrings)
	}

}

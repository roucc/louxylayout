package main

import (
	"fmt"
	"log"
	"louxylayout/internal/config"
	"louxylayout/internal/data"
	"louxylayout/internal/layout"
	"louxylayout/internal/search"
	"sort"
	"time"
)

func main() {
	started := time.Now()
	weights := layout.KeyboardWeights
	options := layout.OptimizeOptions{Seed: 42, Fixed: layout.Layout{' ': "Space"}, InitialBindings: layout.PreferredBindings, InitialShiftBindings: layout.PreferredShiftBindings, EnableShiftLayer: true}
	groups, items, err := data.Load(config.Language)
	if err != nil {
		log.Fatal(err)
	}
	finder := search.New(groups, items, config.Inventory, config.Goals)
	fmt.Printf("Loaded %d recipe groups\n", len(groups))
	fmt.Printf("Found %d craftable items\n", finder.CraftableCount())
	fmt.Print("\n----------------------------------------------\n\n")

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
	keys := make([]string, 0, len(weights.Keys))
	for key := range weights.Keys {
		keys = append(keys, key)
	}
	keyOrder := []string{"1", "2", "3", "4", "5", "Q", "W", "E", "R", "T", "A", "S", "D", "F", "G", "H", "Z", "X", "C", "V", "B", "Space"}
	keyRank := make(map[string]int, len(keyOrder))
	for i, key := range keyOrder {
		keyRank[key] = i
	}
	sort.Slice(keys, func(i, j int) bool {
		first, knownFirst := keyRank[keys[i]]
		second, knownSecond := keyRank[keys[j]]
		if knownFirst != knownSecond {
			return knownFirst
		}
		if knownFirst {
			return first < second
		}
		return keys[i] < keys[j]
	})

	options.InitialCharacters = result.Characters
	optimized, err := layout.Optimize(goals, weights, options)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Layout generation time: %s\n", time.Since(started).Round(time.Millisecond))

	bindings := optimized.Bindings
	layered := layout.LayeredLayout{NonShift: optimized.Bindings, Shift: optimized.ShiftBindings}
	keyCharacters := make(map[string]rune, len(bindings))
	shiftCharacters := make(map[string]rune, len(optimized.ShiftBindings))
	for ch, key := range bindings {
		keyCharacters[key] = ch
	}
	for ch, key := range optimized.ShiftBindings {
		shiftCharacters[key] = ch
	}
	describe := func(characters map[string]rune, key string) string {
		ch, assigned := characters[key]
		if !assigned {
			return "unassigned"
		}
		if ch == ' ' {
			return "Space"
		}
		return string(ch)
	}
	if optimized.ShiftBindings != nil {
		fmt.Println("Rebindings (physical key → non-shift / shift character):")
		for _, key := range keys {
			fmt.Printf("  %s → %s / %s\n", key, describe(keyCharacters, key), describe(shiftCharacters, key))
		}
		fmt.Printf("Assigned characters: %d non-shift, %d shift\n", len(bindings), len(optimized.ShiftBindings))
	} else {
		fmt.Println("Rebindings (physical key → typed character):")
		for _, key := range keys {
			fmt.Printf("  %s → %s\n", key, describe(keyCharacters, key))
		}
		fmt.Printf("Assigned characters: %d\n", len(bindings))
	}
	fmt.Println("Cost:", optimized.Cost)
	fmt.Println("Selected searches:")
	if optimized.ShiftBindings == nil {
		for _, goal := range optimized.Searches {
			fmt.Printf("  %s: %q\n", goal.Item, goal.Substrings)
		}
	}
	for _, choice := range optimized.LayeredSearches {
		click := "click"
		if choice.Layer == layout.ShiftLayer {
			click = "shift-click"
		}
		fmt.Printf("  %s: %q (%s, type on %v, %s)\n", choice.Item, choice.Search, choice.Layer, choice.Keys, click)
	}
	if len(optimized.GroupPlans) > 0 {
		fmt.Println("Group sequences (SH = Shift+Home, BS = Backspace):")
		for _, plan := range optimized.GroupPlans {
			fmt.Printf("  %s (priority %g, edit cost %.2f):\n", plan.Name, plan.Priority, plan.Cost)
			for _, step := range plan.Steps {
				operation := ""
				switch step.Action {
				case "shift-home":
					operation = "SH "
				case "backspace":
					operation = fmt.Sprintf("BS×%d ", step.Backspaces)
				case "keep":
					operation = "keep "
				}
				if step.LayerChange {
					change := "release Shift "
					if step.Layer == layout.ShiftLayer {
						change = "press Shift "
					}
					operation = change + operation
				}
				keys := make([]string, 0, len([]rune(step.Type)))
				for _, ch := range step.Type {
					keys = append(keys, layered.Bindings(step.Layer)[ch])
				}
				if step.Layer == layout.AnyLayer {
					fmt.Printf("    %s: %s%q → %q (type on %v)\n", step.Item, operation, step.Type, step.Search, keys)
				} else {
					fmt.Printf("    %s: %s%q → %q (layer %s, type on %v)\n", step.Item, operation, step.Type, step.Search, step.Layer, keys)
				}
			}
		}
	}

}

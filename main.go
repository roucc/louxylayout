package main

import (
	"fmt"
	"log"
	"louxylayout/internal/config"
	"louxylayout/internal/data"
	"louxylayout/internal/search"
	"strings"
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
	for _, item := range config.Goals {
		var pure, junk []search.SubResult
		for _, r := range finder.ShortestUniqueSubstringWithJunk(item) {
			if len(r.Also) == 0 {
				pure = append(pure, r)
			} else {
				junk = append(junk, r)
			}
		}

		var p []string
		for _, r := range pure[:min(len(pure), 3)] {
			p = append(p, strings.ReplaceAll(r.Sub, " ", "_"))
		}

		var j []string
		for _, r := range junk[:min(len(junk), 3)] {
			var names []string
			for _, a := range r.Also {
				names = append(names, items[a])
			}
			j = append(j, fmt.Sprintf("%s (+ %s)", strings.ReplaceAll(r.Sub, " ", "_"), strings.Join(names, ", ")))
		}

		fmt.Printf("%s\n  pure:      [%s]\n  good junk: [%s]\n", item, strings.Join(p, ", "), strings.Join(j, "; "))
		fmt.Println()
	}
}

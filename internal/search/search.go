package search

import "louxylayout/internal/data"

type Search struct {
	Items              map[string]string
	RecipeGroups       [][]data.Recipe
	craftableItems     []string
	craftableSet       map[string]bool
	craftableGroupSize map[int]int
	itemMinSize        map[string]int
	itemGroupMap       map[string]int
	goalGroups         map[int][]string
}

func New(groups [][]data.Recipe, items map[string]string, inventory, goals []string) *Search {
	search := &Search{Items: items, RecipeGroups: groups}
	search.precomputeCraftableItems(inventory)
	search.buildGoalGroups(goals)
	return search
}
func (search *Search) CraftableCount() int { return len(search.craftableItems) }

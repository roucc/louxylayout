package search

import "louxylayout/internal/data"

func (search *Search) buildGoalGroups(goals []string) {
	search.goalGroups = make(map[int][]string)
	for _, g := range goals {
		if idx, ok := search.itemGroupMap[g]; ok {
			search.goalGroups[idx] = append(search.goalGroups[idx], g)
		}
	}
}

func (search *Search) precomputeCraftableItems(inventory []string) {
	available := make(map[string]bool, len(inventory))
	for _, item := range inventory {
		available[item] = true
	}

	search.craftableSet = make(map[string]bool)
	search.craftableItems = nil
	search.craftableGroupSize = make(map[int]int)
	search.itemMinSize = make(map[string]int)
	search.itemGroupMap = make(map[string]int)

	for groupIdx, group := range search.RecipeGroups {
		// map every output to its group first, so nothing is skipped
		for _, recipe := range group {
			search.itemGroupMap[recipe.Output] = groupIdx
		}

		for _, recipe := range group {
			if !canCraft(recipe, available) {
				continue
			}

			if cur, ok := search.itemMinSize[recipe.Output]; !ok || recipe.Size < cur {
				search.itemMinSize[recipe.Output] = recipe.Size
			}
			if cur, ok := search.craftableGroupSize[groupIdx]; !ok || recipe.Size < cur {
				search.craftableGroupSize[groupIdx] = recipe.Size
			}
			if !search.craftableSet[recipe.Output] {
				search.craftableSet[recipe.Output] = true
				search.craftableItems = append(search.craftableItems, recipe.Output)
			}
		}
	}
}

// canCraft checks each recipe option and each slot in the recipe compared to the available items
func canCraft(recipe data.Recipe, available map[string]bool) bool {
	for _, options := range recipe.Ingredients {
		if len(options) == 0 {
			continue
		}

		hasIngredients := false
		for _, option := range options {
			if available[option] {
				hasIngredients = true
				break
			}
		}

		if !hasIngredients {
			return false
		}
	}
	return true
}

// isCraftable is an O(1) lookup to check if an item is craftable
// requires full internal name e.g. item.minecraft.iron_axe
func (search *Search) IsCraftable(item string) bool {
	return search.craftableSet[item]
}

package search

import "strings"

// groupNameContains reports whether any item in the group has a name containing lowerSub
func (search *Search) groupNameContains(groupIdx int, lowerSub string) bool {
	for _, recipe := range search.RecipeGroups[groupIdx] {
		if strings.Contains(strings.ToLower(search.Items[recipe.Output]), lowerSub) {
			return true
		}
	}
	return false
}

// FindVisibleItemsFromSub returns what the in-game search would show for a
// substring: every member of any craftable group where some member's name matches
func (search *Search) FindVisibleItemsFromSub(sub string) []string {
	lowerSub := strings.ToLower(sub)
	var found []string
	for groupIdx, size := range search.craftableGroupSize {
		if size > search.gridSize() {
			continue
		}
		if search.groupNameContains(groupIdx, lowerSub) {
			for _, recipe := range search.RecipeGroups[groupIdx] {
				found = append(found, recipe.Output)
			}
		}
	}
	return found
}

// IsSameGroup checks if two items share a recipe group
func (search *Search) IsSameGroup(A, B string) bool {
	gA, okA := search.itemGroupMap[A]
	gB, okB := search.itemGroupMap[B]
	return okA && okB && gA == gB
}

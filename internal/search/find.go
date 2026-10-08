package search

import "strings"

// FindItemsFromSub returns all items found from a name substring
// returns as full internal name e.g. item.minecraft.iron_axe
func (search *Search) FindItemsFromSub(sub string) []string {
	var found []string
	for item, name := range search.Items {
		if strings.Contains(name, sub) {
			found = append(found, item)
		}
	}
	return found
}

// FindCraftableItemsFromSub returns craftable items found from a name substring
// take a word (slice of a name) and returns full internal name
func (search *Search) FindCraftableItemsFromSub(sub string) []string {
	var found []string
	items := search.FindItemsFromSub(sub)
	for _, item := range items {
		if search.IsCraftable(item) {
			found = append(found, item)
		}
	}
	return found
}

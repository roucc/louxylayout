package search

import (
	"sort"
	"strings"
)

const maxSubLen = 10

// candidateNamesFor returns the names of every item in the target's recipe group
// (or just the item's own name if it has no group)
func (search *Search) candidateNamesFor(targetItem string) []string {
	var names []string
	if groupIdx, ok := search.itemGroupMap[targetItem]; ok {
		for _, recipe := range search.RecipeGroups[groupIdx] {
			if name, exists := search.Items[recipe.Output]; exists {
				names = append(names, name)
			}
		}
	} else if name, exists := search.Items[targetItem]; exists {
		names = append(names, name)
	}
	return names
}

// gridSize returns the grid for visibility queries without a target.
func (search *Search) gridSize() int {
	if search.GridSize != 0 {
		return search.GridSize
	}
	return 3
}

// targetGridSize reuses the recipe index, with a minimum of the actual 2x2
// inventory grid. A 1-slot recipe still shows competing 2x2 recipes.
func (search *Search) targetGridSize(targetItem string) int {
	if search.GridSize != 0 {
		return search.GridSize
	}
	if size := search.itemMinSize[targetItem]; size != 0 {
		return max(2, size)
	}
	return 3
}

// targetGroupIdx returns the item's recipe group, or -1 if it has none
func (search *Search) targetGroupIdx(targetItem string) int {
	if idx, ok := search.itemGroupMap[targetItem]; ok {
		return idx
	}
	return -1
}

// forEachCandidateSub calls fn once for every distinct (case-insensitive)
// substring of the given names, up to maxSubLen runes long
func forEachCandidateSub(names []string, fn func(sub, lowerSub string)) {
	seen := make(map[string]bool)
	for _, name := range names {
		runes := []rune(name) // runes so æ, ø, å are handled correctly
		n := len(runes)
		for length := 1; length <= n && length <= maxSubLen; length++ {
			for p0 := 0; p0 <= n-length; p0++ {
				sub := string(runes[p0 : p0+length])
				lower := strings.ToLower(sub)
				if seen[lower] {
					continue
				}
				seen[lower] = true
				fn(sub, lower)
			}
		}
	}
}

// ShortestUniqueSubstringForItem finds the shortest unique identifier for an item
// if craftableOnly is true, uniqueness is only checked against search.craftableItems
// otherwise all items in the game are checked
// targetItem is full internal name e.g. item.minecraft.iron_axe
func (search *Search) ShortestUniqueSubstringForItem(targetItem string, craftableOnly bool) []string {
	targetSize := search.targetGridSize(targetItem)
	targetGroup := search.targetGroupIdx(targetItem)

	var subs []string
	forEachCandidateSub(search.candidateNamesFor(targetItem), func(sub, lowerSub string) {
		unique := true

		if craftableOnly {
			for groupIdx, size := range search.craftableGroupSize {
				if groupIdx == targetGroup || size > targetSize {
					continue
				}
				if search.groupNameContains(groupIdx, lowerSub) {
					unique = false
					break
				}
			}
		} else {
			for item, name := range search.Items {
				if item == targetItem || search.IsSameGroup(targetItem, item) {
					continue
				}
				if strings.Contains(strings.ToLower(name), lowerSub) {
					unique = false
					break
				}
			}
		}

		if unique {
			subs = append(subs, sub)
		}
	})

	sort.SliceStable(subs, func(i, j int) bool {
		return len([]rune(subs[i])) < len([]rune(subs[j]))
	})
	return subs
}

type SubResult struct {
	Sub         string
	Also        []string // other goal items this substring also shows (empty = pure)
	Unavoidable []string // otherwise-disallowed items every candidate also shows
}

// ShortestUniqueSubstringWithJunk is craftable-only. Substrings that match other
// craftable groups are accepted if they are goals or unavoidable for this target.
func (search *Search) ShortestUniqueSubstringWithJunk(targetItem string) []SubResult {
	return search.substringsWithJunk(targetItem, true)
}

// JunklessSubstrings rejects other goal groups as well as avoidable non-goal
// groups. Unavoidable matches are retained and reported in Unavoidable.
func (search *Search) JunklessSubstrings(targetItem string) []SubResult {
	return search.substringsWithJunk(targetItem, false)
}

func (search *Search) substringsWithJunk(targetItem string, allowGoodJunk bool) []SubResult {
	targetSize := search.targetGridSize(targetItem)
	targetGroup := search.targetGroupIdx(targetItem)

	// A disallowed group is unavoidable when every candidate matches it.
	unavoidable := make(map[int]bool)
	for groupIdx, size := range search.craftableGroupSize {
		_, isGoal := search.goalGroups[groupIdx]
		if groupIdx != targetGroup && size <= targetSize && (!isGoal || !allowGoodJunk) {
			unavoidable[groupIdx] = true
		}
	}
	names := search.candidateNamesFor(targetItem)
	forEachCandidateSub(names, func(_, lowerSub string) {
		for groupIdx := range unavoidable {
			if !search.groupNameContains(groupIdx, lowerSub) {
				delete(unavoidable, groupIdx)
			}
		}
	})
	var unavoidableItems []string
	for groupIdx := range unavoidable {
		for _, recipe := range search.RecipeGroups[groupIdx] {
			unavoidableItems = append(unavoidableItems, recipe.Output)
		}
	}
	sort.Strings(unavoidableItems)

	var results []SubResult
	forEachCandidateSub(names, func(sub, lowerSub string) {
		var also []string

		for groupIdx, size := range search.craftableGroupSize {
			if groupIdx == targetGroup || size > targetSize {
				continue
			}
			if !search.groupNameContains(groupIdx, lowerSub) {
				continue
			}
			goalItems, isGoal := search.goalGroups[groupIdx]
			if unavoidable[groupIdx] {
				continue
			}
			if !isGoal || !allowGoodJunk {
				return // adds avoidable non-goal junk: reject
			}
			also = append(also, goalItems...)
		}

		sort.Strings(also) // map iteration order is random
		results = append(results, SubResult{Sub: sub, Also: also, Unavoidable: unavoidableItems})
	})

	sort.SliceStable(results, func(i, j int) bool {
		li, lj := len([]rune(results[i].Sub)), len([]rune(results[j].Sub))
		if li != lj {
			return li < lj
		}
		return len(results[i].Also) < len(results[j].Also)
	})
	return results
}

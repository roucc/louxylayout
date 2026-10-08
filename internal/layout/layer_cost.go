package layout

import (
	"math"
	"strings"
	"unicode"
)

func layerPenalty(item string, layer CraftLayer, weights Weights) float64 {
	preferred := weights.PreferredLayers[item]
	if preferred != AnyLayer && preferred != layer {
		return weights.LayerPreferencePenalty
	}
	return 0
}

func craftImportance(item string, weights Weights) float64 {
	if importance, ok := weights.Crafts[item]; ok {
		return importance
	}
	return 1
}

func cheapestLayeredSearch(goal Goal, bindings LayeredLayout, weights Weights) LayeredSearch {
	best := LayeredSearch{Item: goal.Item, Cost: math.Inf(1)}
	for _, layer := range []CraftLayer{NonShiftLayer, ShiftLayer} {
		sub, cost := cheapestSearch(goal, bindings.Bindings(layer), weights)
		cost += layerPenalty(goal.Item, layer, weights)
		if layer == ShiftLayer {
			cost += weights.LayerSwitchPenalty
		}
		if cost < best.Cost {
			best.Search, best.Layer, best.Cost = sub, layer, cost
		}
	}
	if best.Search != "" {
		for _, ch := range strings.ToLower(best.Search) {
			best.Keys = append(best.Keys, bindings.Bindings(best.Layer)[ch])
		}
	}
	return best
}

func bothLayersCost(bindings LayeredLayout, weights Weights) float64 {
	total := 0.0
	for ch, penalty := range weights.PreferBothLayers {
		ch = unicode.ToLower(ch)
		for _, layer := range []Layout{bindings.NonShift, bindings.Shift} {
			key, assigned := layer[ch]
			_, known := weights.Keys[key]
			if !assigned || !known {
				total += penalty
			}
		}
	}
	return total
}

func layeredScore(bindings LayeredLayout, goals []Goal, weights Weights, missingPenalty float64) (float64, bool, []LayeredSearch) {
	total, feasible := bothLayersCost(bindings, weights), true
	choices := make([]LayeredSearch, 0, len(goals))
	byItem := make(map[string]LayeredSearch, len(goals))
	for _, goal := range goals {
		importance := craftImportance(goal.Item, weights)
		if importance == 0 {
			continue
		}
		choice := cheapestLayeredSearch(goal, bindings, weights)
		if math.IsInf(choice.Cost, 1) {
			total += missingPenalty
			feasible = false
		} else {
			total += importance * choice.Cost
			byItem[goal.Item] = choice
		}
		choices = append(choices, choice)
	}
	for _, group := range weights.Groups {
		if group.Priority == 0 {
			continue
		}
		var active [][]string
		for _, item := range uniqueGroupItems(group.Crafts) {
			if choice, ok := byItem[item]; ok {
				active = append(active, choice.Keys)
			}
		}
		for i, from := range active {
			for _, to := range active[i+1:] {
				distance := 0.0
				for _, a := range from {
					first := weights.Keys[a]
					for _, b := range to {
						second := weights.Keys[b]
						distance += math.Hypot(first.X-second.X, first.Y-second.Y)
					}
				}
				total += group.Priority * (2 / float64(len(active))) * distance / float64(len(from)*len(to))
			}
		}
	}
	return total, feasible, choices
}

// LayeredCost uses the same physical-key costs on both layers, adding soft
// craft-layer preferences, missing shared characters and group editing costs.
func LayeredCost(bindings LayeredLayout, goals []Goal, weights Weights) float64 {
	total, _, _ := layeredScore(bindings, goals, weights, math.Inf(1))
	editing, _ := newLayerSequenceScorer(goals, weights).evaluateLayers(bindings, false)
	return total + editing
}

package layout

import (
	"math"
	"strings"
)

// Layout maps typed characters to physical key names, e.g. 'n' -> "A".
type Layout map[rune]string

type KeyWeight struct {
	Effort float64
	Finger string // e.g. "index"; empty means no finger penalty
}

// Transition is directional: A -> S can have a different cost from S -> A.
type Transition struct {
	From string
	To   string
}

// Weights describes physical keys independently of their assigned characters.
// Use nonnegative, finite values. Lower costs mean easier typing.
type Weights struct {
	Keys                     map[string]KeyWeight
	SameFingerPenalty        float64
	DefaultTransitionPenalty float64
	// Transitions overrides the entire transition cost for specific key pairs.
	// Unlisted pairs cost DefaultTransitionPenalty, plus SameFingerPenalty
	// when both keys use the same finger.
	Transitions map[Transition]float64
	// Crafts weights goal importance by item ID. Unlisted goals have weight 1;
	// a weight of zero excludes a goal from the total.
	Crafts map[string]float64
}

// StringCost sums physical key efforts and consecutive-key transition costs.
// Empty searches, missing characters, or keys without weights cost +Inf.
// Character lookup is case-insensitive, matching MinimumCharacters.
func StringCost(substring string, bindings Layout, weights Weights) float64 {
	if substring == "" {
		return math.Inf(1)
	}
	total := 0.0
	previous := ""
	hasPrevious := false
	for _, ch := range strings.ToLower(substring) {
		key, ok := bindings[ch]
		if !ok {
			return math.Inf(1)
		}
		weight, ok := weights.Keys[key]
		if !ok {
			return math.Inf(1)
		}
		total += weight.Effort
		if hasPrevious {
			if penalty, ok := weights.Transitions[Transition{From: previous, To: key}]; ok {
				total += penalty
			} else {
				total += weights.DefaultTransitionPenalty
				if weight.Finger != "" && weights.Keys[previous].Finger == weight.Finger {
					total += weights.SameFingerPenalty
				}
			}
		}
		previous, hasPrevious = key, true
	}
	return total
}

// Cost sums each craft's weight times its cheapest substring cost. Lower is
// better. All candidates are reconsidered on every call, including after swaps.
// A positive-weight goal with no typeable candidate makes the total +Inf.
func Cost(bindings Layout, goals []Goal, weights Weights) float64 {
	total := 0.0
	for _, goal := range goals {
		importance, ok := weights.Crafts[goal.Item]
		if !ok {
			importance = 1
		}
		if importance == 0 {
			continue
		}
		best := math.Inf(1)
		for _, substring := range goal.Substrings {
			best = math.Min(best, StringCost(substring, bindings, weights))
		}
		total += importance * best
	}
	return total
}

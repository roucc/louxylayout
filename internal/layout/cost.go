package layout

import (
	"math"
	"strings"
)

// Layout maps typed characters to physical key names, e.g. 'n' -> "A".
type Layout map[rune]string

type KeyWeight struct {
	Effort float64
	Finger string  // e.g. "index"; empty means no finger penalty
	X, Y   float64 // physical position in key-width units
}

// Transition is directional: A -> S can have a different cost from S -> A.
type Transition struct {
	From string
	To   string
}

// CraftGroup is an unordered set of crafts performed together. Priority scales
// proximity and sequence costs; zero disables the group. Groups may overlap.
type CraftGroup struct {
	Name     string
	Priority float64
	Crafts   []string            // item IDs; the planner chooses their order
	Before   map[string][]string // source craft must be visited before each target
}

// Weights describes physical keys independently of their assigned characters.
// Use nonnegative, finite values. Lower costs mean easier typing.
type Weights struct {
	// LongReachPenalty charges squared distance beyond LongReachThreshold.
	LongReachPenalty, LongReachThreshold float64
	// RowChangePenalty charges per row crossed, excluding the Space control.
	RowChangePenalty float64
	// SameFingerMovementPenalty applies to different keys sharing a finger,
	// including explicit transition overrides.
	SameFingerMovementPenalty float64
	// WorstCraftPenalty adds the largest priority-weighted standalone craft cost.
	WorstCraftPenalty float64
	// Optional multipliers default to 1; an explicit zero disables that component.
	TransitionWeight, GroupProximityWeight, GroupSequenceWeight *float64

	Keys                     map[string]KeyWeight
	SameFingerPenalty        float64
	DefaultTransitionPenalty float64
	// DistancePenalty charges per key-width travelled between consecutive keys.
	DistancePenalty float64
	// FingerReusePenalty charges for every pair of presses using the same named
	// finger within a search, including nonconsecutive presses and repeated keys.
	// This adds to SameFingerPenalty, which only applies to default transitions.
	FingerReusePenalty float64
	// Transitions overrides the base transition cost for specific key pairs.
	// DistancePenalty and FingerReusePenalty still apply to overrides.
	// Unlisted pairs cost DefaultTransitionPenalty, plus SameFingerPenalty
	// when both keys use the same finger.
	Transitions map[Transition]float64
	// Crafts weights goal importance by item ID. Unlisted goals have weight 1;
	// a weight of zero excludes a goal from the total.
	Crafts map[string]float64
	// PreferredLayers describes the desired Shift state when clicking each craft.
	// Unlisted crafts and AnyLayer have no preference. This configuration is
	// used by shift-layer scoring.
	PreferredLayers map[string]CraftLayer
	// LayerPreferencePenalty charges for the wrong layer, scaled by craft
	// priority. It is a soft cost, not a restriction; zero disables the preference.
	LayerPreferencePenalty float64
	// LayerSwitchPenalty is the cost of pressing or releasing Shift between crafts.
	LayerSwitchPenalty float64
	// PreferBothLayers charges per missing layer for these typed characters.
	// All characters can occur on both layers; this is an optional extra preference.
	PreferBothLayers map[rune]float64
	SearchEditing    *SearchEditCosts // nil disables group order/search planning
	Groups           []CraftGroup
}

// StringCost sums physical key efforts and consecutive-key transition costs.
// Empty searches, missing characters, or keys without weights cost +Inf.
// Character lookup is case-insensitive, matching MinimumCharacters.
func StringCost(substring string, bindings Layout, weights Weights) float64 {
	if substring == "" {
		return math.Inf(1)
	}
	total := 0.0
	var fingerUses map[string]int
	if weights.FingerReusePenalty > 0 {
		fingerUses = make(map[string]int)
	}
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
		if fingerUses != nil && weight.Finger != "" {
			total += weights.FingerReusePenalty * float64(fingerUses[weight.Finger])
			fingerUses[weight.Finger]++
		}
		if hasPrevious {
			total += keyPairCost(previous, key, weights)
		}
		previous, hasPrevious = key, true
	}
	return total
}

// cheapestSearch chooses deterministically, including when typing costs tie.
func cheapestSearch(goal Goal, bindings Layout, weights Weights) (string, float64) {
	choice, best := "", math.Inf(1)
	for _, sub := range goal.Substrings {
		if len(goal.PreferredSubstrings) > 0 && !preferredSearch(goal, sub) {
			continue
		}
		value := StringCost(sub, bindings, weights)
		if value < best || value == best && sub < choice {
			choice, best = sub, value
		}
	}
	return choice, best
}

// groupCost pulls all active group members together, independently of their
// configured list order. Scale the all-pairs sum by 2/n to retain n-1 edges'
// worth of proximity cost when every pair has the same distance.
func groupCost(bindings Layout, searches map[string]string, weights Weights) float64 {
	total := 0.0
	for _, group := range weights.Groups {
		if group.Priority == 0 {
			continue
		}
		var active [][]rune
		for _, item := range uniqueGroupItems(group.Crafts) {
			if sub := searches[item]; sub != "" {
				active = append(active, []rune(strings.ToLower(sub)))
			}
		}
		for i, from := range active {
			for _, to := range active[i+1:] {
				distance := 0.0
				for _, a := range from {
					first := weights.Keys[bindings[a]]
					for _, b := range to {
						second := weights.Keys[bindings[b]]
						distance += math.Hypot(first.X-second.X, first.Y-second.Y)
					}
				}
				total += multiplier(weights.GroupProximityWeight) * group.Priority * (2 / float64(len(active))) * distance / float64(len(from)*len(to))
			}
		}
	}
	return total
}

// score uses a finite missing penalty during exploration. Cost passes +Inf so
// any positive-weight goal without a typeable candidate makes the total +Inf.
func score(bindings Layout, goals []Goal, weights Weights, missingPenalty float64) (float64, bool) {
	total, feasible := 0.0, true
	worst := 0.0
	searches := make(map[string]string, len(goals))
	for _, goal := range goals {
		importance, ok := weights.Crafts[goal.Item]
		if !ok {
			importance = 1
		}
		if importance == 0 {
			continue
		}
		sub, best := cheapestSearch(goal, bindings, weights)
		if math.IsInf(best, 1) {
			total += missingPenalty
			feasible = false
		} else {
			total += importance * best
			worst = math.Max(worst, importance*best)
			searches[goal.Item] = sub
		}
	}
	return total + weights.WorstCraftPenalty*worst + groupCost(bindings, searches, weights), feasible
}

// Cost combines weighted typing effort with unordered craft-group proximity.
// All candidates are reconsidered on every call, including after swaps.
func Cost(bindings Layout, goals []Goal, weights Weights) float64 {
	total, _ := score(bindings, goals, weights, math.Inf(1))
	editing, _ := newSequenceScorer(goals, weights).evaluate(bindings, false)
	return total + editing
}

func multiplier(value *float64) float64 {
	if value == nil {
		return 1
	}
	return *value
}

// keyPairCost is shared by direct scoring and compiled group/optimizer tables.
func keyPairCost(from, to string, weights Weights) float64 {
	a, b := weights.Keys[from], weights.Keys[to]
	base, ok := weights.Transitions[Transition{From: from, To: to}]
	if !ok {
		base = weights.DefaultTransitionPenalty
		if a.Finger != "" && a.Finger == b.Finger {
			base += weights.SameFingerPenalty
		}
	}
	cost := multiplier(weights.TransitionWeight) * base
	if weights.DistancePenalty > 0 || weights.LongReachPenalty > 0 {
		distance := math.Hypot(a.X-b.X, a.Y-b.Y)
		excess := math.Max(0, distance-weights.LongReachThreshold)
		cost += weights.DistancePenalty*distance + weights.LongReachPenalty*excess*excess
	}
	if from != "Space" && to != "Space" {
		cost += weights.RowChangePenalty * math.Abs(a.Y-b.Y)
	}
	if from != to && a.Finger != "" && a.Finger == b.Finger {
		cost += weights.SameFingerMovementPenalty
	}
	return cost
}

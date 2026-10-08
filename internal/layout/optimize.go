package layout

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
)

type OptimizeOptions struct {
	Restarts          int
	Iterations        int // attempted changes per restart
	Seed              int64
	Fixed             Layout // characters that must stay on their supplied keys
	InitialCharacters []rune // a feasible starting character set
}

type Optimized struct {
	Bindings Layout
	Cost     float64
	Searches []Goal // one cheapest substring per craft
}

// Optimize explores character sets and assignments using simulated annealing.
// It keeps every goal and uses all candidates. Results are heuristic, not a
// guarantee of the global minimum. Equal-cost results prefer fewer characters.
func Optimize(goals []Goal, weights Weights, options OptimizeOptions) (Optimized, error) {
	if options.Restarts <= 0 {
		options.Restarts = 8
	}
	if options.Iterations <= 0 {
		options.Iterations = 4000
	}
	rng := rand.New(rand.NewSource(options.Seed))
	keys := make([]string, 0, len(weights.Keys))
	fixedKeys := make(map[string]bool)
	for ch, key := range options.Fixed {
		if _, ok := weights.Keys[key]; !ok {
			return Optimized{}, fmt.Errorf("fixed key %q has no weights", key)
		}
		if fixedKeys[key] || string(ch) != strings.ToLower(string(ch)) {
			return Optimized{}, fmt.Errorf("fixed bindings must use distinct keys and lowercase characters")
		}
		fixedKeys[key] = true
	}
	for key, weight := range weights.Keys {
		if weight.Effort < 0 || math.IsNaN(weight.Effort) || math.IsInf(weight.Effort, 0) {
			return Optimized{}, fmt.Errorf("invalid effort for %q", key)
		}
		if !fixedKeys[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	if weights.SameFingerPenalty < 0 || weights.DefaultTransitionPenalty < 0 || math.IsNaN(weights.SameFingerPenalty) || math.IsNaN(weights.DefaultTransitionPenalty) || math.IsInf(weights.SameFingerPenalty, 0) || math.IsInf(weights.DefaultTransitionPenalty, 0) {
		return Optimized{}, fmt.Errorf("penalties must be finite and nonnegative")
	}
	for _, penalty := range weights.Transitions {
		if penalty < 0 || math.IsNaN(penalty) || math.IsInf(penalty, 0) {
			return Optimized{}, fmt.Errorf("transition costs must be finite and nonnegative")
		}
	}
	for _, importance := range weights.Crafts {
		if importance < 0 || math.IsNaN(importance) || math.IsInf(importance, 0) {
			return Optimized{}, fmt.Errorf("craft weights must be finite and nonnegative")
		}
	}

	// A contiguous longer search cannot be cheaper with nonnegative costs.
	// This pruning preserves typing choices, unlike pruning character supersets.
	prepared := make([]Goal, 0, len(goals))
	alphabet := ""
	for _, goal := range goals {
		if importance, ok := weights.Crafts[goal.Item]; ok && importance == 0 {
			continue
		}
		subs := append([]string(nil), goal.Substrings...)
		sort.SliceStable(subs, func(i, j int) bool { return len([]rune(subs[i])) < len([]rune(subs[j])) })
		filtered := Goal{Item: goal.Item}
		for _, sub := range subs {
			if sub == "" {
				continue
			}
			lower, redundant := strings.ToLower(sub), false
			for _, kept := range filtered.Substrings {
				if strings.Contains(lower, strings.ToLower(kept)) {
					redundant = true
					break
				}
			}
			if !redundant {
				filtered.Substrings = append(filtered.Substrings, sub)
				alphabet += lower
			}
		}
		if len(filtered.Substrings) == 0 {
			return Optimized{}, fmt.Errorf("goal %q has no candidates", goal.Item)
		}
		prepared = append(prepared, filtered)
	}
	chars := []rune(characterKey(alphabet))
	var movable []rune
	for _, ch := range chars {
		if _, fixed := options.Fixed[ch]; !fixed {
			movable = append(movable, ch)
		}
	}
	seed := options.InitialCharacters
	if len(seed) == 0 && len(prepared) != 0 {
		minimum, err := MinimumCharacters(prepared)
		if err != nil {
			return Optimized{}, err
		}
		seed = minimum.Characters
	}
	var initial []rune
	for _, ch := range []rune(characterKey(string(seed))) {
		if _, fixed := options.Fixed[ch]; !fixed {
			initial = append(initial, ch)
		}
	}
	if len(initial) > len(keys) {
		return Optimized{}, fmt.Errorf("starting set needs %d assignable keys, have %d", len(initial), len(keys))
	}
	base := make(Layout)
	for ch, key := range options.Fixed {
		base[ch] = key
	}
	for i, ch := range initial {
		base[ch] = keys[i]
	}
	if math.IsInf(Cost(base, prepared, weights), 1) {
		return Optimized{}, fmt.Errorf("starting character set cannot type every goal")
	}
	best := Optimized{Cost: math.Inf(1)}

	// Penalize unreachable goals while exploring, so replacing characters can
	// cross temporary gaps in coverage. Only feasible layouts can be returned.
	maxEffort, maxTransition, maxLength := 0.0, weights.DefaultTransitionPenalty+weights.SameFingerPenalty, 0
	for _, key := range weights.Keys {
		maxEffort = math.Max(maxEffort, key.Effort)
	}
	for _, penalty := range weights.Transitions {
		maxTransition = math.Max(maxTransition, penalty)
	}
	importanceSum := 0.0
	for _, goal := range prepared {
		importance, ok := weights.Crafts[goal.Item]
		if !ok {
			importance = 1
		}
		importanceSum += importance
		for _, sub := range goal.Substrings {
			maxLength = max(maxLength, len([]rune(sub)))
		}
	}
	missingPenalty := 1 + importanceSum*float64(maxLength)*(maxEffort+maxTransition)
	evaluate := func(bindings Layout) (float64, bool) {
		total, feasible := 0.0, true
		for _, goal := range prepared {
			cheapest := math.Inf(1)
			for _, sub := range goal.Substrings {
				cheapest = math.Min(cheapest, StringCost(sub, bindings, weights))
			}
			importance, ok := weights.Crafts[goal.Item]
			if !ok {
				importance = 1
			}
			if math.IsInf(cheapest, 1) {
				total += missingPenalty
				feasible = false
			} else {
				total += importance * cheapest
			}
		}
		return total, feasible
	}
	for restart := 0; restart < options.Restarts; restart++ {
		bindings := make(Layout)
		for ch, key := range options.Fixed {
			bindings[ch] = key
		}
		permutation := rng.Perm(len(keys))
		for i, ch := range initial {
			bindings[ch] = keys[permutation[i]]
		}
		// Some starts fill spare keys with other characters to explore larger sets.
		if restart%2 == 1 {
			for _, index := range rng.Perm(len(movable)) {
				ch := movable[index]
				if _, assigned := bindings[ch]; assigned {
					continue
				}
				for _, key := range keys {
					used := false
					for _, occupied := range bindings {
						if occupied == key {
							used = true
							break
						}
					}
					if !used {
						bindings[ch] = key
						break
					}
				}
			}
		}
		current, _ := evaluate(bindings)
		for iteration := 0; iteration <= options.Iterations; iteration++ {
			cost, feasible := evaluate(bindings)
			if feasible && (cost < best.Cost || cost == best.Cost && len(bindings) < len(best.Bindings)) {
				best.Cost, best.Bindings = cost, make(Layout)
				for ch, key := range bindings {
					best.Bindings[ch] = key
				}
			}
			if iteration == options.Iterations || len(keys) == 0 {
				break
			}
			next := make(Layout, len(bindings)+1)
			for ch, key := range bindings {
				next[ch] = key
			}
			first := keys[rng.Intn(len(keys))]
			var occupant rune
			occupied := false
			for ch, key := range next {
				if key == first {
					occupant, occupied = ch, true
					break
				}
			}
			if rng.Intn(2) == 0 && len(keys) > 1 {
				second := keys[rng.Intn(len(keys))]
				for ch, key := range bindings {
					if key == first {
						next[ch] = second
					} else if key == second {
						next[ch] = first
					}
				}
			} else {
				if occupied {
					delete(next, occupant)
				}
				choice := rng.Intn(len(movable) + 1)
				if choice < len(movable) {
					ch := movable[choice]
					if _, assigned := next[ch]; assigned {
						continue
					}
					next[ch] = first
				}
			}
			nextCost, _ := evaluate(next)
			temperature := 2 * (1 - float64(iteration)/float64(options.Iterations))
			if nextCost <= current || temperature > 0 && rng.Float64() < math.Exp((current-nextCost)/temperature) {
				bindings, current = next, nextCost
			}
		}
	}
	used := make(map[rune]bool)
	for ch := range options.Fixed {
		used[ch] = true
	}
	for _, goal := range goals {
		choice := Goal{Item: goal.Item}
		cost := math.Inf(1)
		for _, sub := range goal.Substrings {
			if value := StringCost(sub, best.Bindings, weights); value < cost {
				cost = value
				choice.Substrings = []string{sub}
			}
		}
		best.Searches = append(best.Searches, choice)
		for _, sub := range choice.Substrings {
			for _, ch := range strings.ToLower(sub) {
				used[ch] = true
			}
		}
	}
	// Drop characters unused by the chosen cheapest searches; this preserves cost.
	for ch := range best.Bindings {
		if !used[ch] {
			delete(best.Bindings, ch)
		}
	}
	return best, nil
}

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
	Fixed             Layout          // characters that must stay on their supplied keys
	InitialCharacters []rune          // a feasible starting character set
	InitialBindings   map[string]rune // physical key -> typed character; first restart only, movable
}

type Optimized struct {
	Bindings   Layout
	Cost       float64
	Searches   []Goal      // one selected substring per craft, including search preferences
	GroupPlans []GroupPlan // contextual searches and editing actions for each group
}

// Optimize explores character sets and assignments using simulated annealing.
// It keeps every goal and uses all candidates. Results are heuristic, not a
// guarantee of the global minimum. Equal-cost results prefer fewer characters.
func Optimize(goals []Goal, weights Weights, options OptimizeOptions) (Optimized, error) {
	goals, err := WithPreferredSearches(goals, nil)
	if err != nil {
		return Optimized{}, err
	}
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
		if math.IsNaN(weight.X) || math.IsNaN(weight.Y) || math.IsInf(weight.X, 0) || math.IsInf(weight.Y, 0) {
			return Optimized{}, fmt.Errorf("key position for %q must be finite", key)
		}
		if weight.Effort < 0 || math.IsNaN(weight.Effort) || math.IsInf(weight.Effort, 0) {
			return Optimized{}, fmt.Errorf("invalid effort for %q", key)
		}
		if !fixedKeys[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	initialBindings, err := normalizeInitialBindings(options.InitialBindings, options.Fixed, weights)
	if err != nil {
		return Optimized{}, err
	}
	for _, penalty := range []float64{weights.SameFingerPenalty, weights.DefaultTransitionPenalty, weights.DistancePenalty, weights.FingerReusePenalty} {
		if penalty < 0 || math.IsNaN(penalty) || math.IsInf(penalty, 0) {
			return Optimized{}, fmt.Errorf("penalties must be finite and nonnegative")
		}
	}
	if weights.SearchEditing != nil {
		for _, cost := range []float64{weights.SearchEditing.ShiftHome, weights.SearchEditing.Backspace} {
			if cost < 0 || math.IsNaN(cost) || math.IsInf(cost, 0) {
				return Optimized{}, fmt.Errorf("search edit costs must be finite and nonnegative")
			}
		}
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

	for _, group := range weights.Groups {
		if group.Priority < 0 || math.IsNaN(group.Priority) || math.IsInf(group.Priority, 0) {
			return Optimized{}, fmt.Errorf("group %q priority must be finite and nonnegative", group.Name)
		}
	}
	// Precompute transition distances once. Explicit overrides still receive
	// the distance charge; the caller's weight maps are never modified.
	if weights.DistancePenalty > 0 {
		transitions := make(map[Transition]float64, len(weights.Keys)*len(weights.Keys))
		for from, a := range weights.Keys {
			for to, b := range weights.Keys {
				pair := Transition{From: from, To: to}
				penalty, ok := weights.Transitions[pair]
				if !ok {
					penalty = weights.DefaultTransitionPenalty
					if a.Finger != "" && a.Finger == b.Finger {
						penalty += weights.SameFingerPenalty
					}
				}
				transitions[pair] = penalty + weights.DistancePenalty*math.Hypot(a.X-b.X, a.Y-b.Y)
			}
		}
		weights.Transitions = transitions
		weights.DistancePenalty = 0
	}
	// With positive efforts, a longer search containing another candidate is
	// strictly more expensive among the allowed searches.
	// For zero-effort keys, only prune when the shorter search wins any cost tie.
	positiveEfforts := true
	for _, key := range weights.Keys {
		positiveEfforts = positiveEfforts && key.Effort > 0
	}
	prepared := make([]Goal, 0, len(goals))
	alphabet := ""
	for _, goal := range goals {
		if importance, ok := weights.Crafts[goal.Item]; ok && importance == 0 {
			continue
		}
		if err := validatePreferredSearches(goal); err != nil {
			return Optimized{}, err
		}
		subs := append([]string(nil), goal.Substrings...)
		sort.SliceStable(subs, func(i, j int) bool { return len([]rune(subs[i])) < len([]rune(subs[j])) })
		filtered := Goal{Item: goal.Item} // goals have already been restricted
		for _, sub := range subs {
			if sub == "" {
				continue
			}
			lower, redundant := strings.ToLower(sub), false
			for _, kept := range filtered.Substrings {
				if strings.Contains(lower, strings.ToLower(kept)) &&
					(kept <= sub || positiveEfforts && len([]rune(lower)) > len([]rune(strings.ToLower(kept)))) {
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
	sequence := newSequenceScorer(goals, weights)
	var fullAlphabet strings.Builder
	fullAlphabet.WriteString(alphabet)
	for _, text := range sequence.texts {
		fullAlphabet.WriteString(text)
	}
	for ch := range initialBindings {
		fullAlphabet.WriteRune(ch)
	}
	chars := []rune(characterKey(fullAlphabet.String()))
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
	for ch := range initialBindings {
		seed = append(append([]rune(nil), seed...), ch)
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
	if cost, _ := score(base, prepared, weights, math.Inf(1)); math.IsInf(cost, 1) {
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
	missingPenalty := 1 + importanceSum*(float64(maxLength)*(maxEffort+maxTransition)+
		weights.FingerReusePenalty*float64(maxLength)*float64(max(0, maxLength-1))/2)
	// Bound group costs too, so losing a goal is never rewarded by dropping
	// its proximity edges during exploration.
	maxDistance := 0.0
	for _, a := range weights.Keys {
		for _, b := range weights.Keys {
			maxDistance = math.Max(maxDistance, math.Hypot(a.X-b.X, a.Y-b.Y))
		}
	}
	for _, group := range weights.Groups {
		missingPenalty += group.Priority * float64(max(0, len(group.Crafts)-1)) * maxDistance
		if weights.SearchEditing != nil {
			missingPenalty += group.Priority * float64(len(group.Crafts)) * (float64(maxLength)*(maxEffort+maxTransition) + weights.FingerReusePenalty*float64(maxLength)*float64(max(0, maxLength-1))/2 + weights.SearchEditing.ShiftHome)
		}
	}
	evaluate := func(bindings Layout) (float64, bool) {
		total, feasible := score(bindings, prepared, weights, missingPenalty)
		editing, _ := sequence.evaluate(bindings, false)
		if math.IsInf(editing, 1) {
			editing = missingPenalty
			feasible = false
		}
		return total + editing, feasible
	}
	for restart := 0; restart < options.Restarts; restart++ {
		preferred := Layout(nil)
		if restart == 0 {
			preferred = initialBindings
		}
		bindings := startingLayout(initial, keys, options.Fixed, preferred, rng)
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
		current, currentFeasible := evaluate(bindings)
		for iteration := 0; iteration <= options.Iterations; iteration++ {
			cost, feasible := current, currentFeasible
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
			nextCost, nextFeasible := evaluate(next)
			temperature := 2 * (1 - float64(iteration)/float64(options.Iterations))
			if nextCost <= current || temperature > 0 && rng.Float64() < math.Exp((current-nextCost)/temperature) {
				bindings, current, currentFeasible = next, nextCost, nextFeasible
			}
		}
	}
	_, best.GroupPlans = sequence.evaluate(best.Bindings, true)
	used := make(map[rune]bool)
	for ch := range options.Fixed {
		used[ch] = true
	}
	for _, goal := range goals {
		choice := Goal{Item: goal.Item}
		sub, cost := cheapestSearch(goal, best.Bindings, weights)
		if !math.IsInf(cost, 1) {
			choice.Substrings = []string{sub}
		}
		best.Searches = append(best.Searches, choice)
		for _, sub := range choice.Substrings {
			for _, ch := range strings.ToLower(sub) {
				used[ch] = true
			}
		}
	}
	for _, plan := range best.GroupPlans {
		for _, step := range plan.Steps {
			for _, ch := range strings.ToLower(step.Search) {
				used[ch] = true
			}
		}
	}
	// Drop characters unused by standalone searches and group plans.
	for ch := range best.Bindings {
		if !used[ch] {
			delete(best.Bindings, ch)
		}
	}
	return best, nil
}

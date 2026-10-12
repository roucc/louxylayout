package layout

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
)

// OptimizePriority constructs important crafts first, then performs staged
// greedy local search. No temperature or acceptance of worse moves is used.
func OptimizePriority(goals []Goal, weights Weights, options OptimizeOptions) (Optimized, error) {
	goals, err := WithPreferredSearches(goals, nil)
	if err != nil {
		return Optimized{}, err
	}
	if err := ValidateWeights(weights); err != nil {
		return Optimized{}, err
	}
	prepared, err := preparedLayerGoals(goals, weights)
	if err != nil {
		return Optimized{}, err
	}
	ordered := append([]Goal(nil), prepared...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return craftImportance(ordered[i].Item, weights) > craftImportance(ordered[j].Item, weights)
	})
	layers := 1
	if options.EnableShiftLayer {
		layers = 2
	}
	fixed := [2]Layout{options.Fixed, options.FixedShift}
	if layers == 2 && fixed[1] == nil {
		fixed[1] = Layout{}
		if key, ok := fixed[0][' ']; ok {
			fixed[1][' '] = key
		}
	}
	var keys [2][]string
	var preferred [2]Layout
	initial := [2]map[string]rune{options.InitialBindings, options.InitialShiftBindings}
	for layer := 0; layer < layers; layer++ {
		keys[layer], err = validateFixedLayer(fixed[layer], weights)
		if err != nil {
			return Optimized{}, err
		}
		preferred[layer], err = normalizeInitialBindings(initial[layer], fixed[layer], weights)
		if err != nil {
			return Optimized{}, err
		}
	}
	fullScorer := priorityScorer(goals, weights, layers)
	if fullScorer.orderError != nil {
		return Optimized{}, fullScorer.orderError
	}
	if fullScorer.invalid {
		return Optimized{}, fmt.Errorf("groups support at most %d active unique crafts", maxGroupCrafts)
	}
	seed := options.InitialCharacters
	if len(seed) == 0 && len(prepared) > 0 {
		minimum, err := MinimumCharacters(prepared)
		if err != nil {
			return Optimized{}, err
		}
		seed = minimum.Characters
	}
	alphabet := ""
	for _, goal := range goals {
		for _, text := range goal.Substrings {
			alphabet += text
		}
	}
	for layer := 0; layer < layers; layer++ {
		for ch := range preferred[layer] {
			alphabet += string(ch)
		}
	}
	for ch, penalty := range weights.PreferBothLayers {
		if penalty > 0 {
			alphabet += string(ch)
		}
	}
	chars := []rune(characterKey(alphabet))
	if options.Restarts <= 0 {
		options.Restarts = 8
	}
	if options.Iterations <= 0 {
		options.Iterations = 4000
	}
	rng := rand.New(rand.NewSource(options.Seed))
	fullScore := func(b LayeredLayout) float64 { return priorityScore(b, prepared, weights, fullScorer, layers) }
	best := Optimized{Cost: math.Inf(1)}
	save := func(b LayeredLayout) {
		cost := fullScore(b)
		size := len(b.NonShift) + len(b.Shift)
		if cost < best.Cost || cost == best.Cost && size < len(best.Bindings)+len(best.ShiftBindings) {
			best.Cost = cost
			best.Bindings = copyLayout(b.NonShift)
			if layers == 2 {
				best.ShiftBindings = copyLayout(b.Shift)
			}
		}
	}
	// Each tier introduces all crafts at that priority; the final phase uses the
	// entire score, including the original group searches and editing plans.
	var tiers [][]Goal
	for i, goal := range ordered {
		if i == 0 || craftImportance(goal.Item, weights) != craftImportance(ordered[i-1].Item, weights) {
			tiers = append(tiers, append([]Goal(nil), ordered[:i+1]...))
		} else {
			tiers[len(tiers)-1] = append([]Goal(nil), ordered[:i+1]...)
		}
	}
	for restart := 0; restart < options.Restarts; restart++ {
		starts := [2]Layout{}
		if restart == 0 {
			starts = preferred
		}
		var current LayeredLayout
		if layers == 2 {
			current, err = makeLayerSeed(prepared, weights, seed, fixed, starts, keys, rng)
		} else {
			initialChars := []rune(characterKey(string(seed)))
			for ch := range starts[0] {
				initialChars = append(initialChars, ch)
			}
			movable := []rune{}
			for _, ch := range []rune(characterKey(string(initialChars))) {
				if _, locked := fixed[0][ch]; !locked {
					movable = append(movable, ch)
				}
			}
			if len(movable) > len(keys[0]) {
				return Optimized{}, fmt.Errorf("starting set needs %d assignable keys, have %d", len(movable), len(keys[0]))
			}
			current = LayeredLayout{NonShift: startingLayout(movable, keys[0], fixed[0], starts[0], rng)}
		}
		if err != nil {
			return Optimized{}, err
		}
		if !priorityFeasible(current, prepared, layers) {
			return Optimized{}, fmt.Errorf("starting layout cannot type every craft")
		}
		current = constructPriority(current, ordered, weights, fixed, starts, keys, layers, restart, rng)
		save(current)
		stageBudget := options.Iterations / 2
		for i, tier := range tiers {
			budget := stageBudget / len(tiers)
			if i < stageBudget%len(tiers) {
				budget++
			}
			scorer := priorityScorer(tier, weights, layers)
			objective := func(b LayeredLayout) float64 { return priorityScore(b, tier, weights, scorer, layers) }
			current = priorityClimb(current, prepared, chars, keys, fixed, layers, budget, rng, objective, save)
		}
		current = priorityClimb(current, prepared, chars, keys, fixed, layers, options.Iterations-stageBudget, rng, fullScore, save)
		save(current)
	}
	if math.IsInf(best.Cost, 1) {
		return Optimized{}, fmt.Errorf("no feasible layout found")
	}
	b := LayeredLayout{NonShift: best.Bindings, Shift: best.ShiftBindings}
	if layers == 2 {
		return finishLayered(best, goals, weights, fixed, fullScorer), nil
	}
	_, best.GroupPlans = fullScorer.evaluate(b.NonShift, true)
	used := map[rune]bool{}
	for ch := range fixed[0] {
		used[ch] = true
	}
	for _, goal := range goals {
		sub, _ := cheapestSearch(goal, b.NonShift, weights)
		choice := Goal{Item: goal.Item}
		if sub != "" {
			choice.Substrings = []string{sub}
		}
		best.Searches = append(best.Searches, choice)
		for _, ch := range strings.ToLower(sub) {
			used[ch] = true
		}
	}
	for _, plan := range best.GroupPlans {
		for _, step := range plan.Steps {
			for _, ch := range strings.ToLower(step.Search) {
				used[ch] = true
			}
		}
	}
	for ch := range best.Bindings {
		if !used[ch] {
			delete(best.Bindings, ch)
		}
	}
	return best, nil
}

func priorityScorer(goals []Goal, w Weights, layers int) *sequenceScorer {
	if layers == 2 {
		return newLayerSequenceScorer(goals, w)
	}
	return newSequenceScorer(goals, w)
}
func priorityScore(b LayeredLayout, goals []Goal, w Weights, s *sequenceScorer, layers int) float64 {
	if layers == 2 {
		cost, _, _ := layeredScore(b, goals, w, math.Inf(1))
		editing, _ := s.evaluateLayers(b, false)
		return cost + editing
	}
	cost, _ := score(b.NonShift, goals, w, math.Inf(1))
	editing, _ := s.evaluate(b.NonShift, false)
	return cost + editing
}
func priorityFeasible(b LayeredLayout, goals []Goal, layers int) bool {
	for _, goal := range goals {
		found := false
		for layer := 0; layer < layers && !found; layer++ {
			for _, sub := range goal.Substrings {
				valid := sub != ""
				for _, ch := range strings.ToLower(sub) {
					if _, ok := b.Bindings(layerName(layer))[ch]; !ok {
						valid = false
						break
					}
				}
				if valid {
					found = true
					break
				}
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// Repack a feasible character set by descending craft priority. Lower-priority
// characters reserve capacity but are assigned only after important searches.
func constructPriority(seed LayeredLayout, goals []Goal, w Weights, fixed, preferred [2]Layout, keys [2][]string, layers, restart int, rng *rand.Rand) LayeredLayout {
	result := LayeredLayout{NonShift: copyLayout(fixed[0])}
	if layers == 2 {
		result.Shift = copyLayout(fixed[1])
	}
	for layer := 0; layer < layers; layer++ {
		for ch, key := range preferred[layer] {
			result.Bindings(layerName(layer))[ch] = key
		}
	}
	orderedKeys := keys
	for layer := 0; layer < layers; layer++ {
		orderedKeys[layer] = append([]string(nil), keys[layer]...)
		rank := map[string]float64{}
		for _, key := range orderedKeys[layer] {
			rank[key] = w.Keys[key].Effort
			if restart > 0 {
				rank[key] += rng.Float64() * .75
			}
		}
		sort.SliceStable(orderedKeys[layer], func(i, j int) bool { return rank[orderedKeys[layer][i]] < rank[orderedKeys[layer][j]] })
	}
	place := func(layer int, text string) {
		assigned := result.Bindings(layerName(layer))
		previous := ""
		fingers := map[string]int{}
		for _, ch := range strings.ToLower(text) {
			key, exists := assigned[ch]
			if !exists {
				best, bestKey := math.Inf(1), ""
				for _, candidate := range orderedKeys[layer] {
					occupied := false
					for _, used := range assigned {
						if used == candidate {
							occupied = true
							break
						}
					}
					if occupied {
						continue
					}
					kw := w.Keys[candidate]
					cost := kw.Effort
					if kw.Finger != "" {
						cost += w.FingerReusePenalty * float64(fingers[kw.Finger])
					}
					if previous != "" {
						cost += keyPairCost(previous, candidate, w)
					}
					if cost < best {
						best, bestKey = cost, candidate
					}
				}
				key = bestKey
				assigned[ch] = key
			}
			if finger := w.Keys[key].Finger; finger != "" {
				fingers[finger]++
			}
			previous = key
		}
	}
	for _, goal := range goals {
		var choice LayeredSearch
		if layers == 2 {
			choice = cheapestLayeredSearch(goal, seed, w)
		} else {
			sub, cost := cheapestSearch(goal, seed.NonShift, w)
			choice = LayeredSearch{Search: sub, Cost: cost, Layer: NonShiftLayer}
		}
		layer := 0
		if choice.Layer == ShiftLayer {
			layer = 1
		}
		place(layer, choice.Search)
	}
	for layer := 0; layer < layers; layer++ {
		remaining := ""
		for ch := range seed.Bindings(layerName(layer)) {
			remaining += string(ch)
		}
		place(layer, characterKey(remaining))
	}
	return result
}

// Sweep swaps, empty-key moves, character replacements and deletions. Every
// accepted move is feasible for all goals and improves the active stage score.
func priorityClimb(current LayeredLayout, all []Goal, chars []rune, keys [2][]string, fixed [2]Layout, layers, budget int, rng *rand.Rand, objective func(LayeredLayout) float64, save func(LayeredLayout)) LayeredLayout {
	cost := objective(current)
	for budget > 0 {
		improved := false
		try := func(next LayeredLayout) {
			if budget <= 0 {
				return
			}
			budget--
			if !priorityFeasible(next, all, layers) {
				return
			}
			value := objective(next)
			if value < cost || value == cost && len(next.NonShift)+len(next.Shift) < len(current.NonShift)+len(current.Shift) {
				current, cost = next, value
				improved = true
				save(current)
			}
		}
		for _, layer := range rng.Perm(layers) {
			for _, i := range rng.Perm(len(keys[layer])) {
				first := keys[layer][i]
				for _, j := range rng.Perm(len(keys[layer])) {
					if j == i {
						continue
					}
					next := copyLayered(current)
					bindings := next.Bindings(layerName(layer))
					second := keys[layer][j]
					for ch, key := range bindings {
						if key == first {
							bindings[ch] = second
						} else if key == second {
							bindings[ch] = first
						}
					}
					try(next)
					if budget == 0 {
						return current
					}
				}
				for _, index := range rng.Perm(len(chars) + 1) {
					next := copyLayered(current)
					bindings := next.Bindings(layerName(layer))
					if index < len(chars) {
						if _, locked := fixed[layer][chars[index]]; locked {
							continue
						}
					}
					for ch, key := range bindings {
						if key == first {
							delete(bindings, ch)
						}
					}
					if index < len(chars) {
						bindings[chars[index]] = first
					}
					try(next)
					if budget == 0 {
						return current
					}
				}
			}
		}
		if !improved {
			break
		}
	}
	return current
}

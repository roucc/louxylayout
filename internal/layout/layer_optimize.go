package layout

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"unicode"
)

func validWeight(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

// ValidateWeights checks keyboard costs, layer preferences and group order rules.
func ValidateWeights(weights Weights) error { return validateLayerWeights(weights) }

func validateLayerWeights(weights Weights) error {
	for key, weight := range weights.Keys {
		if !validWeight(weight.Effort) {
			return fmt.Errorf("invalid effort for %q", key)
		}
		if math.IsNaN(weight.X) || math.IsNaN(weight.Y) || math.IsInf(weight.X, 0) || math.IsInf(weight.Y, 0) {
			return fmt.Errorf("key position for %q must be finite", key)
		}
	}
	for _, penalty := range []float64{weights.SameFingerPenalty, weights.DefaultTransitionPenalty, weights.DistancePenalty, weights.FingerReusePenalty, weights.LayerPreferencePenalty, weights.LayerSwitchPenalty} {
		if !validWeight(penalty) {
			return fmt.Errorf("penalties must be finite and nonnegative")
		}
	}
	for _, penalty := range weights.Transitions {
		if !validWeight(penalty) {
			return fmt.Errorf("transition costs must be finite and nonnegative")
		}
	}
	for _, importance := range weights.Crafts {
		if !validWeight(importance) {
			return fmt.Errorf("craft weights must be finite and nonnegative")
		}
	}
	for item, layer := range weights.PreferredLayers {
		if layer != AnyLayer && layer != NonShiftLayer && layer != ShiftLayer {
			return fmt.Errorf("invalid preferred layer %q for %q", layer, item)
		}
	}
	for ch, penalty := range weights.PreferBothLayers {
		if !validWeight(penalty) || unicode.ToLower(ch) != ch {
			return fmt.Errorf("shared character preferences must be lowercase with finite nonnegative penalties")
		}
	}
	for _, group := range weights.Groups {
		if err := validateGroupOrder(group); err != nil {
			return err
		}
		if !validWeight(group.Priority) {
			return fmt.Errorf("group %q priority must be finite and nonnegative", group.Name)
		}
	}
	if weights.SearchEditing != nil {
		if !validWeight(weights.SearchEditing.ShiftHome) || !validWeight(weights.SearchEditing.Backspace) {
			return fmt.Errorf("search edit costs must be finite and nonnegative")
		}
	}
	return nil
}

func validateFixedLayer(fixed Layout, weights Weights) ([]string, error) {
	used := make(map[string]bool)
	for ch, key := range fixed {
		if _, ok := weights.Keys[key]; !ok {
			return nil, fmt.Errorf("fixed key %q has no weights", key)
		}
		if used[key] || unicode.ToLower(ch) != ch {
			return nil, fmt.Errorf("fixed bindings must use distinct keys and lowercase characters")
		}
		used[key] = true
	}
	var keys []string
	for key := range weights.Keys {
		if !used[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys, nil
}

func copyLayout(bindings Layout) Layout {
	result := make(Layout, len(bindings))
	for ch, key := range bindings {
		result[ch] = key
	}
	return result
}

func copyLayered(bindings LayeredLayout) LayeredLayout {
	return LayeredLayout{NonShift: copyLayout(bindings.NonShift), Shift: copyLayout(bindings.Shift)}
}

func preparedLayerGoals(goals []Goal, weights Weights) ([]Goal, error) {
	positive := true
	for _, weight := range weights.Keys {
		positive = positive && weight.Effort > 0
	}
	var result []Goal
	for _, goal := range goals {
		if craftImportance(goal.Item, weights) == 0 {
			continue
		}
		subs := append([]string(nil), goal.Substrings...)
		sort.SliceStable(subs, func(i, j int) bool { return len([]rune(subs[i])) < len([]rune(subs[j])) })
		filtered := Goal{Item: goal.Item}
		for _, sub := range subs {
			if sub == "" {
				continue
			}
			redundant := false
			for _, kept := range filtered.Substrings {
				if strings.Contains(strings.ToLower(sub), strings.ToLower(kept)) && (kept <= sub || positive && len([]rune(sub)) > len([]rune(kept))) {
					redundant = true
					break
				}
			}
			if !redundant {
				filtered.Substrings = append(filtered.Substrings, sub)
			}
		}
		if len(filtered.Substrings) == 0 {
			return nil, fmt.Errorf("goal %q has no candidates", goal.Item)
		}
		result = append(result, filtered)
	}
	return result, nil
}

// makeLayerSeed duplicates the feasible starting set when it fits both layers.
// Otherwise it places whole searches greedily across the two layers, allowing
// a combined character set larger than the number of physical keys.
func makeLayerSeed(goals []Goal, weights Weights, seed []rune, fixed [2]Layout, preferred [2]Layout, keys [2][]string, rng *rand.Rand) (LayeredLayout, error) {
	assigned := [2]Layout{copyLayout(fixed[0]), copyLayout(fixed[1])}
	available := [2][]string{}
	for layer := 0; layer < 2; layer++ {
		used := make(map[string]bool)
		for ch, key := range preferred[layer] {
			assigned[layer][ch] = key
		}
		for _, key := range assigned[layer] {
			used[key] = true
		}
		for _, i := range rng.Perm(len(keys[layer])) {
			key := keys[layer][i]
			if !used[key] {
				available[layer] = append(available[layer], key)
			}
		}
	}
	assign := func(layer int, chars []rune) {
		for _, ch := range chars {
			if _, ok := assigned[layer][ch]; ok {
				continue
			}
			key := available[layer][0]
			available[layer] = available[layer][1:]
			assigned[layer][ch] = key
		}
	}
	additional := func(layer int, chars []rune) int {
		count := 0
		for _, ch := range chars {
			if _, ok := assigned[layer][ch]; !ok {
				count++
			}
		}
		return count
	}
	chars := []rune(characterKey(string(seed)))
	if additional(0, chars) <= len(available[0]) && additional(1, chars) <= len(available[1]) {
		assign(0, chars)
		assign(1, chars)
	} else {
		ordered := append([]Goal(nil), goals...)
		sort.SliceStable(ordered, func(i, j int) bool {
			return craftImportance(ordered[i].Item, weights) > craftImportance(ordered[j].Item, weights)
		})
		for _, goal := range ordered {
			best, bestLayer, bestChars := math.Inf(1), -1, []rune(nil)
			for layer := 0; layer < 2; layer++ {
				for _, sub := range goal.Substrings {
					chars := []rune(characterKey(sub))
					count := additional(layer, chars)
					if count > len(available[layer]) {
						continue
					}
					cost := float64(count) + layerPenalty(goal.Item, layerName(layer), weights)
					if cost < best {
						best, bestLayer, bestChars = cost, layer, chars
					}
				}
			}
			if bestLayer < 0 {
				return LayeredLayout{}, fmt.Errorf("cannot seed complete search for %q on either layer; reduce initial bindings or supply more keys", goal.Item)
			}
			assign(bestLayer, bestChars)
		}
	}
	shared := make([]rune, 0, len(weights.PreferBothLayers))
	for ch, penalty := range weights.PreferBothLayers {
		if penalty > 0 {
			shared = append(shared, ch)
		}
	}
	sort.Slice(shared, func(i, j int) bool { return shared[i] < shared[j] })
	for _, ch := range shared {
		for layer := 0; layer < 2; layer++ {
			if _, ok := assigned[layer][ch]; !ok && len(available[layer]) > 0 {
				assign(layer, []rune{ch})
			}
		}
	}
	return LayeredLayout{NonShift: assigned[0], Shift: assigned[1]}, nil
}

func layeredMissingPenalty(goals []Goal, weights Weights) float64 {
	maxLength, maxEffort, maxTransition, maxDistance := 0, 0.0, weights.DefaultTransitionPenalty+weights.SameFingerPenalty, 0.0
	for _, goal := range goals {
		for _, sub := range goal.Substrings {
			maxLength = max(maxLength, len([]rune(sub)))
		}
	}
	for _, key := range weights.Keys {
		maxEffort = math.Max(maxEffort, key.Effort)
		for _, other := range weights.Keys {
			maxDistance = math.Max(maxDistance, math.Hypot(key.X-other.X, key.Y-other.Y))
		}
	}
	for _, cost := range weights.Transitions {
		maxTransition = math.Max(maxTransition, cost)
	}
	length := float64(maxLength)
	bound := length*(maxEffort+maxTransition+weights.DistancePenalty*maxDistance) + weights.FingerReusePenalty*length*float64(max(0, maxLength-1))/2 + weights.LayerPreferencePenalty + weights.LayerSwitchPenalty
	result := 1.0
	for _, goal := range goals {
		result += craftImportance(goal.Item, weights) * bound
	}
	for _, group := range weights.Groups {
		size := float64(len(uniqueGroupItems(group.Crafts)))
		editBound := bound
		if weights.SearchEditing != nil {
			editBound += weights.SearchEditing.ShiftHome + weights.SearchEditing.Backspace
		}
		maxImportance := 1.0
		for _, item := range group.Crafts {
			maxImportance = math.Max(maxImportance, craftImportance(item, weights))
		}
		result += group.Priority * size * (maxImportance*editBound + maxDistance)
	}
	for _, penalty := range weights.PreferBothLayers {
		result += 2 * penalty
	}
	return result
}

// OptimizeLayers explores independent assignments on both layers, including
// duplicate characters. Key efforts, fingers and rolls use the same weights.
func OptimizeLayers(goals []Goal, weights Weights, options OptimizeOptions) (Optimized, error) {
	goals, err := WithPreferredSearches(goals, nil)
	if err != nil {
		return Optimized{}, err
	}
	if err := validateLayerWeights(weights); err != nil {
		return Optimized{}, err
	}
	prepared, err := preparedLayerGoals(goals, weights)
	if err != nil {
		return Optimized{}, err
	}
	fixed := [2]Layout{options.Fixed, options.FixedShift}
	if fixed[1] == nil {
		fixed[1] = make(Layout)
		if key, ok := fixed[0][' ']; ok {
			fixed[1][' '] = key
		}
	}
	var keys [2][]string
	var preferred [2]Layout
	for layer := 0; layer < 2; layer++ {
		keys[layer], err = validateFixedLayer(fixed[layer], weights)
		if err != nil {
			return Optimized{}, err
		}
	}
	preferred[0], err = normalizeInitialBindings(options.InitialBindings, fixed[0], weights)
	if err != nil {
		return Optimized{}, err
	}
	preferred[1], err = normalizeInitialBindings(options.InitialShiftBindings, fixed[1], weights)
	if err != nil {
		return Optimized{}, err
	}
	sequence := newLayerSequenceScorer(goals, weights)
	if sequence.orderError != nil {
		return Optimized{}, sequence.orderError
	}
	if sequence.invalid {
		return Optimized{}, fmt.Errorf("groups support at most %d active unique crafts", maxGroupCrafts)
	}
	var alphabet strings.Builder
	for _, goal := range goals {
		for _, sub := range goal.Substrings {
			alphabet.WriteString(sub)
		}
	}
	for _, layer := range preferred {
		for ch := range layer {
			alphabet.WriteRune(ch)
		}
	}
	for ch, penalty := range weights.PreferBothLayers {
		if penalty > 0 {
			alphabet.WriteRune(ch)
		}
	}
	chars := []rune(characterKey(alphabet.String()))
	seed := options.InitialCharacters
	if len(seed) == 0 && len(prepared) > 0 {
		minimum, err := MinimumCharacters(prepared)
		if err != nil {
			return Optimized{}, err
		}
		seed = minimum.Characters
	}
	if options.Restarts <= 0 {
		options.Restarts = 8
	}
	if options.Iterations <= 0 {
		options.Iterations = 4000
	}
	rng := rand.New(rand.NewSource(options.Seed))
	missingPenalty := layeredMissingPenalty(goals, weights)
	evaluate := func(bindings LayeredLayout) (float64, bool) {
		total, feasible, _ := layeredScore(bindings, prepared, weights, missingPenalty)
		editing, _ := sequence.evaluateLayers(bindings, false)
		if math.IsInf(editing, 1) {
			editing = missingPenalty
			feasible = false
		}
		return total + editing, feasible
	}
	best := Optimized{Cost: math.Inf(1)}
	for restart := 0; restart < options.Restarts; restart++ {
		starting := [2]Layout{}
		if restart == 0 {
			starting = preferred
		}
		current, err := makeLayerSeed(prepared, weights, seed, fixed, starting, keys, rng)
		if err != nil {
			return Optimized{}, err
		}
		currentCost, currentFeasible := evaluate(current)
		if !currentFeasible {
			return Optimized{}, fmt.Errorf("starting layout cannot type every craft on either layer")
		}
		for iteration := 0; iteration <= options.Iterations; iteration++ {
			size := len(current.NonShift) + len(current.Shift)
			if currentFeasible && (currentCost < best.Cost || currentCost == best.Cost && size < len(best.Bindings)+len(best.ShiftBindings)) {
				best.Cost = currentCost
				best.Bindings, best.ShiftBindings = copyLayout(current.NonShift), copyLayout(current.Shift)
			}
			if iteration == options.Iterations || len(keys[0])+len(keys[1]) == 0 {
				break
			}
			layer := rng.Intn(2)
			if len(keys[layer]) == 0 {
				layer = 1 - layer
			}
			next := copyLayered(current)
			bindings := next.Bindings(layerName(layer))
			first := keys[layer][rng.Intn(len(keys[layer]))]
			if rng.Intn(2) == 0 && len(keys[layer]) > 1 {
				second := keys[layer][rng.Intn(len(keys[layer]))]
				for ch, key := range bindings {
					if key == first {
						bindings[ch] = second
					} else if key == second {
						bindings[ch] = first
					}
				}
			} else {
				choice := rng.Intn(len(chars) + 1)
				if choice < len(chars) {
					if _, locked := fixed[layer][chars[choice]]; locked {
						continue
					}
				}
				for ch, key := range bindings {
					if key == first {
						delete(bindings, ch)
					}
				}
				if choice < len(chars) {
					bindings[chars[choice]] = first
				}
			}
			cost, feasible := evaluate(next)
			temperature := 2 * (1 - float64(iteration)/float64(options.Iterations))
			if cost <= currentCost || temperature > 0 && rng.Float64() < math.Exp((currentCost-cost)/temperature) {
				current, currentCost, currentFeasible = next, cost, feasible
			}
		}
	}
	layered := LayeredLayout{NonShift: best.Bindings, Shift: best.ShiftBindings}
	_, _, best.LayeredSearches = layeredScore(layered, goals, weights, math.Inf(1))
	_, best.GroupPlans = sequence.evaluateLayers(layered, true)
	used := [2]map[rune]bool{make(map[rune]bool), make(map[rune]bool)}
	mark := func(layer CraftLayer, text string) {
		index := 0
		if layer == ShiftLayer {
			index = 1
		}
		for _, ch := range strings.ToLower(text) {
			used[index][ch] = true
		}
	}
	for layer := 0; layer < 2; layer++ {
		for ch := range fixed[layer] {
			used[layer][ch] = true
		}
	}
	for ch, penalty := range weights.PreferBothLayers {
		if penalty > 0 {
			used[0][ch], used[1][ch] = true, true
		}
	}
	for _, choice := range best.LayeredSearches {
		mark(choice.Layer, choice.Search)
		goal := Goal{Item: choice.Item}
		if choice.Search != "" {
			goal.Substrings = []string{choice.Search}
		}
		best.Searches = append(best.Searches, goal)
	}
	for _, plan := range best.GroupPlans {
		for _, step := range plan.Steps {
			mark(step.Layer, step.Search)
		}
	}
	for layer, bindings := range []Layout{best.Bindings, best.ShiftBindings} {
		for ch := range bindings {
			if !used[layer][ch] {
				delete(bindings, ch)
			}
		}
	}
	return best, nil
}

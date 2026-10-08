package layout

import (
	"math"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

func TestSharedPhysicalTransitionsAcrossLayers(t *testing.T) {
	weights := Weights{Keys: map[string]KeyWeight{
		"Q": {Effort: 1, Finger: "ring", X: 0}, "W": {Effort: 1, Finger: "middle", X: 1}, "E": {Effort: 1, Finger: "index", X: 2},
	}, Transitions: map[Transition]float64{{From: "Q", To: "W"}: .1, {From: "W", To: "E"}: .2}, DefaultTransitionPenalty: 3, DistancePenalty: .5, FingerReusePenalty: 1}
	layers := LayeredLayout{NonShift: Layout{'a': "Q", 'b': "W", 'c': "E"}, Shift: Layout{'x': "Q", 'y': "W", 'z': "E"}}
	normal := StringCost("abc", layers.NonShift, weights)
	shifted := StringCost("xyz", layers.Shift, weights)
	if math.Abs(normal-4.3) > 1e-9 || normal != shifted {
		t.Fatalf("same QWE roll must have identical typing costs: normal=%v shift=%v", normal, shifted)
	}
}

func TestLayerPreferencesAreSoftAndAnyLayerIsDefault(t *testing.T) {
	weights := Weights{Keys: map[string]KeyWeight{"A": {Effort: 1}, "G": {Effort: 20}}, PreferredLayers: map[string]CraftLayer{"bed": ShiftLayer}, LayerPreferencePenalty: 5, LayerSwitchPenalty: .5}
	goal := Goal{Item: "bed", Substrings: []string{"n"}}
	bindings := LayeredLayout{NonShift: Layout{'n': "A"}, Shift: Layout{'n': "G"}}
	choice := cheapestLayeredSearch(goal, bindings, weights)
	if choice.Layer != NonShiftLayer || choice.Cost != 6 {
		t.Fatalf("wrong layer should remain usable when much easier: %+v", choice)
	}
	bindings.Shift['n'] = "A"
	choice = cheapestLayeredSearch(goal, bindings, weights)
	if choice.Layer != ShiftLayer || choice.Cost != 1.5 {
		t.Fatalf("preferred shift layer should win: %+v", choice)
	}
	goal.Item = "unlisted"
	choice = cheapestLayeredSearch(goal, bindings, weights)
	if choice.Layer != NonShiftLayer || choice.Cost != 1 {
		t.Fatalf("unlisted craft should have no mismatch cost: %+v", choice)
	}
	weights.PreferredLayers[goal.Item] = AnyLayer
	if explicit := cheapestLayeredSearch(goal, bindings, weights); !reflect.DeepEqual(choice, explicit) {
		t.Fatal("AnyLayer must match omitted preference")
	}
	weights.Keys["G"] = KeyWeight{Effort: 10}
	bindings.NonShift['n'] = "G"
	choice = cheapestLayeredSearch(goal, bindings, weights)
	if choice.Layer != ShiftLayer {
		t.Fatalf("AnyLayer must allow shifted crafts: %+v", choice)
	}
}

func TestOptimizeDuplicatesCharacterOnBothLayers(t *testing.T) {
	weights := Weights{Keys: map[string]KeyWeight{"A": {Effort: 1}}, PreferredLayers: map[string]CraftLayer{"bed": ShiftLayer, "sticks": NonShiftLayer}, LayerPreferencePenalty: 5, PreferBothLayers: map[rune]float64{'n': 5}}
	goals := []Goal{{Item: "bed", Substrings: []string{"n"}}, {Item: "sticks", Substrings: []string{"n"}}}
	options := OptimizeOptions{EnableShiftLayer: true, Seed: 42, Restarts: 1, Iterations: 30}
	got, err := Optimize(goals, weights, options)
	if err != nil {
		t.Fatal(err)
	}
	if got.Bindings['n'] != "A" || got.ShiftBindings['n'] != "A" || got.Cost != 2 {
		t.Fatalf("n should be available on both layers: %+v", got)
	}
	if got.LayeredSearches[0].Layer != ShiftLayer || got.LayeredSearches[1].Layer != NonShiftLayer {
		t.Fatalf("craft layer preferences ignored: %+v", got.LayeredSearches)
	}
	again, err := Optimize(goals, weights, options)
	if err != nil || !reflect.DeepEqual(got, again) {
		t.Fatalf("seed not reproducible: %+v, %v", again, err)
	}
}

func TestBothLayerAvailabilityIsSoft(t *testing.T) {
	weights := Weights{Keys: map[string]KeyWeight{"A": {Effort: 1}}, PreferBothLayers: map[rune]float64{'n': 5}}
	goals := []Goal{{Item: "one", Substrings: []string{"x"}}}
	got, err := OptimizeLayers(goals, weights, OptimizeOptions{Seed: 42, Restarts: 2, Iterations: 100})
	if err != nil {
		t.Fatal(err)
	}
	layers := LayeredLayout{NonShift: got.Bindings, Shift: got.ShiftBindings}
	if got.Cost != 6 || got.Cost != LayeredCost(layers, goals, weights) {
		t.Fatalf("shared preference must not make this infeasible: %+v", got)
	}
	if _, normal := got.Bindings['n']; !normal {
		if _, shifted := got.ShiftBindings['n']; !shifted {
			t.Fatalf("one layer should retain n: %+v", got)
		}
	}
}

func TestLayeredCapacityAndFixedSpace(t *testing.T) {
	weights := Weights{Keys: map[string]KeyWeight{"A": {Effort: 1}, "B": {Effort: 1}, "Space": {Effort: 1}}}
	goals := []Goal{{Item: "one", Substrings: []string{"a "}}, {Item: "two", Substrings: []string{"b "}}, {Item: "three", Substrings: []string{"c "}}}
	got, err := OptimizeLayers(goals, weights, OptimizeOptions{Fixed: Layout{' ': "Space"}, Seed: 42, Restarts: 2, Iterations: 50})
	if err != nil {
		t.Fatal(err)
	}
	if got.Bindings[' '] != "Space" || got.ShiftBindings[' '] != "Space" {
		t.Fatalf("Space must stay fixed on both layers: %+v", got)
	}
	if got.Cost != 6 {
		t.Fatalf("more than one layer's character capacity should be feasible: %+v", got)
	}
	assertLayeredBindings(t, got, weights)
}

func TestLayeredInitialBindingsAndValidation(t *testing.T) {
	weights := Weights{Keys: map[string]KeyWeight{"A": {Effort: 1}, "B": {Effort: 1}, "Space": {Effort: 1}}}
	fixed := [2]Layout{{' ': "Space"}, {' ': "Space"}}
	preferred := [2]Layout{{'n': "A"}, {'n': "B"}}
	keys := [2][]string{{"A", "B"}, {"A", "B"}}
	// Check initialization separately so equal-cost optimizer mutations cannot
	// obscure whether the supplied positions were actually used.
	rng := rand.New(rand.NewSource(42))
	seed, err := makeLayerSeed([]Goal{{Item: "one", Substrings: []string{"n "}}}, weights, []rune("n "), fixed, preferred, keys, rng)
	if err != nil || seed.NonShift['n'] != "A" || seed.Shift['n'] != "B" {
		t.Fatalf("layer-specific seed mismatch: %+v, %v", seed, err)
	}
	for _, invalid := range []Weights{
		{LayerPreferencePenalty: -1}, {LayerSwitchPenalty: math.NaN()}, {PreferredLayers: map[string]CraftLayer{"bed": "invalid"}}, {PreferBothLayers: map[rune]float64{'N': 5}},
	} {
		if err := validateLayerWeights(invalid); err == nil {
			t.Fatalf("accepted invalid layer configuration: %+v", invalid)
		}
	}
	if _, err := OptimizeLayers(nil, weights, OptimizeOptions{InitialShiftBindings: map[string]rune{"Missing": 'n'}}); err == nil {
		t.Fatal("unknown shifted physical key should fail")
	}
	if _, err := OptimizeLayers(nil, weights, OptimizeOptions{FixedShift: Layout{'N': "A"}}); err == nil {
		t.Fatal("uppercase shifted fixed character should fail")
	}
}

func TestLayeredOverlapSwitchAndReportedCost(t *testing.T) {
	goals := []Goal{{Item: "bed", Substrings: []string{"l "}}, {Item: "pick", Substrings: []string{"lha"}}}
	normal, weights := sequenceFixture(goals)
	shifted := copyLayout(normal)
	weights.PreferredLayers = map[string]CraftLayer{"bed": ShiftLayer, "pick": NonShiftLayer}
	weights.LayerPreferencePenalty = 50
	weights.LayerSwitchPenalty = .5
	layers := LayeredLayout{NonShift: normal, Shift: shifted}
	plans, err := PlanLayeredGroups(layers, goals, weights)
	if err != nil {
		t.Fatal(err)
	}
	bed, pick := stepFor(plans[0], "bed"), stepFor(plans[0], "pick")
	if bed.Layer != ShiftLayer || pick.Layer != NonShiftLayer || pick.Action != "backspace" || pick.Backspaces != 1 || pick.Type != "ha" || !pick.LayerChange {
		t.Fatalf("expected layer switch with l SPACE, BS, ha: %+v", plans[0])
	}
	if plans[0].Cost != 5.5 {
		t.Fatalf("expected typing/edit cost 4.5 + two Shift changes: %+v", plans[0])
	}
	assertLayeredReplay(t, plans[0], layers)
	got, err := OptimizeLayers(goals, weights, OptimizeOptions{Fixed: normal, FixedShift: shifted, Restarts: 1, Iterations: 1})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(got.Cost-LayeredCost(LayeredLayout{NonShift: got.Bindings, Shift: got.ShiftBindings}, goals, weights)) > 1e-9 {
		t.Fatalf("layered cost mismatch: %+v", got)
	}
	if !reflect.DeepEqual(got.GroupPlans, plans) {
		t.Fatalf("optimizer plan mismatch: %+v", got.GroupPlans)
	}
}

func assertLayeredBindings(t *testing.T, got Optimized, weights Weights) {
	t.Helper()
	for _, bindings := range []Layout{got.Bindings, got.ShiftBindings} {
		seen := make(map[string]bool)
		for ch, key := range bindings {
			if seen[key] {
				t.Fatalf("two characters share one key in a layer: %q", key)
			}
			seen[key] = true
			if _, ok := weights.Keys[key]; !ok {
				t.Fatalf("unknown physical key for %q: %q", ch, key)
			}
		}
	}
}

func assertLayeredReplay(t *testing.T, plan GroupPlan, layers LayeredLayout) {
	t.Helper()
	text := ""
	layer := NonShiftLayer
	sum := 0.0
	for _, step := range plan.Steps {
		if step.LayerChange != (layer != step.Layer) {
			t.Fatalf("wrong Shift change marker: %+v", step)
		}
		layer = step.Layer
		if step.Backspaces > 1 {
			t.Fatalf("multiple Backspaces: %+v", step)
		}
		switch step.Action {
		case "shift-home":
			text = ""
		case "backspace":
			text = string([]rune(text)[:len([]rune(text))-step.Backspaces])
		}
		for _, ch := range step.Type {
			if _, ok := layers.Bindings(layer)[ch]; !ok {
				t.Fatalf("new text %q is unreachable on %s", step.Type, layer)
			}
		}
		text += step.Type
		if text != strings.ToLower(step.Search) {
			t.Fatalf("operations produced %q instead of %q", text, step.Search)
		}
		sum += step.Cost
	}
	if math.Abs(sum-plan.Cost) > 1e-9 {
		t.Fatalf("reported step costs %v != plan cost %v", sum, plan.Cost)
	}
}

func TestLayeredSequenceMatchesExhaustive(t *testing.T) {
	rng := rand.New(rand.NewSource(123))
	for iteration := 0; iteration < 20; iteration++ {
		goals := make([]Goal, 3)
		for i := range goals {
			goals[i].Item = string(rune('0' + i))
			for c := 0; c < 3; c++ {
				sub := ""
				for length := 1 + rng.Intn(4); length > 0; length-- {
					sub += string(rune('a' + rng.Intn(3)))
				}
				goals[i].Substrings = append(goals[i].Substrings, sub)
			}
		}
		normal, weights := sequenceFixture(goals)
		shifted := copyLayout(normal)
		keys := []string{"a", "b", "c"}
		for i, ch := range []rune("abc") {
			normal[ch] = keys[i]
			shifted[ch] = keys[(i+1)%3]
			weights.Keys[keys[i]] = KeyWeight{Effort: 1 + float64(i)/2, Finger: "index", X: float64(i)}
		}
		layers := LayeredLayout{NonShift: normal, Shift: shifted}
		weights.LayerSwitchPenalty = rng.Float64() * 2
		weights.LayerPreferencePenalty = rng.Float64() * 3
		weights.DistancePenalty = .3
		weights.FingerReusePenalty = .4
		weights.DefaultTransitionPenalty = .2
		weights.SearchEditing.ShiftHome = rng.Float64() * 2
		weights.SearchEditing.Backspace = rng.Float64()
		weights.PreferredLayers = make(map[string]CraftLayer)
		weights.Crafts = make(map[string]float64)
		for _, goal := range goals {
			weights.PreferredLayers[goal.Item] = []CraftLayer{AnyLayer, NonShiftLayer, ShiftLayer}[rng.Intn(3)]
			weights.Crafts[goal.Item] = float64(1 + rng.Intn(3))
		}
		plans, err := PlanLayeredGroups(layers, goals, weights)
		if err != nil {
			t.Fatal(err)
		}
		want := exhaustiveLayeredSequence(goals, layers, weights)
		if math.Abs(plans[0].Cost-want) > 1e-9 {
			t.Fatalf("layered DP %v != exhaustive %v, plan %+v", plans[0].Cost, want, plans[0])
		}
		assertLayeredReplay(t, plans[0], layers)
	}
}

func exhaustiveLayeredSequence(goals []Goal, layers LayeredLayout, weights Weights) float64 {
	var visit func(int, string, CraftLayer) float64
	visit = func(mask int, previous string, previousLayer CraftLayer) float64 {
		if mask == (1<<len(goals))-1 {
			return 0
		}
		best := math.Inf(1)
		for i, goal := range goals {
			if mask&(1<<i) != 0 {
				continue
			}
			for _, layer := range []CraftLayer{NonShiftLayer, ShiftLayer} {
				bindings := layers.Bindings(layer)
				for _, sub := range goal.Substrings {
					fullCost := StringCost(sub, bindings, weights)
					if math.IsInf(fullCost, 1) {
						continue
					}
					cost := fullCost
					if mask != 0 {
						cost += weights.SearchEditing.ShiftHome
						from, to := []rune(previous), []rune(sub)
						if len(from) == 1 {
							cost = math.Min(cost, weights.SearchEditing.Backspace+fullCost)
						}
						for k := 1; k <= min(len(from), len(to)); k++ {
							if from[k-1] != to[k-1] {
								break
							}
							if len(from)-k > 1 {
								continue
							}
							typed := 0.0
							if k < len(to) {
								typed = StringCost(string(to[k:]), bindings, weights)
							}
							cost = math.Min(cost, float64(len(from)-k)*weights.SearchEditing.Backspace+typed)
						}
					}
					if previousLayer != layer {
						cost += weights.LayerSwitchPenalty
					}
					cost += craftImportance(goal.Item, weights) * layerPenalty(goal.Item, layer, weights)
					best = math.Min(best, cost+visit(mask|(1<<i), sub, layer))
				}
			}
		}
		return best
	}
	return visit(0, "", NonShiftLayer)
}

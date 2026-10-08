package layout

import (
	"math"
	"reflect"
	"testing"
)

func TestOptimizeChangesCharactersAndPreservesSpace(t *testing.T) {
	weights := Weights{Keys: map[string]KeyWeight{
		"A": {Effort: 1}, "Space": {Effort: 1},
	}}
	goals := []Goal{{Item: "craft", Substrings: []string{"aa ", "b "}}}
	options := OptimizeOptions{
		Restarts: 3, Iterations: 300, Seed: 42,
		Fixed: Layout{' ': "Space"}, InitialCharacters: []rune("a "),
	}
	got, err := Optimize(goals, weights, options)
	if err != nil {
		t.Fatal(err)
	}
	if got.Bindings[' '] != "Space" || got.Bindings['b'] != "A" || got.Cost != 2 {
		t.Fatalf("expected replacement of a by b with fixed Space: %+v", got)
	}
	if _, retained := got.Bindings['a']; retained {
		t.Fatal("unused a should be removed")
	}
	if got.Cost != Cost(got.Bindings, goals, weights) || got.Searches[0].Substrings[0] != "b " {
		t.Fatalf("cost/search mismatch: %+v", got)
	}
	again, err := Optimize(goals, weights, options)
	if err != nil || !reflect.DeepEqual(got, again) {
		t.Fatalf("seed is not reproducible: %+v, %v", again, err)
	}
}

func TestOptimizeCanUseMoreThanMinimumCharacters(t *testing.T) {
	weights := Weights{Keys: map[string]KeyWeight{"A": {Effort: 1}, "S": {Effort: 1}}}
	goals := []Goal{
		{Item: "one", Substrings: []string{"aaa", "b"}},
		{Item: "two", Substrings: []string{"aaa", "c"}},
	}
	got, err := Optimize(goals, weights, OptimizeOptions{Seed: 9, Restarts: 4, Iterations: 500, InitialCharacters: []rune("a")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Cost != 2 || len(got.Bindings) != 2 {
		t.Fatalf("expected b and c instead of minimum set a: %+v", got)
	}
	if _, ok := got.Bindings['a']; ok {
		t.Fatal("minimum set must not restrict optimization")
	}
}

func TestOptimizeRejectsUnusableStartingSet(t *testing.T) {
	weights := Weights{Keys: map[string]KeyWeight{"A": {Effort: 1}}}
	if _, err := Optimize([]Goal{{Item: "missing"}}, weights, OptimizeOptions{}); err == nil {
		t.Fatal("missing candidates must fail")
	}
	if _, err := Optimize([]Goal{{Substrings: []string{"ab"}}}, weights, OptimizeOptions{}); err == nil {
		t.Fatal("insufficient physical keys must fail")
	}
}

func TestOptimizePullsGroupedCraftsTogether(t *testing.T) {
	weights := Weights{Keys: map[string]KeyWeight{
		"A": {Effort: 1, X: 0}, "S": {Effort: 1, X: 1}, "G": {Effort: 1, X: 5},
	}, Groups: []CraftGroup{{Priority: 5, Crafts: []string{"one", "two"}}}}
	goals := []Goal{{Item: "one", Substrings: []string{"a"}}, {Item: "two", Substrings: []string{"b"}}, {Item: "three", Substrings: []string{"c"}}}
	got, err := Optimize(goals, weights, OptimizeOptions{Seed: 42, Restarts: 2, Iterations: 300})
	if err != nil {
		t.Fatal(err)
	}
	if got.Cost != 8 || got.Cost != Cost(got.Bindings, goals, weights) {
		t.Fatalf("expected adjacent grouped keys and consistent cost: %+v", got)
	}
	if got.Bindings['c'] != "G" {
		t.Fatalf("ungrouped craft should use distant key: %+v", got.Bindings)
	}
	for _, priority := range []float64{-1, math.NaN(), math.Inf(1)} {
		weights.Groups[0].Priority = priority
		if _, err := Optimize(goals, weights, OptimizeOptions{}); err == nil {
			t.Fatalf("accepted invalid group priority %v", priority)
		}
	}
}

func TestOptimizeGroupPruningPreservesZeroEffortTies(t *testing.T) {
	weights := Weights{Keys: map[string]KeyWeight{
		"A": {X: 0}, "Z": {X: 2}, "B": {X: 4},
	}, Groups: []CraftGroup{{Priority: 1, Crafts: []string{"one", "two"}}}}
	goals := []Goal{{Item: "one", Substrings: []string{"z", "az"}}, {Item: "two", Substrings: []string{"b"}}}
	got, err := Optimize(goals, weights, OptimizeOptions{Fixed: Layout{'a': "A", 'z': "Z", 'b': "B"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Searches[0].Substrings[0] != "az" || got.Cost != 3 || got.Cost != Cost(got.Bindings, goals, weights) {
		t.Fatalf("pruning changed alphabetical tie choice or group distance: %+v", got)
	}
	weights.Keys["A"] = KeyWeight{Effort: 1, X: 0}
	weights.Keys["Z"] = KeyWeight{Effort: 1, X: 2}
	weights.Keys["B"] = KeyWeight{Effort: 1, X: 4}
	got, err = Optimize(goals, weights, OptimizeOptions{Fixed: Layout{'a': "A", 'z': "Z", 'b': "B"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Searches[0].Substrings[0] != "z" || got.Cost != Cost(got.Bindings, goals, weights) {
		t.Fatalf("positive-effort pruning changed cost: %+v", got)
	}
}

func TestOptimizeComfortPenaltiesMatchPublicCost(t *testing.T) {
	weights := Weights{Keys: map[string]KeyWeight{
		"A": {Effort: 1, Finger: "middle", X: 0},
		"B": {Effort: 1, Finger: "index", X: 1},
		"C": {Effort: 1, Finger: "middle", X: 5},
		"D": {Effort: 1, Finger: "ring", X: 2},
	}, DistancePenalty: 2, FingerReusePenalty: 3,
		Transitions: map[Transition]float64{{From: "A", To: "B"}: 0}}
	goals := []Goal{{Item: "pick", Substrings: []string{"abc", "abd"}}}
	fixed := Layout{'a': "A", 'b': "B", 'c': "C", 'd': "D"}
	got, err := Optimize(goals, weights, OptimizeOptions{Fixed: fixed})
	if err != nil {
		t.Fatal(err)
	}
	if got.Searches[0].Substrings[0] != "abd" || math.Abs(got.Cost-Cost(got.Bindings, goals, weights)) > 1e-9 {
		t.Fatalf("expected compact search using distinct fingers with consistent cost: %+v", got)
	}
	if len(weights.Transitions) != 1 || weights.Transitions[Transition{From: "A", To: "B"}] != 0 {
		t.Fatal("precomputing distances mutated caller's transitions")
	}
	for _, penalty := range []float64{-1, math.NaN(), math.Inf(1)} {
		invalid := weights
		invalid.DistancePenalty = penalty
		if _, err := Optimize(goals, invalid, OptimizeOptions{Fixed: fixed}); err == nil {
			t.Fatalf("accepted distance penalty %v", penalty)
		}
		invalid = weights
		invalid.FingerReusePenalty = penalty
		if _, err := Optimize(goals, invalid, OptimizeOptions{Fixed: fixed}); err == nil {
			t.Fatalf("accepted finger reuse penalty %v", penalty)
		}
	}
}

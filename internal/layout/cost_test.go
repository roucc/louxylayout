package layout

import (
	"math"
	"testing"
)

func TestDefaultTransitionPenalty(t *testing.T) {
	bindings := Layout{'a': "A", 's': "S", 'q': "Q"}
	weights := Weights{
		Keys: map[string]KeyWeight{
			"A": {Effort: 1, Finger: "pinky"},
			"S": {Effort: 1, Finger: "ring"},
			"Q": {Effort: 1, Finger: "pinky"},
		},
		DefaultTransitionPenalty: 0.5,
		SameFingerPenalty:        0.7,
		Transitions:              map[Transition]float64{{From: "A", To: "S"}: 0},
	}
	for _, tc := range []struct {
		sub  string
		want float64
	}{
		{"as", 2}, {"sa", 2.5}, {"aq", 3.2}, {"a", 1},
	} {
		if got := StringCost(tc.sub, bindings, weights); math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("%q cost = %v, want %v", tc.sub, got, tc.want)
		}
	}
}

func TestStringCost(t *testing.T) {
	bindings := Layout{'n': "A", 'ø': "Q", 'v': "S"}
	weights := Weights{
		Keys: map[string]KeyWeight{
			"A": {Effort: 1, Finger: "pinky"},
			"Q": {Effort: 2, Finger: "pinky"},
			"S": {Effort: 1, Finger: "ring"},
		},
		SameFingerPenalty: 0.5,
		Transitions:       map[Transition]float64{{From: "Q", To: "S"}: 0.25},
	}
	if got := StringCost("NØv", bindings, weights); got != 4.75 {
		t.Fatalf("cost = %v, want 4.75", got)
	}
	// Explicit zero overrides the default same-finger penalty.
	weights.Transitions[Transition{From: "A", To: "Q"}] = 0
	if got := StringCost("nø", bindings, weights); got != 3 {
		t.Fatalf("override cost = %v, want 3", got)
	}
	if got := StringCost("øn", bindings, weights); got != 3.5 {
		t.Fatalf("reverse transition cost = %v, want 3.5", got)
	}
	for _, sub := range []string{"", "?"} {
		if !math.IsInf(StringCost(sub, bindings, weights), 1) {
			t.Fatalf("%q must be untypeable", sub)
		}
	}
	delete(weights.Keys, "S")
	if !math.IsInf(StringCost("v", bindings, weights), 1) {
		t.Fatal("missing physical key weights must be untypeable")
	}
}

func TestCostReconsidersCandidatesAfterSwap(t *testing.T) {
	bindings := Layout{'a': "A", 'b': "Q"}
	weights := Weights{
		Keys:   map[string]KeyWeight{"A": {Effort: 1}, "Q": {Effort: 4}},
		Crafts: map[string]float64{"first": 2, "ignored": 0},
	}
	goals := []Goal{
		{Item: "first", Substrings: []string{"aa", "b"}},
		{Item: "second", Substrings: []string{"a"}},
		{Item: "ignored"},
	}
	if got := Cost(bindings, goals, weights); got != 5 {
		t.Fatalf("before swap = %v, want 5", got)
	}
	bindings['a'], bindings['b'] = bindings['b'], bindings['a']
	if got := Cost(bindings, goals, weights); got != 6 {
		t.Fatalf("after swap = %v, want 6 (first goal switches to b)", got)
	}
	if !math.IsInf(Cost(bindings, []Goal{{Item: "missing"}}, weights), 1) {
		t.Fatal("goal without candidates must cost infinity")
	}
}

func TestCraftGroupPriorityAndOrder(t *testing.T) {
	bindings := Layout{'a': "A", 'b': "B", 'c': "C"}
	weights := Weights{Keys: map[string]KeyWeight{
		"A": {Effort: 1, X: 0}, "B": {Effort: 1, X: 1}, "C": {Effort: 1, X: 4},
	}, Groups: []CraftGroup{{Name: "sequence", Priority: 2, Crafts: []string{"one", "two", "three"}}}}
	goals := []Goal{{Item: "one", Substrings: []string{"a"}}, {Item: "two", Substrings: []string{"b"}}, {Item: "three", Substrings: []string{"c"}}}
	if got := Cost(bindings, goals, weights); got != 11 {
		t.Fatalf("cost = %v, want 3 + 2*(1+3)", got)
	}
	weights.Groups[0].Crafts = []string{"two", "one", "three"}
	if got := Cost(bindings, goals, weights); got != 13 {
		t.Fatalf("reordered cost = %v, want 13", got)
	}
	weights.Crafts = map[string]float64{"one": 0}
	if got := Cost(bindings, goals, weights); got != 2 {
		t.Fatalf("ignored craft must skip both incident edges: %v", got)
	}
	weights.Crafts = nil
	weights.Groups[0].Crafts = []string{"one", "absent", "three"}
	if got := Cost(bindings, goals, weights); got != 3 {
		t.Fatalf("absent craft must not bridge neighbours: %v", got)
	}
	weights.Groups[0].Priority = 0
	if got := Cost(bindings, goals, weights); got != 3 {
		t.Fatalf("disabled group cost = %v", got)
	}
}

func TestCraftGroupUsesAllSearchKeys(t *testing.T) {
	weights := Weights{Keys: map[string]KeyWeight{
		"A": {Effort: 1, X: 0}, "B": {Effort: 1, X: 2}, "C": {Effort: 1, X: 4},
	}, Groups: []CraftGroup{{Priority: 1, Crafts: []string{"one", "two"}}}}
	goals := []Goal{{Item: "one", Substrings: []string{"AB"}}, {Item: "two", Substrings: []string{"c"}}}
	if got := Cost(Layout{'a': "A", 'b': "B", 'c': "C"}, goals, weights); got != 6 {
		t.Fatalf("cost = %v, want 3 + average(4,2)", got)
	}
}

func TestWithinCraftDistanceAndFingerReuse(t *testing.T) {
	weights := Weights{Keys: map[string]KeyWeight{
		"X": {Effort: 1, Finger: "middle", X: 0, Y: 3},
		"C": {Effort: 1, Finger: "index", X: 1, Y: 3},
		"2": {Effort: 1, Finger: "middle", X: 0, Y: 0},
		"Z": {Effort: 1, Finger: "ring", X: -1, Y: 3},
	}, DistancePenalty: 1, FingerReusePenalty: 2,
		Transitions: map[Transition]float64{{From: "X", To: "C"}: 0, {From: "C", To: "2"}: 0}}
	bindings := Layout{'l': "X", 'h': "C", 'a': "2", 'b': "Z"}
	want := 3 + 1 + math.Sqrt(10) + 2
	if got := StringCost("lha", bindings, weights); math.Abs(got-want) > 1e-9 {
		t.Fatalf("distant key and nonconsecutive finger reuse cost = %v, want %v", got, want)
	}
	if got := StringCost("lhb", bindings, weights); got != 6 {
		t.Fatalf("nearby keys with distinct fingers cost = %v, want 6", got)
	}
	goals := []Goal{{Item: "pick", Substrings: []string{"lha"}}}
	weights.Crafts = map[string]float64{"pick": 5}
	if got := Cost(bindings, goals, weights); math.Abs(got-5*want) > 1e-9 {
		t.Fatalf("craft priority must scale comfort penalties: %v", got)
	}
	// Every pair of uses contributes, including repeated presses on one key.
	weights.DistancePenalty = 0
	if got := StringCost("lll", bindings, weights); got != 9 {
		t.Fatalf("three uses should charge three pairs: %v", got)
	}
	weights.FingerReusePenalty = 0
	if got := StringCost("lha", bindings, weights); got != 3 {
		t.Fatalf("zero penalties must preserve original costs: %v", got)
	}
}

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

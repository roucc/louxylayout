package layout

import (
	"math"
	"testing"
)

func scalar(v float64) *float64 { return &v }

func TestComfortCostsOverrideAndRepeatedKey(t *testing.T) {
	w := Weights{Keys: map[string]KeyWeight{"A": {Effort: 1, Finger: "ring"}, "Q": {Effort: 1, Finger: "ring", Y: 2}}, Transitions: map[Transition]float64{{From: "A", To: "Q"}: 2}, TransitionWeight: scalar(.5), LongReachThreshold: 1, LongReachPenalty: 2, SameFingerMovementPenalty: 3, FingerReusePenalty: 4}
	bindings := Layout{'a': "A", 'q': "Q"}
	// efforts 2 + reuse 4 + scaled override 1 + excess reach 2 + movement 3.
	if got := StringCost("aq", bindings, w); got != 12 {
		t.Fatalf("aq cost %v", got)
	}
	// Repeated key incurs reuse, but no movement or reach charge.
	if got := StringCost("aa", bindings, w); got != 6 {
		t.Fatalf("aa cost %v", got)
	}
	w.SearchEditing = &SearchEditCosts{ShiftHome: 1}
	w.Groups = []CraftGroup{{Priority: 1, Crafts: []string{"craft"}}}
	scorer := newSequenceScorer([]Goal{{Item: "craft", Substrings: []string{"aq"}}}, w)
	for i, text := range scorer.texts {
		if text == "aq" {
			scorer.evaluate(bindings, false)
			if got := scorer.textCost(i, bindings); got != 12 {
				t.Fatalf("compiled cost %v", got)
			}
		}
	}
}

func TestWorstCraftAndIndependentGroupWeights(t *testing.T) {
	w := Weights{Keys: map[string]KeyWeight{"A": {Effort: 1}, "S": {Effort: 2, X: 1}}, Crafts: map[string]float64{"a": 5, "b": 1}, WorstCraftPenalty: .5, Groups: []CraftGroup{{Priority: 2, Crafts: []string{"a", "b"}}}, GroupProximityWeight: scalar(0), GroupSequenceWeight: scalar(0), SearchEditing: &SearchEditCosts{ShiftHome: 1}}
	goals := []Goal{{Item: "a", Substrings: []string{"a"}}, {Item: "b", Substrings: []string{"b"}}}
	bindings := Layout{'a': "A", 'b': "S"}
	if got := Cost(bindings, goals, w); got != 9.5 {
		t.Fatalf("cost %v", got)
	}
	if got := LayeredCost(LayeredLayout{NonShift: bindings, Shift: bindings}, goals, w); got != 9.5 {
		t.Fatalf("layered cost %v", got)
	}
	w.GroupProximityWeight = scalar(3)
	if got := Cost(bindings, goals, w); got != 15.5 {
		t.Fatalf("proximity cost %v", got)
	}
	w.GroupProximityWeight = scalar(0)
	w.GroupSequenceWeight = scalar(2)
	plans, err := PlanGroups(bindings, goals, w)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := Cost(bindings, goals, w), 9.5+4*plans[0].Cost; math.Abs(got-want) > 1e-9 {
		t.Fatalf("sequence cost %v want %v", got, want)
	}
}

func TestNewComfortWeightsValidation(t *testing.T) {
	for _, w := range []Weights{{LongReachPenalty: -1}, {LongReachThreshold: math.NaN()}, {SameFingerMovementPenalty: math.Inf(1)}, {WorstCraftPenalty: -1}, {TransitionWeight: scalar(-1)}, {GroupProximityWeight: scalar(-1)}, {GroupSequenceWeight: scalar(-1)}} {
		if err := ValidateWeights(w); err == nil {
			t.Fatalf("accepted invalid weights %+v", w)
		}
	}
}

func TestComfortOptimizerMatchesPublicScoring(t *testing.T) {
	w := Weights{Keys: map[string]KeyWeight{"A": {Effort: 1, Finger: "ring"}, "S": {Effort: 1, Finger: "ring", X: 1}, "D": {Effort: 1, Finger: "middle", X: 2}}, Crafts: map[string]float64{"first": 5, "second": 2}, FingerReusePenalty: 3, LongReachPenalty: .5, LongReachThreshold: 1, RowChangePenalty: 1, SameFingerMovementPenalty: 1.5, WorstCraftPenalty: .5, TransitionWeight: scalar(.5), GroupProximityWeight: scalar(.25), GroupSequenceWeight: scalar(2), Groups: []CraftGroup{{Priority: 3, Crafts: []string{"first", "second"}}}, SearchEditing: &SearchEditCosts{ShiftHome: 1, Backspace: .5}}
	goals := []Goal{{Item: "first", Substrings: []string{"ab", "bc"}}, {Item: "second", Substrings: []string{"ac", "ba"}}}
	for _, shift := range []bool{false, true} {
		got, err := Optimize(goals, w, OptimizeOptions{Seed: 42, Restarts: 1, Iterations: 20, EnableShiftLayer: shift})
		if err != nil {
			t.Fatal(err)
		}
		want := Cost(got.Bindings, goals, w)
		if shift {
			want = LayeredCost(LayeredLayout{NonShift: got.Bindings, Shift: got.ShiftBindings}, goals, w)
		}
		if math.Abs(got.Cost-want) > 1e-9 {
			t.Fatalf("shift %v optimized %v rescored %v", shift, got.Cost, want)
		}
	}
}

func TestRowChangesPreferHorizontalRolls(t *testing.T) {
	w := Weights{Keys: map[string]KeyWeight{"X": {Effort: 2, Finger: "middle", X: 1.75, Y: 3}, "D": {Effort: 1, Finger: "index", X: 2.25, Y: 2}, "C": {Effort: 1.5, Finger: "index", X: 2.75, Y: 3}, "Space": {Effort: 1, Finger: "thumb", X: 3, Y: 4}}, DefaultTransitionPenalty: .5, TransitionWeight: scalar(.5), DistancePenalty: 1}
	b := Layout{'x': "X", 'd': "D", 'c': "C", ' ': "Space"}
	diagonal, horizontal := StringCost("xd", b, w), StringCost("xc", b, w)
	if diagonal >= horizontal {
		t.Fatal("fixture should prefer lower effort D before row penalty")
	}
	spaceBefore := StringCost("x ", b, w)
	w.RowChangePenalty = 1
	if got := StringCost("xd", b, w); math.Abs(got-diagonal-1) > 1e-9 {
		t.Fatalf("row surcharge incorrect: %v", got)
	}
	if got := StringCost("xc", b, w); got != horizontal {
		t.Fatalf("charged a horizontal roll: %v", got)
	}
	if StringCost("xd", b, w) <= StringCost("xc", b, w) {
		t.Fatal("row penalty did not prefer horizontal roll")
	}
	if got := StringCost("x ", b, w); got != spaceBefore {
		t.Fatalf("charged Space transition: %v", got)
	}
	if err := ValidateWeights(Weights{RowChangePenalty: -1}); err == nil {
		t.Fatal("accepted negative row penalty")
	}
}

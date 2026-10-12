package layout

import (
	"math"
	"reflect"
	"testing"
)

func TestPriorityConstructionPlacesImportantCraftFirst(t *testing.T) {
	w := Weights{Keys: map[string]KeyWeight{"A": {Effort: 1, Finger: "ring"}, "S": {Effort: 1, Finger: "middle", X: 1}, "D": {Effort: 4, Finger: "index", X: 2}}, Crafts: map[string]float64{"important": 5, "other": 1}, FingerReusePenalty: 3}
	goals := []Goal{{Item: "other", Substrings: []string{"c"}}, {Item: "important", Substrings: []string{"ab"}}}
	got, err := Optimize(goals, w, OptimizeOptions{Method: "priority", Restarts: 1, Iterations: 1, Seed: 42})
	if err != nil {
		t.Fatal(err)
	}
	if got.Bindings['c'] != "D" {
		t.Fatalf("lower-priority craft got a cheap key: %+v", got.Bindings)
	}
	if w.Keys[got.Bindings['a']].Finger == w.Keys[got.Bindings['b']].Finger {
		t.Fatal("important craft should use distinct fingers")
	}
	if cost := Cost(got.Bindings, goals, w); math.Abs(got.Cost-cost) > 1e-9 {
		t.Fatalf("score mismatch %v %v", got.Cost, cost)
	}
}

func TestPriorityLayersConstraintsAndDeterminism(t *testing.T) {
	w := Weights{Keys: map[string]KeyWeight{"A": {Effort: 1, Finger: "ring"}, "S": {Effort: 1, Finger: "middle", X: 1}, "D": {Effort: 1, Finger: "index", X: 2}, "Space": {Effort: 1, Finger: "thumb", X: 3, Y: 1}}, Crafts: map[string]float64{"ingot": 5, "tool": 4, "bed": 3}, PreferredLayers: map[string]CraftLayer{"ingot": ShiftLayer, "tool": NonShiftLayer, "bed": ShiftLayer}, LayerPreferencePenalty: 5, FingerReusePenalty: 3, RowChangePenalty: 1, WorstCraftPenalty: .5, Groups: []CraftGroup{{Name: "group", Priority: 4, Crafts: []string{"bed", "tool", "ingot"}, Before: map[string][]string{"ingot": {"tool"}}}}, SearchEditing: &SearchEditCosts{ShiftHome: 1, Backspace: .5}, PreferBothLayers: map[rune]float64{'a': 2}}
	goals := []Goal{{Item: "ingot", Substrings: []string{"ab", "bc"}, PreferredSubstrings: []string{"ab"}}, {Item: "tool", Substrings: []string{"ac"}}, {Item: "bed", Substrings: []string{"b "}}}
	options := OptimizeOptions{Method: "priority", EnableShiftLayer: true, Restarts: 2, Iterations: 100, Seed: 42, Fixed: Layout{' ': "Space"}, InitialBindings: map[string]rune{"A": 'a'}, InitialShiftBindings: map[string]rune{"D": 'a'}}
	got, err := Optimize(goals, w, options)
	if err != nil {
		t.Fatal(err)
	}
	b := LayeredLayout{NonShift: got.Bindings, Shift: got.ShiftBindings}
	if got.Bindings[' '] != "Space" || got.ShiftBindings[' '] != "Space" {
		t.Fatal("fixed Space was moved")
	}
	for _, assigned := range []Layout{got.Bindings, got.ShiftBindings} {
		seen := map[string]bool{}
		for _, key := range assigned {
			if seen[key] || key == "" {
				t.Fatalf("invalid assignment %+v", assigned)
			}
			seen[key] = true
		}
	}
	if cost := LayeredCost(b, goals, w); math.Abs(got.Cost-cost) > 1e-9 {
		t.Fatalf("score mismatch %v %v", got.Cost, cost)
	}
	ingotIndex, toolIndex := -1, -1
	for i, step := range got.GroupPlans[0].Steps {
		if step.Item == "ingot" {
			ingotIndex = i
			if step.Search != "ab" {
				t.Fatal("ignored allowed searches")
			}
		}
		if step.Item == "tool" {
			toolIndex = i
		}
		if step.Backspaces > 1 {
			t.Fatal("multiple Backspaces")
		}
	}
	if ingotIndex < 0 || toolIndex < ingotIndex {
		t.Fatal("ignored prerequisites")
	}
	again, err := Optimize(goals, w, options)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, again) {
		t.Fatal("priority optimizer is not reproducible")
	}
}

func TestPriorityClimbNeverWorsensActiveScore(t *testing.T) {
	goals := []Goal{{Item: "craft", Substrings: []string{"ab"}}}
	w := Weights{Keys: map[string]KeyWeight{"A": {Effort: 1}, "S": {Effort: 2}, "D": {Effort: 5}}}
	options := OptimizeOptions{Method: "priority", Restarts: 1, Iterations: 1, Seed: 42}
	before, err := Optimize(goals, w, options)
	if err != nil {
		t.Fatal(err)
	}
	options.Iterations = 100
	after, err := Optimize(goals, w, options)
	if err != nil {
		t.Fatal(err)
	}
	if after.Cost > before.Cost {
		t.Fatalf("refinement worsened score: %v to %v", before.Cost, after.Cost)
	}
}

func TestOptimizerRejectsUnknownMethod(t *testing.T) {
	if _, err := Optimize(nil, Weights{}, OptimizeOptions{Method: "unknown"}); err == nil {
		t.Fatal("accepted unknown optimizer")
	}
}

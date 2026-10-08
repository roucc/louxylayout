package layout

import (
	"reflect"
	"testing"
)

func TestBeforeRuleExcludesCheaperInvalidOrder(t *testing.T) {
	goals := []Goal{{Item: "ingot", Substrings: []string{"abcd"}}, {Item: "tool", Substrings: []string{"ab"}}}
	bindings, weights := sequenceFixture(goals)
	free, err := PlanGroups(bindings, goals, weights)
	if err != nil {
		t.Fatal(err)
	}
	if free[0].Steps[0].Item != "tool" || free[0].Cost != 4 {
		t.Fatalf("fixture should prefer tool first: %+v", free)
	}
	weights.Groups[0].Before = map[string][]string{"ingot": {"tool"}}
	plans, err := PlanGroups(bindings, goals, weights)
	if err != nil {
		t.Fatal(err)
	}
	if plans[0].Steps[0].Item != "ingot" || plans[0].Steps[1].Item != "tool" || plans[0].Cost != 7 {
		t.Fatalf("constraint should require ingot before tool: %+v", plans)
	}
	assertSequenceReplay(t, plans[0], goals)
	weights.PreferredLayers = map[string]CraftLayer{"ingot": ShiftLayer, "tool": NonShiftLayer}
	weights.LayerPreferencePenalty = 5
	weights.LayerSwitchPenalty = .5
	layers := LayeredLayout{NonShift: bindings, Shift: copyLayout(bindings)}
	shifted, err := PlanLayeredGroups(layers, goals, weights)
	if err != nil {
		t.Fatal(err)
	}
	if shifted[0].Steps[0].Item != "ingot" || shifted[0].Steps[1].Item != "tool" || shifted[0].Cost != 8 {
		t.Fatalf("layered constraint failed: %+v", shifted)
	}
	assertLayeredReplay(t, shifted[0], layers)
	got, err := Optimize(goals, weights, OptimizeOptions{EnableShiftLayer: true, Fixed: bindings, FixedShift: bindings, Restarts: 1, Iterations: 1})
	if err != nil || !reflect.DeepEqual(got.GroupPlans, shifted) {
		t.Fatalf("optimizer failed to respect order: %+v, %v", got, err)
	}
}

func TestBeforeRulesChainsAndInactiveMembers(t *testing.T) {
	goals := []Goal{{Item: "ingot", Substrings: []string{"a"}}, {Item: "tool", Substrings: []string{"a"}}, {Item: "last", Substrings: []string{"a"}}}
	bindings, weights := sequenceFixture(goals)
	weights.Groups[0].Before = map[string][]string{"ingot": {"tool", "tool"}, "tool": {"last"}}
	plans, err := PlanGroups(bindings, goals, weights)
	if err != nil {
		t.Fatal(err)
	}
	for i, item := range []string{"ingot", "tool", "last"} {
		if plans[0].Steps[i].Item != item {
			t.Fatalf("transitive constraints not respected: %+v", plans[0])
		}
	}
	weights.Crafts = map[string]float64{"ingot": 0}
	plans, err = PlanGroups(bindings, goals, weights)
	if err != nil || len(plans[0].Steps) != 2 || plans[0].Steps[0].Item != "tool" {
		t.Fatalf("ignored prerequisite must be omitted: %+v, %v", plans, err)
	}
}

func TestBeforeRuleValidation(t *testing.T) {
	goals := []Goal{{Item: "ingot", Substrings: []string{"a"}}, {Item: "tool", Substrings: []string{"b"}}}
	bindings, weights := sequenceFixture(goals)
	for _, invalid := range []map[string][]string{
		{"typo": {"tool"}}, {"ingot": {"typo"}}, {"ingot": {"ingot"}}, {"ingot": {"tool"}, "tool": {"ingot"}},
	} {
		weights.Groups[0].Before = invalid
		if err := validateGroupOrder(weights.Groups[0]); err == nil {
			t.Fatalf("accepted invalid rule: %v", invalid)
		}
		if _, err := PlanGroups(bindings, goals, weights); err == nil {
			t.Fatalf("single-layer planner accepted invalid rule: %v", invalid)
		}
		if _, err := PlanLayeredGroups(LayeredLayout{NonShift: bindings, Shift: bindings}, goals, weights); err == nil {
			t.Fatalf("layered planner accepted invalid rule: %v", invalid)
		}
	}
}

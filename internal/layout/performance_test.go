package layout_test

import (
	"math"
	"testing"

	"louxylayout/internal/data"
	"louxylayout/internal/layout"
	"louxylayout/internal/profile"
	"louxylayout/internal/search"
	"louxylayout/profiles"
)

// Integration checks and benchmarks use the bundled default profile, so no
// personal settings are copied into Go test code.
func profileFixture(tb testing.TB) ([]layout.Goal, layout.Weights, layout.OptimizeOptions) {
	tb.Helper()
	encoded, err := profiles.Files.ReadFile(profiles.Default + ".json")
	if err != nil {
		tb.Fatal(err)
	}
	personal, err := profile.Parse(encoded)
	if err != nil {
		tb.Fatal(err)
	}
	weights, options, err := personal.Settings()
	if err != nil {
		tb.Fatal(err)
	}
	groups, items, err := data.Load(personal.Language)
	if err != nil {
		tb.Fatal(err)
	}
	finder := search.New(groups, items, personal.Inventory, personal.Goals)
	goals, err := personal.BuildGoals(finder)
	if err != nil {
		tb.Fatal(err)
	}
	minimum, err := layout.MinimumCharacters(goals)
	if err != nil {
		tb.Fatal(err)
	}
	options.InitialCharacters = minimum.Characters
	return goals, weights, options
}

func BenchmarkOptimizeCraftGroups(b *testing.B) { benchmarkOptimizer(b, false, true) }
func BenchmarkOptimizeShiftLayers(b *testing.B) { benchmarkOptimizer(b, true, true) }
func BenchmarkCraftOrderConstraints(b *testing.B) {
	b.Run("Free", func(b *testing.B) { benchmarkOptimizer(b, true, false) })
	b.Run("IngotsBeforeTools", func(b *testing.B) { benchmarkOptimizer(b, true, true) })
}

func benchmarkOptimizer(b *testing.B, shift, constrained bool) {
	goals, weights, options := profileFixture(b)
	options.Restarts, options.Iterations, options.EnableShiftLayer = 1, 200, shift
	if !constrained {
		for i := range weights.Groups {
			weights.Groups[i].Before = nil
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := layout.Optimize(goals, weights, options); err != nil {
			b.Fatal(err)
		}
	}
}

func TestProfileLayeredOptimizerCostAndPlans(t *testing.T) {
	goals, weights, options := profileFixture(t)
	options.Restarts, options.Iterations, options.EnableShiftLayer = 1, 30, true
	got, err := layout.Optimize(goals, weights, options)
	if err != nil {
		t.Fatal(err)
	}
	bindings := layout.LayeredLayout{NonShift: got.Bindings, Shift: got.ShiftBindings}
	if cost := layout.LayeredCost(bindings, goals, weights); math.Abs(got.Cost-cost) > 1e-7 {
		t.Fatalf("profile optimizer cost %v != rescored %v", got.Cost, cost)
	}
	for _, assigned := range []layout.Layout{got.Bindings, got.ShiftBindings} {
		seen := make(map[string]bool, len(assigned))
		for ch, key := range assigned {
			if seen[key] {
				t.Fatalf("two characters share key %q in one layer", key)
			}
			seen[key] = true
			if _, known := weights.Keys[key]; !known {
				t.Fatalf("unknown key %q for %q", key, ch)
			}
		}
	}
	for _, plan := range got.GroupPlans {
		assertPlanReplay(t, plan, bindings)
	}
	again, err := layout.PlanLayeredGroups(bindings, goals, weights)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != len(got.GroupPlans) {
		t.Fatal("group report count mismatch")
	}
	for i, plan := range again {
		if math.Abs(plan.Cost-got.GroupPlans[i].Cost) > 1e-7 {
			t.Fatalf("group report cost changed: %+v, %+v", plan, got.GroupPlans[i])
		}
	}
}

func assertPlanReplay(t *testing.T, plan layout.GroupPlan, bindings layout.LayeredLayout) {
	t.Helper()
	text, layer, sum := "", layout.NonShiftLayer, 0.0
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
			if _, known := bindings.Bindings(layer)[ch]; !known {
				t.Fatalf("text %q is unreachable on %s", step.Type, layer)
			}
		}
		text += step.Type
		if text != step.Search {
			t.Fatalf("actions produced %q instead of %q", text, step.Search)
		}
		sum += step.Cost
	}
	if math.Abs(sum-plan.Cost) > 1e-7 {
		t.Fatalf("reported step costs %v != plan cost %v", sum, plan.Cost)
	}
}

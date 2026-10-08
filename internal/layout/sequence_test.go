package layout

import (
	"math"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

func sequenceFixture(goals []Goal) (Layout, Weights) {
	bindings := make(Layout)
	weights := Weights{Keys: make(map[string]KeyWeight), SearchEditing: &SearchEditCosts{ShiftHome: 1, Backspace: .5}}
	group := CraftGroup{Name: "sequence", Priority: 1}
	for _, goal := range goals {
		group.Crafts = append(group.Crafts, goal.Item)
		for _, sub := range goal.Substrings {
			for _, ch := range strings.ToLower(sub) {
				bindings[ch] = string(ch)
				weights.Keys[string(ch)] = KeyWeight{Effort: 1}
			}
		}
	}
	weights.Groups = []CraftGroup{group}
	return bindings, weights
}

func TestFortressSearchOverlap(t *testing.T) {
	goals := []Goal{
		{Item: "bow", Substrings: []string{"ue"}},
		{Item: "anchor", Substrings: []string{"ivs", "liv"}},
		{Item: "bed", Substrings: []string{"n ", "l "}},
		{Item: "pick", Substrings: []string{"lha"}},
	}
	bindings, weights := sequenceFixture(goals)
	plans, err := PlanGroups(bindings, goals, weights)
	if err != nil {
		t.Fatal(err)
	}
	bed, pick := stepFor(plans[0], "bed"), stepFor(plans[0], "pick")
	if plans[0].Cost != 11.5 || bed.Search != "l " || pick.Action != "backspace" || pick.Backspaces != 1 || pick.Type != "ha" {
		t.Fatalf("expected l SPACE, BS, ha: %+v", plans[0])
	}
	assertSequenceReplay(t, plans[0], goals)
}

func TestBastionSearchOverlap(t *testing.T) {
	goals := []Goal{
		{Item: "bed", Substrings: []string{"n ", "l "}},
		{Item: "ingot", Substrings: []string{"rnb"}},
		{Item: "axe", Substrings: []string{"rnø"}},
		{Item: "sword", Substrings: []string{"rnsv"}},
	}
	bindings, weights := sequenceFixture(goals)
	plans, err := PlanGroups(bindings, goals, weights)
	if err != nil {
		t.Fatal(err)
	}
	axe, sword := stepFor(plans[0], "axe"), stepFor(plans[0], "sword")
	if plans[0].Cost != 10 || axe.Type != "ø" || sword.Type != "sv" || axe.Backspaces != 1 || sword.Backspaces != 1 {
		t.Fatalf("expected rnb, BS, ø, BS, sv: %+v", plans[0])
	}
	assertSequenceReplay(t, plans[0], goals)
}

func TestSequenceControlsAndRestrictions(t *testing.T) {
	goals := []Goal{{Item: "one", Substrings: []string{"abc"}}, {Item: "two", Substrings: []string{"abd"}}}
	bindings, weights := sequenceFixture(goals)
	weights.SearchEditing.ShiftHome = 10
	plans, err := PlanGroups(bindings, goals, weights)
	if err != nil {
		t.Fatal(err)
	}
	if plans[0].Cost != 4.5 || plans[0].Steps[1].Backspaces != 1 {
		t.Fatalf("expected prefix reuse: %+v", plans[0])
	}
	weights.SearchEditing.Backspace = 100
	plans, err = PlanGroups(bindings, goals, weights)
	if err != nil {
		t.Fatal(err)
	}
	if plans[0].Cost != 16 || plans[0].Steps[1].Action != "shift-home" {
		t.Fatalf("expected replacement: %+v", plans[0])
	}
	goals[1].PreferredSubstrings = []string{"abd"}
	plans, err = PlanGroups(bindings, goals, weights)
	if err != nil {
		t.Fatal(err)
	}
	if stepFor(plans[0], "two").Search != "abd" {
		t.Fatal("plan ignored search restriction")
	}
	delete(bindings, 'd')
	if _, err := PlanGroups(bindings, goals, weights); err == nil {
		t.Fatal("unreachable allowed search should fail")
	}
	weights.SearchEditing = nil
	if plans, err := PlanGroups(bindings, goals, weights); err != nil || len(plans) != 0 {
		t.Fatalf("disabled planner: %v, %v", plans, err)
	}
}

func TestSequenceAppendKeepAndDelete(t *testing.T) {
	goals := []Goal{{Item: "one", Substrings: []string{"ø"}}, {Item: "two", Substrings: []string{"øa"}}, {Item: "three", Substrings: []string{"øa"}}, {Item: "four", Substrings: []string{"ø"}}}
	bindings, weights := sequenceFixture(goals)
	plans, err := PlanGroups(bindings, goals, weights)
	if err != nil {
		t.Fatal(err)
	}
	if plans[0].Cost != 2 || plans[0].Steps[1].Action != "keep" || plans[0].Steps[2].Type != "a" || plans[0].Steps[3].Action != "keep" {
		t.Fatalf("order should put identical searches together and avoid deletion: %+v", plans[0])
	}
	assertSequenceReplay(t, plans[0], goals)
	weights.Groups[0].Crafts = []string{"one", "missing", "two"}
	plans, err = PlanGroups(bindings, goals, weights)
	if err != nil {
		t.Fatal(err)
	}
	if plans[0].Steps[1].Type != "a" || plans[0].Steps[1].Action != "type" || plans[0].Cost != 2 {
		t.Fatalf("missing craft must be omitted: %+v", plans[0])
	}
	weights.Groups[0].Crafts = []string{"one", "three", "two"}
	weights.Crafts = map[string]float64{"three": 0}
	plans, err = PlanGroups(bindings, goals, weights)
	if err != nil || plans[0].Cost != 2 {
		t.Fatalf("ignored craft must be omitted: %+v, %v", plans, err)
	}
}

func TestSequenceDPMatchesExhaustive(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for iteration := 0; iteration < 40; iteration++ {
		goals := make([]Goal, 3)
		for i := range goals {
			goals[i].Item = string(rune('0' + i))
			for j := 0; j < 4; j++ {
				sub := ""
				for n := 1 + rng.Intn(4); n > 0; n-- {
					sub += string(rune('a' + rng.Intn(3)))
				}
				goals[i].Substrings = append(goals[i].Substrings, sub)
			}
		}
		bindings, weights := sequenceFixture(goals)
		weights.SearchEditing.ShiftHome = rng.Float64() * 4
		weights.SearchEditing.Backspace = rng.Float64() * 2
		weights.FingerReusePenalty = .3
		weights.DistancePenalty = .4
		for key, weight := range weights.Keys {
			weight.X = float64([]rune(key)[0] - 'a')
			weight.Finger = "index"
			weights.Keys[key] = weight
		}
		plans, err := PlanGroups(bindings, goals, weights)
		if err != nil {
			t.Fatal(err)
		}
		want := exhaustiveSequenceCost(goals, bindings, weights)
		if math.Abs(plans[0].Cost-want) > 1e-9 {
			t.Fatalf("DP cost %v, exhaustive %v, goals %+v", plans[0].Cost, want, goals)
		}
		sum := 0.0
		for _, step := range plans[0].Steps {
			sum += step.Cost
		}
		if math.Abs(sum-want) > 1e-9 {
			t.Fatalf("reported action costs %v != %v", sum, want)
		}
		assertSequenceReplay(t, plans[0], goals)
	}
}

func exhaustiveOrderedSequenceCost(goals []Goal, bindings Layout, weights Weights) float64 {
	var visit func(int, string) float64
	visit = func(stage int, previous string) float64 {
		if stage == len(goals) {
			return 0
		}
		best := math.Inf(1)
		for _, sub := range goals[stage].Substrings {
			cost := StringCost(sub, bindings, weights)
			if stage > 0 {
				cost += weights.SearchEditing.ShiftHome
				if len([]rune(previous)) == 1 {
					cost = math.Min(cost, weights.SearchEditing.Backspace+StringCost(sub, bindings, weights))
				}
				from, to := []rune(previous), []rune(sub)
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
			best = math.Min(best, cost+visit(stage+1, sub))
		}
		return best
	}
	return visit(0, "")
}

func assertSequenceReplay(t *testing.T, plan GroupPlan, goals []Goal) {
	t.Helper()
	text := ""
	for _, step := range plan.Steps {
		if step.Backspaces > 1 {
			t.Fatalf("plan contains multiple Backspaces: %+v", step)
		}
		switch step.Action {
		case "shift-home":
			text = ""
		case "backspace":
			text = string([]rune(text)[:len([]rune(text))-step.Backspaces])
		}
		text += step.Type
		if text != step.Search {
			t.Fatalf("actions produced %q instead of %q", text, step.Search)
		}
		valid := false
		for _, goal := range goals {
			if goal.Item != step.Item {
				continue
			}
			for _, sub := range goal.Substrings {
				valid = valid || strings.EqualFold(text, sub)
			}
		}
		if !valid {
			t.Fatalf("plan produced invalid search %q", text)
		}
	}
}

func TestOptimizeSequenceCostAndCharacterRetention(t *testing.T) {
	goals := []Goal{{Item: "bed", Substrings: []string{"n ", "l "}}, {Item: "pick", Substrings: []string{"ha", "lha"}}}
	bindings, weights := sequenceFixture(goals)
	weights.SearchEditing.ShiftHome = 10
	options := OptimizeOptions{Fixed: bindings, Restarts: 1, Iterations: 1}
	got, err := Optimize(goals, weights, options)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(got.Cost-Cost(got.Bindings, goals, weights)) > 1e-9 {
		t.Fatalf("optimizer cost mismatch: %+v", got)
	}
	if got.GroupPlans[0].Steps[1].Search != "lha" {
		t.Fatalf("longer overlap search must survive standalone pruning: %+v", got.GroupPlans)
	}
	plans, err := PlanGroups(got.Bindings, goals, weights)
	if err != nil || !reflect.DeepEqual(plans, got.GroupPlans) {
		t.Fatalf("report mismatch: %+v, %v", plans, err)
	}
}

func TestOptimizeRetainsCharactersUsedOnlyInGroupPlan(t *testing.T) {
	goals := []Goal{{Item: "one", Substrings: []string{"xx", "xxy"}}, {Item: "two", Substrings: []string{"zz", "xxyz"}}}
	_, weights := sequenceFixture(goals)
	weights.SearchEditing.ShiftHome = 100
	got, err := Optimize(goals, weights, OptimizeOptions{Restarts: 2, Iterations: 100, Seed: 42})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Bindings['y']; !ok {
		t.Fatalf("group-only character y was removed: %+v", got)
	}
	if got.Cost != 8 || got.Cost != Cost(got.Bindings, goals, weights) {
		t.Fatalf("expected standalone cost 4 + sequence cost 4: %+v", got)
	}
	if stepFor(got.GroupPlans[0], "two").Search != "xxyz" {
		t.Fatalf("expected overlap search xxyz: %+v", got.GroupPlans)
	}
}

func TestSequenceRejectsTwoBackspaces(t *testing.T) {
	goals := []Goal{{Item: "one", Substrings: []string{"abcx"}}, {Item: "two", Substrings: []string{"abdy"}}}
	bindings, weights := sequenceFixture(goals)
	weights.SearchEditing.ShiftHome = 100
	plans, err := PlanGroups(bindings, goals, weights)
	if err != nil {
		t.Fatal(err)
	}
	step := plans[0].Steps[1]
	if step.Action != "shift-home" || step.Backspaces != 0 || len([]rune(step.Type)) != 4 || plans[0].Cost != 108 {
		t.Fatalf("two Backspaces must never be used, even when cheaper: %+v", plans[0])
	}
}

func exhaustiveSequenceCost(goals []Goal, bindings Layout, weights Weights) float64 {
	best := math.Inf(1)
	var visit func(int)
	ordered := append([]Goal(nil), goals...)
	visit = func(i int) {
		if i == len(ordered) {
			best = math.Min(best, exhaustiveOrderedSequenceCost(ordered, bindings, weights))
			return
		}
		for j := i; j < len(ordered); j++ {
			ordered[i], ordered[j] = ordered[j], ordered[i]
			visit(i + 1)
			ordered[i], ordered[j] = ordered[j], ordered[i]
		}
	}
	visit(0)
	return best
}

func stepFor(plan GroupPlan, item string) CraftStep {
	for _, step := range plan.Steps {
		if step.Item == item {
			return step
		}
	}
	return CraftStep{}
}

func TestGroupOrderIgnoresConfiguredOrder(t *testing.T) {
	goals := []Goal{{Item: "long", Substrings: []string{"abcd"}}, {Item: "short", Substrings: []string{"ab"}}, {Item: "other", Substrings: []string{"abd"}}}
	bindings, weights := sequenceFixture(goals)
	plans, err := PlanGroups(bindings, goals, weights)
	if err != nil {
		t.Fatal(err)
	}
	if plans[0].Cost != exhaustiveSequenceCost(goals, bindings, weights) {
		t.Fatalf("did not choose cheapest order: %+v", plans[0])
	}
	// Including the same craft twice still means crafting it once.
	weights.Groups[0].Crafts = []string{"other", "long", "short", "long"}
	reordered, err := PlanGroups(bindings, goals, weights)
	if err != nil || !reflect.DeepEqual(plans, reordered) {
		t.Fatalf("list order or duplicates affected plan: %+v, %+v, %v", plans, reordered, err)
	}
	assertSequenceReplay(t, plans[0], goals)
	seen := make(map[string]bool)
	for _, step := range plans[0].Steps {
		if seen[step.Item] {
			t.Fatalf("craft visited twice: %s", step.Item)
		}
		seen[step.Item] = true
	}
	if len(seen) != len(goals) {
		t.Fatalf("plan omitted crafts: %+v", plans[0])
	}
}

func TestGroupPlanningSizeLimit(t *testing.T) {
	goals := make([]Goal, maxGroupCrafts+1)
	for i := range goals {
		goals[i] = Goal{Item: string(rune('a' + i)), Substrings: []string{"x"}}
	}
	bindings, weights := sequenceFixture(goals)
	if _, err := PlanGroups(bindings, goals, weights); err == nil {
		t.Fatal("oversized exact planning should fail explicitly")
	}
	if _, err := Optimize(goals, weights, OptimizeOptions{}); err == nil {
		t.Fatal("optimizer should reject oversized group")
	}
	// Ignored members do not count toward the active group size limit.
	weights.Crafts = map[string]float64{goals[len(goals)-1].Item: 0}
	plans, err := PlanGroups(bindings, goals, weights)
	if err != nil || len(plans[0].Steps) != maxGroupCrafts {
		t.Fatalf("active craft count mismatch: %+v, %v", plans, err)
	}
}

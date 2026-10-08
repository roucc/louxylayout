package layout

import (
	"math"
	"math/rand"
	"testing"
)

func TestStartingBindings(t *testing.T) {
	weights := Weights{Keys: map[string]KeyWeight{"A": {}, "Q": {}, "W": {}, "E": {}, "Space": {}}}
	fixed := Layout{' ': "Space"}
	preferred, err := normalizeInitialBindings(map[string]rune{"A": 'N', "Q": 'L', "W": 'H', "E": 'A'}, fixed, weights)
	if err != nil {
		t.Fatal(err)
	}
	got := startingLayout([]rune("ahln"), []string{"A", "E", "Q", "W"}, fixed, preferred, rand.New(rand.NewSource(42)))
	for ch, key := range (Layout{'n': "A", 'l': "Q", 'h': "W", 'a': "E", ' ': "Space"}) {
		if got[ch] != key {
			t.Fatalf("starting binding %q = %q, want %q", ch, got[ch], key)
		}
	}
	for _, invalid := range []map[string]rune{{"Missing": 'n'}, {"Q": 'n', "W": 'n'}, {"Space": 'n'}, {"Q": ' '}, {"Q": 'N', "W": 'n'}} {
		if _, err := normalizeInitialBindings(invalid, fixed, weights); err == nil {
			t.Fatalf("accepted invalid initial bindings: %v", invalid)
		}
	}
}

func TestStartingBindingsCanMove(t *testing.T) {
	weights := Weights{Keys: map[string]KeyWeight{"A": {Effort: 5}, "S": {Effort: 1}}}
	goals := []Goal{{Item: "craft", Substrings: []string{"n"}}}
	got, err := Optimize(goals, weights, OptimizeOptions{InitialBindings: map[string]rune{"A": 'N'}, Seed: 42, Restarts: 1, Iterations: 100})
	if err != nil {
		t.Fatal(err)
	}
	if got.Bindings['n'] != "S" || got.Cost != 1 {
		t.Fatalf("initial binding should be movable: %+v", got)
	}
}

func TestStartingBindingsIncludeExtraCharacters(t *testing.T) {
	weights := Weights{Keys: map[string]KeyWeight{"A": {Effort: 1}, "S": {Effort: 1}}}
	goals := []Goal{{Item: "craft", Substrings: []string{"n"}}}
	if _, err := Optimize(goals, weights, OptimizeOptions{InitialBindings: map[string]rune{"A": 'L'}, Restarts: 1, Iterations: 1}); err != nil {
		t.Fatal(err)
	}
}

func TestPreferredSearches(t *testing.T) {
	goals := []Goal{{Item: "bed", Substrings: []string{"l ", "n ", "s "}}}
	preferred, err := WithPreferredSearches(goals, map[string][]string{"bed": {"L ", "n "}})
	if err != nil {
		t.Fatal(err)
	}
	if len(goals[0].PreferredSubstrings) != 0 {
		t.Fatal("preferences mutated input goals")
	}
	weights := Weights{Keys: map[string]KeyWeight{
		"L": {Effort: 2}, "N": {Effort: 3}, "S": {Effort: 1}, "Space": {Effort: 1},
	}, Crafts: map[string]float64{"bed": 5}}
	bindings := Layout{'l': "L", 'n': "N", 's': "S", ' ': "Space"}
	sub, cost := cheapestSearch(preferred[0], bindings, weights)
	if sub != "l " || cost != 3 || Cost(bindings, preferred, weights) != 15 {
		t.Fatalf("preferred search = %q, cost = %v", sub, cost)
	}
	delete(bindings, 'l')
	delete(bindings, 'n')
	sub, cost = cheapestSearch(preferred[0], bindings, weights)
	if sub != "" || !math.IsInf(cost, 1) || !math.IsInf(Cost(bindings, preferred, weights), 1) {
		t.Fatalf("unlisted alternative must be excluded: %q, %v", sub, cost)
	}
	unrestricted, err := WithPreferredSearches(goals, map[string][]string{"bed": {}})
	if err != nil {
		t.Fatal(err)
	}
	if got := Cost(bindings, unrestricted, weights); got != 10 {
		t.Fatalf("empty list must allow alternatives: %v", got)
	}
	for _, invalid := range []map[string][]string{{"bed": {"not a search"}}, {"unknown": {"n "}}, {"bed": {""}}} {
		if _, err := WithPreferredSearches(goals, invalid); err == nil {
			t.Fatalf("accepted invalid preferences: %v", invalid)
		}
	}
	if _, err := WithPreferredSearches(goals, map[string][]string{"bed": {}}); err != nil {
		t.Fatal(err)
	}
}

func TestPreferredLongerSearchSurvivesPruning(t *testing.T) {
	goals := []Goal{{Item: "craft", Substrings: []string{"a", "ab"}, PreferredSubstrings: []string{"ab"}}}
	weights := Weights{Keys: map[string]KeyWeight{"A": {Effort: 1}, "B": {Effort: 1}}}
	got, err := Optimize(goals, weights, OptimizeOptions{Fixed: Layout{'a': "A", 'b': "B"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Searches[0].Substrings[0] != "ab" || got.Cost != 2 || got.Cost != Cost(got.Bindings, goals, weights) {
		t.Fatalf("preferred longer search was pruned: %+v", got)
	}
}

func TestRestrictedSearchesSeedMinimumCharacters(t *testing.T) {
	goals := []Goal{{Item: "craft", Substrings: []string{"a", "b", "bc"}, PreferredSubstrings: []string{"b", "bc"}}}
	minimum, err := MinimumCharacters(goals)
	if err != nil {
		t.Fatal(err)
	}
	if string(minimum.Characters) != "b" {
		t.Fatalf("minimum must use only allowed searches: %q", string(minimum.Characters))
	}
	weights := Weights{Keys: map[string]KeyWeight{"A": {Effort: 1}, "B": {Effort: 1}, "C": {Effort: 1}}}
	got, err := Optimize(goals, weights, OptimizeOptions{Restarts: 1, Iterations: 10})
	if err != nil {
		t.Fatal(err)
	}
	if got.Searches[0].Substrings[0] != "b" || got.Cost != Cost(got.Bindings, goals, weights) {
		t.Fatalf("restricted selection mismatch: %+v", got)
	}
	if _, err := Optimize(minimum.Goals, weights, OptimizeOptions{Restarts: 1, Iterations: 10}); err != nil {
		t.Fatal(err)
	}
}

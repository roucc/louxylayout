package layout

import (
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

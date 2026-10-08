package search

import (
	"reflect"
	"strings"
	"testing"

	"louxylayout/internal/config"
	"louxylayout/internal/data"
)

func TestBlazePowderRejectsLoomJunk(t *testing.T) {
	groups, items, err := data.Load("no_no")
	if err != nil {
		t.Fatal(err)
	}
	finder := New(groups, items, config.Inventory, config.Goals)
	if !finder.IsCraftable("block.minecraft.loom") {
		t.Fatal("fixture must include craftable loom")
	}
	foundValid := false
	for _, result := range finder.ShortestUniqueSubstringWithJunk("item.minecraft.blaze_powder") {
		if strings.ToLower(result.Sub) == "ve" {
			t.Fatal("ve must be rejected because it also shows loom")
		}
		foundValid = true
	}
	if !foundValid {
		t.Fatal("blaze powder should still have valid searches")
	}
	for _, sub := range finder.ShortestUniqueSubstringForItem("item.minecraft.blaze_powder", true) {
		if strings.ToLower(sub) == "ve" {
			t.Fatal("pure search must reject ve too")
		}
	}
}

func TestGlowstoneUsesInventoryGrid(t *testing.T) {
	groups, items, err := data.Load("no_no")
	if err != nil {
		t.Fatal(err)
	}
	finder := New(groups, items, config.Inventory, config.Goals)
	hasES := func() bool {
		for _, result := range finder.ShortestUniqueSubstringWithJunk("block.minecraft.glowstone") {
			if result.Sub == "es" {
				return true
			}
		}
		return false
	}
	if finder.targetGridSize("item.minecraft.blaze_powder") != 2 || finder.targetGridSize("block.minecraft.glowstone") != 2 || finder.targetGridSize("item.minecraft.bow") != 3 {
		t.Fatal("targets must use their smallest actual crafting grid")
	}
	if !hasES() {
		t.Fatal("es should work for glowstone in the inventory grid")
	}
	finder.GridSize = 3
	if hasES() {
		t.Fatal("es should have avoidable junk at a crafting table")
	}
}

func TestUnavoidableJunk(t *testing.T) {
	finder := &Search{
		Items: map[string]string{"bow": "Bow", "bowl": "Bowl", "boat": "Boat"},
		RecipeGroups: [][]data.Recipe{
			{{Output: "bow"}}, {{Output: "bowl"}}, {{Output: "boat"}},
		},
		itemGroupMap:       map[string]int{"bow": 0, "bowl": 1, "boat": 2},
		itemMinSize:        map[string]int{"bow": 3},
		craftableGroupSize: map[int]int{0: 3, 1: 3, 2: 3},
		goalGroups:         map[int][]string{0: {"bow"}},
	}
	results := finder.ShortestUniqueSubstringWithJunk("bow")
	if len(results) == 0 {
		t.Fatal("bow must have candidates despite unavoidable bowl junk")
	}
	foundBow := false
	for _, result := range results {
		if result.Sub == "B" || result.Sub == "Bo" || result.Sub == "o" {
			t.Fatalf("accepted avoidable boat junk: %+v", result)
		}
		if !reflect.DeepEqual(result.Unavoidable, []string{"bowl"}) {
			t.Fatalf("unavoidable junk = %v, want bowl", result.Unavoidable)
		}
		foundBow = foundBow || result.Sub == "Bow"
	}
	if !foundBow {
		t.Fatal("full bow search must be accepted")
	}

	// A target with a clean search should not gain avoidable non-goal junk.
	finder.Items["bowl"] = "Plate"
	for _, result := range finder.ShortestUniqueSubstringWithJunk("bow") {
		if len(result.Unavoidable) != 0 || result.Sub == "Bo" {
			t.Fatalf("unexpected junk allowance: %+v", result)
		}
	}
}

func TestJunklessRejectsAvoidableGoalMatches(t *testing.T) {
	finder := New([][]data.Recipe{
		{{Output: "bow", Size: 3}},
		{{Output: "bowl", Size: 3}},
		{{Output: "boat", Size: 3}},
	}, map[string]string{"bow": "Bow", "bowl": "Bowl", "boat": "Boat"}, nil, []string{"bow", "bowl", "boat"})
	hasB := false
	for _, result := range finder.ShortestUniqueSubstringWithJunk("bow") {
		hasB = hasB || result.Sub == "B"
	}
	if !hasB {
		t.Fatal("good-junk mode should accept B matching other goals")
	}
	results := finder.JunklessSubstrings("bow")
	if len(results) == 0 {
		t.Fatal("unavoidable bowl matches should remain usable")
	}
	for _, result := range results {
		if result.Sub == "B" || result.Sub == "Bo" || result.Sub == "o" || len(result.Also) != 0 {
			t.Fatalf("accepted avoidable goal junk: %+v", result)
		}
		if !reflect.DeepEqual(result.Unavoidable, []string{"bowl"}) {
			t.Fatalf("expected unavoidable bowl: %+v", result)
		}
	}
}

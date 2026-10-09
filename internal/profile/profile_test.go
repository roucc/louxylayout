package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"louxylayout/internal/data"
	"louxylayout/internal/layout"
	"louxylayout/internal/search"
	"louxylayout/profiles"
)

const minimalProfile = `{
 "name":"other", "language":"no_no", "goals":["one"],
 "preferred_searches":{"one":["n "]},
 "preferred_bindings":{"A":"N"}, "preferred_shift_bindings":{"A":"n"},
 "fixed_bindings":{"Space":"Space"}, "fixed_shift_bindings":{"Space":" "},
 "weights":{
  "keys":{"A":{"effort":1,"finger":"pinky","x":0.25,"y":2},"Space":{"effort":1,"finger":"thumb","x":3,"y":4}},
  "crafts":{"one":5}, "preferred_layers":{"one":"shift"},
  "prefer_both_layers":{"n":5}, "layer_preference_penalty":5,
  "transitions":[{"from":"A","to":"Space","cost":0.2}]
 },
 "optimizer":{"seed":0,"restarts":1,"iterations":2,"enable_shift_layer":false}
}`

func TestProfileConversionPreservesPersonalSettings(t *testing.T) {
	p, err := Parse([]byte(minimalProfile))
	if err != nil {
		t.Fatal(err)
	}
	weights, options, err := p.Settings()
	if err != nil {
		t.Fatal(err)
	}
	if p.PreferredSearches["one"][0] != "n " {
		t.Fatal("search spaces were not preserved")
	}
	if options.Seed != 0 || options.EnableShiftLayer || options.Restarts != 1 || options.Iterations != 2 {
		t.Fatalf("explicit optimizer values not preserved: %+v", options)
	}
	if options.InitialBindings["A"] != 'n' || options.InitialShiftBindings["A"] != 'n' || options.Fixed[' '] != "Space" || options.FixedShift[' '] != "Space" {
		t.Fatalf("binding conversion failed: %+v", options)
	}
	if weights.PreferredLayers["one"] != layout.ShiftLayer || weights.PreferBothLayers['n'] != 5 || weights.Transitions[layout.Transition{From: "A", To: "Space"}] != .2 {
		t.Fatalf("weights conversion failed: %+v", weights)
	}
}

func TestProfileDefaultsAndIsolation(t *testing.T) {
	p, err := Parse([]byte(`{"language":"en_gb","goals":["one"],"weights":{"keys":{"A":{"effort":1}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	weights, options, err := p.Settings()
	if err != nil {
		t.Fatal(err)
	}
	if options.Seed != 42 || !options.EnableShiftLayer {
		t.Fatalf("optimizer defaults incorrect: %+v", options)
	}
	if len(weights.PreferredLayers) != 0 || len(weights.Crafts) != 0 || len(weights.Groups) != 0 || len(options.InitialBindings) != 0 || len(p.PreferredSearches) != 0 {
		t.Fatal("other profile inherited personal preferences")
	}
	// Each parse owns its nested maps; modifying another person must not alter p.
	other, err := Parse([]byte(minimalProfile))
	if err != nil {
		t.Fatal(err)
	}
	other.Weights.Keys["A"] = KeyWeight{Effort: 9}
	if p.Weights.Keys["A"].Effort != 1 {
		t.Fatal("profiles share mutable state")
	}
}

func TestProfileNamedLoadingAndEmbeddedFallback(t *testing.T) {
	t.Chdir(t.TempDir())
	bundled, err := Load(profiles.Default)
	if err != nil {
		t.Fatal(err)
	}
	if bundled.Name != "roux" {
		t.Fatalf("default profile name = %q", bundled.Name)
	}
	if err := os.Mkdir("profiles", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("profiles", profiles.Default+".json"), []byte(minimalProfile), 0644); err != nil {
		t.Fatal(err)
	}
	local, err := Load(profiles.Default)
	if err != nil {
		t.Fatal(err)
	}
	if local.Name != "other" {
		t.Fatal("local profile did not override bundled copy")
	}
	if err := os.WriteFile("profiles/friend.json", []byte(minimalProfile), 0644); err != nil {
		t.Fatal(err)
	}
	names, err := List()
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{"friend"}
	bundledFiles, err := profiles.Files.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range bundledFiles {
		expected = append(expected, strings.TrimSuffix(file.Name(), ".json"))
	}
	sort.Strings(expected)
	if !reflect.DeepEqual(names, expected) {
		t.Fatalf("unexpected profile list: %v", names)
	}
	explicit, err := Load("profiles/friend.json")
	if err != nil || explicit.Name != "other" {
		t.Fatalf("explicit profile loading failed: %+v, %v", explicit, err)
	}
	if _, err := Load("missing"); err == nil {
		t.Fatal("unknown profile silently fell back")
	}
	if err := os.WriteFile("profiles/roux.json", []byte(`{"bad":true}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load("roux"); err == nil {
		t.Fatal("invalid local profile silently fell back to bundled default")
	}
}

func TestProfileValidation(t *testing.T) {
	invalid := []string{
		strings.Replace(minimalProfile, `"name":"other"`, `"typo":"other"`, 1),
		minimalProfile + ` {}`,
		strings.Replace(minimalProfile, `"A":"N"`, `"A":"nn"`, 1),
		strings.Replace(minimalProfile, `"A":"N"`, `"Unknown":"N"`, 1),
		strings.Replace(minimalProfile, `"effort":1`, `"effort":-1`, 1),
		strings.Replace(minimalProfile, `"one":"shift"`, `"one":"invalid"`, 1),
		strings.Replace(minimalProfile, `"A":"N"`, `"Space":"N"`, 1),
		strings.Replace(minimalProfile, `"from":"A"`, `"from":"Missing"`, 1),
		strings.Replace(minimalProfile, `"goals":["one"]`, `"goals":["one","one"]`, 1),
	}
	for _, text := range invalid {
		if _, err := Parse([]byte(text)); err == nil {
			t.Fatalf("invalid profile accepted: %s", text)
		}
	}
}

func TestBundledProfileProducesLayout(t *testing.T) {
	encoded, err := profiles.Files.ReadFile("roux.json")
	if err != nil {
		t.Fatal(err)
	}
	p, err := Parse(encoded)
	if err != nil {
		t.Fatal(err)
	}
	weights, options, err := p.Settings()
	if err != nil {
		t.Fatal(err)
	}
	groups, items, err := data.Load(p.Language)
	if err != nil {
		t.Fatal(err)
	}
	finder := search.New(groups, items, p.Inventory, p.Goals)
	goals, err := p.BuildGoals(finder)
	if err != nil {
		t.Fatal(err)
	}
	options.Restarts, options.Iterations = 1, 2
	got, err := layout.Optimize(goals, weights, options)
	if err != nil {
		t.Fatal(err)
	}
	if got.ShiftBindings == nil || len(got.Searches) != len(p.Goals) {
		t.Fatalf("profile did not produce complete layered layout: %+v", got)
	}
	// Profiles can be copied, renamed and edited without code changes.
	p.Name = "friend"
	copied, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(copied); err != nil {
		t.Fatal(err)
	}
}

func TestNorwegianBedPreferencesAreValid(t *testing.T) {
	groups, items, err := data.Load("no_no")
	if err != nil {
		t.Fatal(err)
	}
	const bed = "block.minecraft.white_bed"
	finder := search.New(groups, items,
		[]string{"block.minecraft.white_wool", "block.minecraft.oak_planks"},
		[]string{bed},
	)
	goal := layout.Goal{Item: bed}
	for _, candidate := range finder.JunklessSubstrings(bed) {
		goal.Substrings = append(goal.Substrings, candidate.Sub)
	}
	if _, err := layout.WithPreferredSearches([]layout.Goal{goal}, map[string][]string{bed: {"l ", "n "}}); err != nil {
		t.Fatal(err)
	}
}

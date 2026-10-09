package profile

import (
	"testing"

	"louxylayout/internal/data"
	"louxylayout/internal/layout"
	"louxylayout/internal/search"
	"louxylayout/profiles"
)

func junkFixture() *search.Search {
	return search.New([][]data.Recipe{
		{{Output: "bow", Size: 2}}, {{Output: "wood", Size: 2}},
	}, map[string]string{"bow": "uvbugi", "wood": "uvbwood"}, nil, []string{"bow"})
}

func TestExplicitJunkPolicy(t *testing.T) {
	p := Profile{Goals: []string{"bow"}, PreferredSearches: map[string][]string{"bow": {"vb"}}, AllowGoodJunk: true}
	finder := junkFixture()
	if _, err := p.BuildGoals(finder); err == nil {
		t.Fatal("good-junk mode must still reject avoidable non-goal matches")
	}
	p.AllowedJunkSearches = map[string][]string{"bow": {"VB"}}
	goals, err := p.BuildGoals(finder)
	if err != nil {
		t.Fatal(err)
	}
	if len(goals) != 1 || len(goals[0].Substrings) != 1 || goals[0].Substrings[0] != "VB" {
		t.Fatalf("exact allowance missing: %+v", goals)
	}
	// The exception must neither admit unrelated substrings nor bypass membership.
	p.PreferredSearches = map[string][]string{"bow": {"uv"}}
	if _, err := p.BuildGoals(finder); err == nil {
		t.Fatal("exception leaked to a different search")
	}
	p.PreferredSearches = nil
	p.AllowedJunkSearches = map[string][]string{"bow": {"not in target"}}
	if _, err := p.BuildGoals(finder); err == nil {
		t.Fatal("invalid target search accepted")
	}
	p.AllowedJunkSearches = map[string][]string{"unknown": {"vb"}}
	if _, err := p.BuildGoals(finder); err == nil {
		t.Fatal("unknown goal exception accepted")
	}
}

func TestLilyProfileProducesLayoutWithOriginalSearches(t *testing.T) {
	encoded, err := profiles.Files.ReadFile("lily.json")
	if err != nil {
		t.Fatal(err)
	}
	p, err := Parse(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "lily" || p.Language != "ovd" || !p.AllowGoodJunk {
		t.Fatalf("wrong Lily profile metadata: %+v", p)
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
		t.Fatalf("incomplete Lily layout: %+v", got)
	}
	for _, goal := range got.Searches {
		allowed := p.PreferredSearches[goal.Item]
		if len(allowed) == 0 {
			continue
		}
		matched := false
		for _, sub := range allowed {
			matched = matched || goal.Substrings[0] == sub
		}
		if !matched {
			t.Fatalf("search restriction ignored: %+v", goal)
		}
	}
}

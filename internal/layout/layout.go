// Package layout finds the louxy layout
package layout

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"strings"
)

type Goal struct {
	Item       string
	Substrings []string
}

type CharacterSet struct {
	Characters []rune
	// Goals retains every craft and all its usable searches, in input order.
	Goals []Goal
}

// MinimumCharacters finds the smallest distinct character set that can type
// at least one substring for every goal. It is case-insensitive and counts
// spaces and Unicode characters as keys. Empty substrings are not searches.
func MinimumCharacters(goals []Goal) (CharacterSet, error) {
	return MinimumCharactersWithContext(context.Background(), goals)
}

// MinimumCharactersWithContext performs the same exact search, with cancellation
// for expensive inputs. No partial result is returned on cancellation.
func MinimumCharactersWithContext(ctx context.Context, goals []Goal) (CharacterSet, error) {
	candidates := make([][]string, len(goals))
	for i, goal := range goals {
		sets := make(map[string]bool)
		for _, sub := range goal.Substrings {
			if sub != "" {
				sets[characterKey(sub)] = true
			}
		}
		for set := range sets {
			candidates[i] = append(candidates[i], set)
		}
		sort.Slice(candidates[i], func(a, b int) bool {
			x, y := candidates[i][a], candidates[i][b]
			if len([]rune(x)) != len([]rune(y)) {
				return len([]rune(x)) < len([]rune(y))
			}
			return x < y
		})
		// A superset never helps minimize the union: its subset already
		// satisfies this goal. Keep the original strings for the final result.
		var minimal []string
		for _, set := range candidates[i] {
			dominated := false
			for _, smaller := range minimal {
				if contains(set, smaller) {
					dominated = true
					break
				}
			}
			if !dominated {
				minimal = append(minimal, set)
			}
		}
		candidates[i] = minimal
		if len(minimal) == 0 {
			return CharacterSet{}, fmt.Errorf("goal %q has no nonempty substrings", goal.Item)
		}
	}

	best := ""
	for _, options := range candidates {
		best = characterKey(best + options[0])
	}
	visited := make(map[string]bool)
	var visit func(string) error
	visit = func(current string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if visited[current] || len([]rune(current)) >= len([]rune(best)) {
			return nil
		}
		visited[current] = true
		var branches []string
		for _, options := range candidates {
			covered := false
			for _, option := range options {
				if contains(current, option) {
					covered = true
					break
				}
			}
			if !covered && (branches == nil || len(options) < len(branches)) {
				branches = options
			}
		}
		if branches == nil {
			best = current
			return nil
		}
		unions := make([]string, 0, len(branches))
		for _, option := range branches {
			unions = append(unions, characterKey(current+option))
		}
		sort.SliceStable(unions, func(i, j int) bool {
			return len([]rune(unions[i])) < len([]rune(unions[j]))
		})
		for _, union := range unions {
			if err := visit(union); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(""); err != nil {
		return CharacterSet{}, err
	}
	result := CharacterSet{Characters: []rune(best), Goals: make([]Goal, len(goals))}
	for i, goal := range goals {
		result.Goals[i].Item = goal.Item
		for _, sub := range goal.Substrings {
			if sub != "" && contains(best, characterKey(sub)) {
				result.Goals[i].Substrings = append(result.Goals[i].Substrings, sub)
			}
		}
	}
	return result, nil
}

func characterKey(s string) string {
	set := make(map[rune]bool)
	for _, ch := range strings.ToLower(s) {
		set[ch] = true
	}
	chars := make([]rune, 0, len(set))
	for ch := range set {
		chars = append(chars, ch)
	}
	sort.Slice(chars, func(i, j int) bool { return chars[i] < chars[j] })
	return string(chars)
}

func contains(set, subset string) bool {
	for _, ch := range subset {
		if !strings.ContainsRune(set, ch) {
			return false
		}
	}
	return true
}

// RandomAssign maps each character to a distinct physical key supplied by the
// caller. All supplied keys are eligible; unused keys remain unassigned.
// Pass a seeded RNG for reproducible layouts.
func RandomAssign(characters []rune, keys []string, rng *rand.Rand) (map[rune]string, error) {
	chars := []rune(characterKey(string(characters)))
	if len(chars) > len(keys) {
		return nil, fmt.Errorf("need %d keys for %d characters, have %d", len(chars), len(chars), len(keys))
	}
	seen := make(map[string]bool)
	for _, key := range keys {
		if key == "" || seen[key] {
			return nil, fmt.Errorf("physical keys must be nonempty and distinct: %q", key)
		}
		seen[key] = true
	}
	if rng == nil {
		return nil, fmt.Errorf("random source is required")
	}
	positions := rng.Perm(len(keys))
	assignment := make(map[rune]string, len(chars))
	for i, ch := range chars {
		assignment[ch] = keys[positions[i]]
	}
	return assignment, nil
}

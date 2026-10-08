package layout

import (
	"fmt"
	"math/rand"
	"strings"
	"unicode"
)

// normalizeInitialBindings accepts uppercase typed characters for convenience.
// Fixed bindings remain authoritative; conflicts are configuration errors.
func normalizeInitialBindings(preferred map[string]rune, fixed Layout, weights Weights) (Layout, error) {
	normalized := make(Layout, len(preferred))
	used := make(map[string]rune, len(fixed)+len(preferred))
	for ch, key := range fixed {
		used[key] = ch
	}
	for key, ch := range preferred {
		ch = unicode.ToLower(ch)
		if _, ok := weights.Keys[key]; !ok {
			return nil, fmt.Errorf("initial key %q has no weights", key)
		}
		if existing, ok := normalized[ch]; ok && existing != key {
			return nil, fmt.Errorf("conflicting initial bindings for %q", ch)
		}
		if existing, ok := fixed[ch]; ok && existing != key {
			return nil, fmt.Errorf("initial binding for %q conflicts with fixed key %q", ch, existing)
		}
		if occupant, ok := used[key]; ok && occupant != ch {
			return nil, fmt.Errorf("initial key %q is already assigned to %q", key, occupant)
		}
		used[key], normalized[ch] = ch, key
	}
	return normalized, nil
}

// startingLayout fills unassigned characters randomly around supplied bindings.
func startingLayout(chars []rune, keys []string, fixed, preferred Layout, rng *rand.Rand) Layout {
	bindings := make(Layout, len(chars)+len(fixed))
	used := make(map[string]bool, len(keys))
	for ch, key := range fixed {
		bindings[ch], used[key] = key, true
	}
	for ch, key := range preferred {
		bindings[ch], used[key] = key, true
	}
	available := make([]string, 0, len(keys))
	for _, key := range keys {
		if !used[key] {
			available = append(available, key)
		}
	}
	permutation := rng.Perm(len(available))
	next := 0
	for _, ch := range chars {
		if _, assigned := bindings[ch]; !assigned {
			bindings[ch] = available[permutation[next]]
			next++
		}
	}
	return bindings
}

// WithPreferredSearches copies goals and restricts nonempty preference lists.
// Spaces are significant. Every preference must be an existing valid candidate.
func WithPreferredSearches(goals []Goal, preferences map[string][]string) ([]Goal, error) {
	known := make(map[string]bool, len(goals))
	result := make([]Goal, len(goals))
	for i, goal := range goals {
		known[goal.Item] = true
		if preferred, ok := preferences[goal.Item]; ok {
			goal.PreferredSubstrings = append([]string(nil), preferred...)
		}
		if err := validatePreferredSearches(goal); err != nil {
			return nil, err
		}
		if len(goal.PreferredSubstrings) > 0 {
			allowed := make([]string, 0, len(goal.PreferredSubstrings))
			for _, sub := range goal.Substrings {
				if preferredSearch(goal, sub) {
					allowed = append(allowed, sub)
				}
			}
			goal.Substrings = allowed
		}
		result[i] = goal
	}
	for item, preferred := range preferences {
		if len(preferred) > 0 && !known[item] {
			return nil, fmt.Errorf("preferred searches refer to unknown goal %q", item)
		}
	}
	return result, nil
}

func validatePreferredSearches(goal Goal) error {
	for _, preferred := range goal.PreferredSubstrings {
		found := false
		for _, sub := range goal.Substrings {
			if preferred != "" && strings.EqualFold(preferred, sub) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("preferred search %q for %q is not a valid candidate", preferred, goal.Item)
		}
	}
	return nil
}

func preferredSearch(goal Goal, sub string) bool {
	for _, preferred := range goal.PreferredSubstrings {
		if strings.EqualFold(preferred, sub) {
			return true
		}
	}
	return false
}

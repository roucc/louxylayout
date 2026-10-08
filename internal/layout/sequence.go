package layout

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// SearchEditCosts models existing controls; it does not assign physical keys.
// A nil Weights.SearchEditing disables sequence planning.
type SearchEditCosts struct {
	ShiftHome float64 // selecting the entire previous search
	Backspace float64 // deleting one rune at the end; at most one per transition
}

type CraftStep struct {
	Item       string
	Search     string // complete search visible after this step
	Action     string // type, shift-home, backspace, or keep
	Backspaces int
	Type       string // only the new text to type
	Cost       float64
}

type GroupPlan struct {
	Name     string
	Priority float64
	Cost     float64 // unweighted edit/typing cost
	Steps    []CraftStep
}

type sequenceCandidate struct {
	search   string
	text     int
	length   int
	prefixes []int
	incoming []int
	suffixes []int
}

type sequenceStage struct {
	item        string
	reset       bool // no active immediately preceding craft
	candidates  []sequenceCandidate
	prefixCount int
}

type sequenceGroup struct {
	group  CraftGroup
	stages []sequenceStage
}

// sequenceScorer compiles prefix indexes and suffixes once. For a prefix of
// length k, the best predecessor is min(previous cost + length*Backspace).
// This avoids comparing every candidate against every preceding candidate.
type sequenceScorer struct {
	weights                     Weights
	texts                       []string
	groups                      []sequenceGroup
	costs                       []float64
	stamps                      []uint64
	generation                  uint64
	characters                  []rune
	textCharacters              [][]int
	keyIDs                      map[string]int
	characterKeys               []int
	efforts                     []float64
	fingers                     []int
	fingerCounts                []int
	transitions                 []float64
	distances                   []float64
	previous, next, prefixCosts []float64
}

func newSequenceScorer(goals []Goal, weights Weights) *sequenceScorer {
	scorer := &sequenceScorer{weights: weights}
	if weights.SearchEditing == nil {
		return scorer
	}
	byItem := make(map[string]Goal, len(goals))
	for _, goal := range goals {
		if importance, ok := weights.Crafts[goal.Item]; ok && importance == 0 {
			continue
		}
		byItem[goal.Item] = goal
	}
	textIDs := make(map[string]int)
	intern := func(text string) int {
		if id, ok := textIDs[text]; ok {
			return id
		}
		id := len(scorer.texts)
		textIDs[text] = id
		scorer.texts = append(scorer.texts, text)
		return id
	}
	maxCandidates, maxPrefixes := 0, 0
	for _, group := range weights.Groups {
		if group.Priority == 0 {
			continue
		}
		compiled := sequenceGroup{group: group}
		previousActive := false
		for _, item := range group.Crafts {
			goal, active := byItem[item]
			if !active {
				previousActive = false
				continue
			}
			stage := sequenceStage{item: item, reset: !previousActive}
			seen := make(map[string]bool)
			for _, sub := range goal.Substrings {
				if sub == "" || len(goal.PreferredSubstrings) > 0 && !preferredSearch(goal, sub) {
					continue
				}
				lower := strings.ToLower(sub)
				if seen[lower] {
					continue
				}
				seen[lower] = true
				runes := []rune(lower)
				candidate := sequenceCandidate{search: lower, text: intern(lower), length: len(runes)}
				for k := 1; k <= len(runes); k++ {
					candidate.suffixes = append(candidate.suffixes, intern(string(runes[k:])))
				}
				stage.candidates = append(stage.candidates, candidate)
			}
			sort.Slice(stage.candidates, func(i, j int) bool { return stage.candidates[i].search < stage.candidates[j].search })
			compiled.stages = append(compiled.stages, stage)
			previousActive = true
		}
		for i := range compiled.stages {
			stage := &compiled.stages[i]
			maxCandidates = max(maxCandidates, len(stage.candidates))
			if stage.reset {
				continue
			}
			prefixIDs := make(map[string]int)
			previous := &compiled.stages[i-1]
			for c := range previous.candidates {
				candidate := &previous.candidates[c]
				runes := []rune(candidate.search)
				for k := 1; k <= len(runes); k++ {
					prefix := string(runes[:k])
					id, ok := prefixIDs[prefix]
					if !ok {
						id = len(prefixIDs)
						prefixIDs[prefix] = id
					}
					candidate.prefixes = append(candidate.prefixes, id)
				}
			}
			// Separate incoming indexes from outgoing indexes: each edge has its own trie.
			for c := range stage.candidates {
				candidate := &stage.candidates[c]
				runes := []rune(candidate.search)
				for k := 1; k <= len(runes); k++ {
					id, ok := prefixIDs[string(runes[:k])]
					if !ok {
						id = -1
					}
					candidate.incoming = append(candidate.incoming, id)
				}
			}
			stage.prefixCount = len(prefixIDs)
			maxPrefixes = max(maxPrefixes, len(prefixIDs))
		}
		scorer.groups = append(scorer.groups, compiled)
	}
	scorer.costs = make([]float64, len(scorer.texts))
	scorer.stamps = make([]uint64, len(scorer.texts))
	scorer.previous = make([]float64, maxCandidates)
	scorer.next = make([]float64, maxCandidates)
	scorer.prefixCosts = make([]float64, maxPrefixes)
	scorer.compileTyping()
	return scorer
}

func (s *sequenceScorer) textCost(id int, bindings Layout) float64 {
	if s.stamps[id] != s.generation {
		cost, previous := 0.0, -1
		clear(s.fingerCounts)
		for _, character := range s.textCharacters[id] {
			key := s.characterKeys[character]
			if key < 0 {
				cost = math.Inf(1)
				break
			}
			cost += s.efforts[key]
			finger := s.fingers[key]
			if s.weights.FingerReusePenalty > 0 && finger >= 0 {
				cost += s.weights.FingerReusePenalty * float64(s.fingerCounts[finger])
				s.fingerCounts[finger]++
			}
			if previous >= 0 {
				pair := previous*len(s.efforts) + key
				if s.weights.DistancePenalty > 0 {
					cost += s.distances[pair]
				}
				cost += s.transitions[pair]
			}
			previous = key
		}
		s.costs[id], s.stamps[id] = cost, s.generation
	}
	return s.costs[id]
}

func (s *sequenceScorer) evaluate(bindings Layout, report bool) (float64, []GroupPlan) {
	if s.weights.SearchEditing == nil {
		return 0, nil
	}
	s.generation++
	for i, ch := range s.characters {
		key, assigned := bindings[ch]
		id, known := s.keyIDs[key]
		if !assigned || !known {
			id = -1
		}
		s.characterKeys[i] = id
	}
	total := 0.0
	var plans []GroupPlan
	for _, group := range s.groups {
		previous, next := s.previous, s.next
		plan := GroupPlan{Name: group.group.Name, Priority: group.group.Priority}
		var histories [][]sequenceChoice
		segmentCost := 0.0
		previousCount := 0
		for i, stage := range group.stages {
			if stage.reset && previousCount > 0 {
				segmentCost += minimumCost(previous[:previousCount])
			}
			minPrevious, minIndex := math.Inf(1), -1
			clearPrevious, clearIndex := math.Inf(1), -1
			prefixOwners := []int(nil)
			if !stage.reset {
				for c := 0; c < previousCount; c++ {
					if group.stages[i-1].candidates[c].length == 1 && previous[c] < clearPrevious {
						clearPrevious, clearIndex = previous[c], c
					}
					if previous[c] < minPrevious {
						minPrevious, minIndex = previous[c], c
					}
				}
				for p := 0; p < stage.prefixCount; p++ {
					s.prefixCosts[p] = math.Inf(1)
				}
				if report {
					prefixOwners = make([]int, stage.prefixCount)
				}
				for c, candidate := range group.stages[i-1].candidates {
					value := previous[c] + float64(candidate.length)*s.weights.SearchEditing.Backspace
					for k, prefix := range candidate.prefixes {
						// A transition may retain the full search or delete its last rune.
						// Multiple Backspaces are excluded, regardless of their numeric cost.
						if candidate.length-(k+1) > 1 {
							continue
						}
						if value < s.prefixCosts[prefix] {
							s.prefixCosts[prefix] = value
							if report {
								prefixOwners[prefix] = c
							}
						}
					}
				}
			}
			var choices []sequenceChoice
			if report {
				choices = make([]sequenceChoice, len(stage.candidates))
			}
			for c, candidate := range stage.candidates {
				fullCost := s.textCost(candidate.text, bindings)
				value := fullCost
				choice := sequenceChoice{previous: -1}
				if !stage.reset {
					value = minPrevious + s.weights.SearchEditing.ShiftHome + fullCost
					choice.previous, choice.replace = minIndex, true
					if clearCost := clearPrevious + s.weights.SearchEditing.Backspace + fullCost; clearCost < value {
						value = clearCost
						choice = sequenceChoice{previous: clearIndex}
					}
					// Require the entire resulting search to be typeable, including its
					// retained prefix; missing characters are never allowed via reuse.
					if !math.IsInf(fullCost, 1) {
						for k, prefix := range candidate.incoming {
							if prefix < 0 {
								continue
							}
							overlapCost := s.prefixCosts[prefix] - float64(k+1)*s.weights.SearchEditing.Backspace + s.textCost(candidate.suffixes[k], bindings)
							if overlapCost < value {
								value = overlapCost
								if report {
									choice = sequenceChoice{previous: prefixOwners[prefix], retained: k + 1}
								}
							}
						}
					}
				}
				next[c] = value
				if report {
					choices[c] = choice
				}
			}
			previous, next = next, previous
			previousCount = len(stage.candidates)
			if report {
				histories = append(histories, choices)
			}
		}
		plan.Cost = segmentCost + minimumCost(previous[:previousCount])
		total += group.group.Priority * plan.Cost
		if report && !math.IsInf(plan.Cost, 1) {
			plan.Steps = s.reconstruct(group, histories, bindings)
			plans = append(plans, plan)
		}
	}
	return total, plans
}

type sequenceChoice struct {
	previous int
	retained int
	replace  bool
}

func minimumCost(costs []float64) float64 {
	if len(costs) == 0 {
		return 0
	}
	best := math.Inf(1)
	for _, cost := range costs {
		best = math.Min(best, cost)
	}
	return best
}

// Reconstructing runs only for the final report, using the same costs as scoring.
func (s *sequenceScorer) reconstruct(group sequenceGroup, histories [][]sequenceChoice, bindings Layout) []CraftStep {
	steps := make([]CraftStep, len(group.stages))
	// Recover each independent segment by evaluating it with history tracking.
	for end := len(group.stages) - 1; end >= 0; {
		start := end
		for start > 0 && !group.stages[start].reset {
			start--
		}
		costs := make([]float64, len(group.stages[end].candidates))
		// Histories encode predecessor choices; replay their chosen edge costs.
		stageCosts := make([][]float64, end-start+1)
		for i := start; i <= end; i++ {
			stageCosts[i-start] = make([]float64, len(group.stages[i].candidates))
			for c, candidate := range group.stages[i].candidates {
				choice := histories[i][c]
				value := s.textCost(candidate.text, bindings)
				if choice.previous >= 0 {
					from := group.stages[i-1].candidates[choice.previous]
					value = stageCosts[i-start-1][choice.previous] + s.edgeCost(from, candidate, choice, bindings)
				}
				stageCosts[i-start][c] = value
			}
		}
		copy(costs, stageCosts[end-start])
		best := 0
		for c := range costs {
			if costs[c] < costs[best] {
				best = c
			}
		}
		for i := end; i >= start; i-- {
			candidate := group.stages[i].candidates[best]
			choice := histories[i][best]
			step := CraftStep{Item: group.stages[i].item, Search: candidate.search, Action: "type", Type: candidate.search, Cost: s.textCost(candidate.text, bindings)}
			if choice.previous >= 0 {
				from := group.stages[i-1].candidates[choice.previous]
				step.Cost = s.edgeCost(from, candidate, choice, bindings)
				if choice.replace {
					step.Action = "shift-home"
				} else {
					step.Backspaces = from.length - choice.retained
					step.Type = string([]rune(candidate.search)[choice.retained:])
					step.Action = "backspace"
					if step.Backspaces == 0 {
						step.Action = "type"
						if step.Type == "" {
							step.Action = "keep"
						}
					}
				}
			}
			steps[i], best = step, choice.previous
		}
		end = start - 1
	}
	return steps
}

func (s *sequenceScorer) edgeCost(from, to sequenceCandidate, choice sequenceChoice, bindings Layout) float64 {
	if choice.replace {
		return s.weights.SearchEditing.ShiftHome + s.textCost(to.text, bindings)
	}
	typed := to.text
	if choice.retained > 0 {
		typed = to.suffixes[choice.retained-1]
	}
	return float64(from.length-choice.retained)*s.weights.SearchEditing.Backspace + s.textCost(typed, bindings)
}

// PlanGroups finds the cheapest editing sequence for each active ordered group.
// Every segment starts with an empty search. Missing/ignored crafts break chains.
func PlanGroups(bindings Layout, goals []Goal, weights Weights) ([]GroupPlan, error) {
	goals, err := WithPreferredSearches(goals, nil)
	if err != nil {
		return nil, err
	}
	if weights.SearchEditing != nil {
		for _, value := range []float64{weights.SearchEditing.ShiftHome, weights.SearchEditing.Backspace} {
			if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
				return nil, fmt.Errorf("search edit costs must be finite and nonnegative")
			}
		}
	}
	scorer := newSequenceScorer(goals, weights)
	cost, plans := scorer.evaluate(bindings, true)
	if math.IsInf(cost, 1) {
		return nil, fmt.Errorf("group has no typeable search sequence")
	}
	return plans, nil
}

// compileTyping removes string maps, Unicode conversions and per-search
// allocations from the repeated DP evaluations.
func (s *sequenceScorer) compileTyping() {
	charIDs := make(map[rune]int)
	for _, text := range s.texts {
		var indices []int
		for _, ch := range text {
			id, ok := charIDs[ch]
			if !ok {
				id = len(s.characters)
				charIDs[ch] = id
				s.characters = append(s.characters, ch)
			}
			indices = append(indices, id)
		}
		s.textCharacters = append(s.textCharacters, indices)
	}
	s.characterKeys = make([]int, len(s.characters))
	keys := make([]string, 0, len(s.weights.Keys))
	for key := range s.weights.Keys {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	s.keyIDs = make(map[string]int, len(keys))
	fingerIDs := make(map[string]int)
	for i, key := range keys {
		s.keyIDs[key] = i
		weight := s.weights.Keys[key]
		s.efforts = append(s.efforts, weight.Effort)
		finger := -1
		if weight.Finger != "" {
			var ok bool
			finger, ok = fingerIDs[weight.Finger]
			if !ok {
				finger = len(fingerIDs)
				fingerIDs[weight.Finger] = finger
			}
		}
		s.fingers = append(s.fingers, finger)
	}
	s.fingerCounts = make([]int, len(fingerIDs))
	for _, from := range keys {
		for _, to := range keys {
			a, b := s.weights.Keys[from], s.weights.Keys[to]
			penalty, ok := s.weights.Transitions[Transition{From: from, To: to}]
			if !ok {
				penalty = s.weights.DefaultTransitionPenalty
				if b.Finger != "" && a.Finger == b.Finger {
					penalty += s.weights.SameFingerPenalty
				}
			}
			s.transitions = append(s.transitions, penalty)
			s.distances = append(s.distances, s.weights.DistancePenalty*math.Hypot(a.X-b.X, a.Y-b.Y))
		}
	}
}

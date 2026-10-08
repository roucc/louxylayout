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
	Item        string
	Search      string // complete search visible after this step
	Action      string // type, shift-home, backspace, or keep
	Backspaces  int
	Type        string // only the new text to type
	Cost        float64
	Layer       CraftLayer // typing and crafting-click layer; empty in single-layer mode
	LayerChange bool       // press or release Shift before this craft
}

type GroupPlan struct {
	Name     string
	Priority float64
	Cost     float64 // unweighted edit/typing cost
	Steps    []CraftStep
}

type sequenceCandidate struct {
	search              string
	text, length, stage int
	layer               int
	penalty             float64
	outgoing            []int // full search and search without its last rune
	incoming            []int // every prefix of this search
	suffixes            []int
}

type sequenceStage struct {
	item          string
	start, end    int
	prerequisites int   // required visited crafts, checked using the existing mask
	active        []int // typeable candidate indexes, reused across evaluations
}

type sequenceGroup struct {
	group       CraftGroup
	stages      []sequenceStage
	candidates  []sequenceCandidate
	prefixCount int
	dp          []float64
}

// Exact subset planning considers craft order and search choice together. The
// prefix minima avoid comparing every predecessor candidate with every next one.
const maxGroupCrafts = 10

type sequenceScorer struct {
	weights            Weights
	texts              []string
	groups             []sequenceGroup
	costs              []float64
	stamps             []uint64
	generation         uint64
	characters         []rune
	textCharacters     [][]int
	keyIDs             map[string]int
	characterKeys      []int
	efforts            []float64
	fingers            []int
	fingerCounts       []int
	transitions        []float64
	distances          []float64
	prefixCosts        []float64
	invalid            bool
	orderError         error
	layered            bool
	layerCharacterKeys [2][]int
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
	maxPrefixes := 0
	for _, group := range weights.Groups {
		if group.Priority == 0 {
			continue
		}
		if err := validateGroupOrder(group); err != nil {
			scorer.orderError = err
			return scorer
		}
		compiled := sequenceGroup{group: group}
		items := uniqueGroupItems(group.Crafts)
		for _, item := range items {
			goal, active := byItem[item]
			if !active {
				continue
			}
			stage := sequenceStage{item: item, start: len(compiled.candidates)}
			seen := make(map[string]bool)
			var candidates []sequenceCandidate
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
				candidate := sequenceCandidate{search: lower, text: intern(lower), length: len(runes), stage: len(compiled.stages)}
				for k := 1; k <= len(runes); k++ {
					candidate.suffixes = append(candidate.suffixes, intern(string(runes[k:])))
				}
				candidates = append(candidates, candidate)
			}
			sort.Slice(candidates, func(i, j int) bool { return candidates[i].search < candidates[j].search })
			compiled.candidates = append(compiled.candidates, candidates...)
			stage.end = len(compiled.candidates)
			stage.active = make([]int, 0, len(candidates))
			compiled.stages = append(compiled.stages, stage)
		}
		stageIndex := make(map[string]int, len(compiled.stages))
		for i, stage := range compiled.stages {
			stageIndex[stage.item] = i
		}
		for before, targets := range group.Before {
			from, active := stageIndex[before]
			if !active {
				continue
			}
			for _, after := range targets {
				if to, active := stageIndex[after]; active {
					compiled.stages[to].prerequisites |= 1 << from
				}
			}
		}
		if len(compiled.stages) > maxGroupCrafts {
			scorer.invalid = true
			return scorer
		}
		prefixes := make(map[string]int)
		for c := range compiled.candidates {
			candidate := &compiled.candidates[c]
			runes := []rune(candidate.search)
			for k := max(1, len(runes)-1); k <= len(runes); k++ {
				prefix := string(runes[:k])
				id, ok := prefixes[prefix]
				if !ok {
					id = len(prefixes)
					prefixes[prefix] = id
				}
				candidate.outgoing = append(candidate.outgoing, id)
			}
		}
		for c := range compiled.candidates {
			candidate := &compiled.candidates[c]
			runes := []rune(candidate.search)
			for k := 1; k <= len(runes); k++ {
				id, ok := prefixes[string(runes[:k])]
				if !ok {
					id = -1
				}
				candidate.incoming = append(candidate.incoming, id)
			}
		}
		compiled.prefixCount = len(prefixes)
		maxPrefixes = max(maxPrefixes, compiled.prefixCount)
		compiled.dp = make([]float64, (1<<len(compiled.stages))*len(compiled.candidates))
		scorer.groups = append(scorer.groups, compiled)
	}
	scorer.costs = make([]float64, len(scorer.texts))
	scorer.stamps = make([]uint64, len(scorer.texts))
	scorer.prefixCosts = make([]float64, maxPrefixes)
	scorer.compileTyping()
	return scorer
}

// Sorting and deduplication ensure configured list order cannot affect the plan.
func uniqueGroupItems(items []string) []string {
	seen := make(map[string]bool, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if !seen[item] {
			result = append(result, item)
			seen[item] = true
		}
	}
	sort.Strings(result)
	return result
}

func (s *sequenceScorer) textCost(id int, bindings Layout) float64 {
	if s.stamps[id] != s.generation {
		cost, previous := 0.0, -1
		clear(s.fingerCounts)
		keys := s.characterKeys
		if s.layered {
			keys = s.layerCharacterKeys[id/len(s.texts)]
		}
		for _, character := range s.textCharacters[id%len(s.texts)] {
			key := keys[character]
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
	if s.invalid || s.orderError != nil {
		return math.Inf(1), nil
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
	return s.evaluatePrepared(bindings, report)
}

func (s *sequenceScorer) evaluatePrepared(bindings Layout, report bool) (float64, []GroupPlan) {
	layerCount := 1
	if s.layered {
		layerCount = 2
	}
	total := 0.0
	var plans []GroupPlan
	for _, group := range s.groups {
		count := len(group.candidates)
		if len(group.stages) == 0 {
			if report {
				plans = append(plans, GroupPlan{Name: group.group.Name, Priority: group.group.Priority})
			}
			continue
		}
		for i := range group.dp {
			group.dp[i] = math.Inf(1)
		}
		var history []sequenceChoice
		var prefixOwners []int
		if report {
			history = make([]sequenceChoice, len(group.dp))
			prefixOwners = make([]int, layerCount*group.prefixCount)
		}
		for stage := range group.stages {
			craft := &group.stages[stage]
			craft.active = craft.active[:0]
			for c := craft.start; c < craft.end; c++ {
				if !math.IsInf(s.textCost(group.candidates[c].text, bindings), 1) {
					craft.active = append(craft.active, c)
				}
			}
		}
		for c, candidate := range group.candidates {
			if group.stages[candidate.stage].prerequisites != 0 {
				continue
			}
			state := (1<<candidate.stage)*count + c
			group.dp[state] = s.textCost(candidate.text, bindings) + candidate.penalty
			if candidate.layer == 1 {
				group.dp[state] += s.weights.LayerSwitchPenalty
			}
			if report {
				history[state] = sequenceChoice{previous: -1}
			}
		}
		fullMask := (1 << len(group.stages)) - 1
		for mask := 1; mask < fullMask; mask++ {
			minPrevious, clearPrevious := [2]float64{math.Inf(1), math.Inf(1)}, [2]float64{math.Inf(1), math.Inf(1)}
			minIndex, clearIndex := [2]int{-1, -1}, [2]int{-1, -1}
			for p := 0; p < layerCount*group.prefixCount; p++ {
				s.prefixCosts[p] = math.Inf(1)
			}
			for stage, craft := range group.stages {
				if mask&(1<<stage) == 0 {
					continue
				}
				for _, c := range craft.active {
					previous := group.dp[mask*count+c]
					if math.IsInf(previous, 1) {
						continue
					}
					candidate := group.candidates[c]
					for targetLayer := 0; targetLayer < layerCount; targetLayer++ {
						adjusted := previous
						if targetLayer != candidate.layer {
							adjusted += s.weights.LayerSwitchPenalty
						}
						if adjusted < minPrevious[targetLayer] {
							minPrevious[targetLayer], minIndex[targetLayer] = adjusted, c
						}
						if candidate.length == 1 && adjusted < clearPrevious[targetLayer] {
							clearPrevious[targetLayer], clearIndex[targetLayer] = adjusted, c
						}
						value := adjusted + float64(candidate.length)*s.weights.SearchEditing.Backspace
						for _, prefix := range candidate.outgoing {
							index := targetLayer*group.prefixCount + prefix
							if value < s.prefixCosts[index] {
								s.prefixCosts[index] = value
								if report {
									prefixOwners[index] = c
								}
							}
						}
					}
				}
			}
			if minIndex[0] < 0 {
				continue
			}
			for stage, craft := range group.stages {
				if mask&(1<<stage) != 0 {
					continue
				}
				if mask&craft.prerequisites != craft.prerequisites {
					continue
				}
				nextMask := mask | (1 << stage)
				for _, c := range craft.active {
					candidate := group.candidates[c]
					fullCost := s.textCost(candidate.text, bindings)
					if math.IsInf(fullCost, 1) {
						continue
					}
					value := minPrevious[candidate.layer] + s.weights.SearchEditing.ShiftHome + fullCost
					choice := sequenceChoice{previous: minIndex[candidate.layer], replace: true}
					if clearCost := clearPrevious[candidate.layer] + s.weights.SearchEditing.Backspace + fullCost; clearCost < value {
						value = clearCost
						choice = sequenceChoice{previous: clearIndex[candidate.layer]}
					}
					for k, prefix := range candidate.incoming {
						if prefix < 0 {
							continue
						}
						prefix += candidate.layer * group.prefixCount
						overlap := s.prefixCosts[prefix] - float64(k+1)*s.weights.SearchEditing.Backspace + s.textCost(candidate.suffixes[k], bindings)
						if overlap < value {
							value = overlap
							if report {
								choice = sequenceChoice{previous: prefixOwners[prefix], retained: k + 1}
							}
						}
					}
					value += candidate.penalty
					state := nextMask*count + c
					if value < group.dp[state] {
						group.dp[state] = value
						if report {
							history[state] = choice
						}
					}
				}
			}
		}
		best, bestIndex := math.Inf(1), -1
		for c := 0; c < count; c++ {
			if value := group.dp[fullMask*count+c]; value < best {
				best, bestIndex = value, c
			}
		}
		total += group.group.Priority * best
		if report && bestIndex >= 0 {
			plan := GroupPlan{Name: group.group.Name, Priority: group.group.Priority, Cost: best}
			for mask, c := fullMask, bestIndex; mask > 0; {
				candidate := group.candidates[c]
				choice := history[mask*count+c]
				step := CraftStep{Item: group.stages[candidate.stage].item, Search: candidate.search, Action: "type", Type: candidate.search, Cost: s.textCost(candidate.text, bindings) + candidate.penalty}
				if s.layered {
					step.Layer = layerName(candidate.layer)
					step.LayerChange = candidate.layer == 1
					if step.LayerChange {
						step.Cost += s.weights.LayerSwitchPenalty
					}
				}
				if choice.previous >= 0 {
					from := group.candidates[choice.previous]
					step.Cost = s.edgeCost(from, candidate, choice, bindings)
					if s.layered {
						step.LayerChange = from.layer != candidate.layer
					}
					if choice.replace {
						step.Action = "shift-home"
					} else {
						step.Backspaces = from.length - choice.retained
						step.Type = string([]rune(candidate.search)[choice.retained:])
						if step.Backspaces > 0 {
							step.Action = "backspace"
						} else if step.Type == "" {
							step.Action = "keep"
						}
					}
				}
				plan.Steps = append(plan.Steps, step)
				mask ^= 1 << candidate.stage
				c = choice.previous
			}
			for i, j := 0, len(plan.Steps)-1; i < j; i, j = i+1, j-1 {
				plan.Steps[i], plan.Steps[j] = plan.Steps[j], plan.Steps[i]
			}
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

func (s *sequenceScorer) edgeCost(from, to sequenceCandidate, choice sequenceChoice, bindings Layout) float64 {
	cost := to.penalty
	if from.layer != to.layer {
		cost += s.weights.LayerSwitchPenalty
	}
	if choice.replace {
		return cost + s.weights.SearchEditing.ShiftHome + s.textCost(to.text, bindings)
	}
	typed := to.text
	if choice.retained > 0 {
		typed = to.suffixes[choice.retained-1]
	}
	return cost + float64(from.length-choice.retained)*s.weights.SearchEditing.Backspace + s.textCost(typed, bindings)
}

// PlanGroups jointly chooses craft order, searches and single-Backspace edits.
// Missing and ignored crafts are omitted; every included craft is visited once.
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
	if scorer.orderError != nil {
		return nil, scorer.orderError
	}
	if scorer.invalid {
		return nil, fmt.Errorf("groups support at most %d active unique crafts", maxGroupCrafts)
	}
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

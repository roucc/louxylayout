package layout

import (
	"fmt"
	"math"
)

func newLayerSequenceScorer(goals []Goal, weights Weights) *sequenceScorer {
	scorer := newSequenceScorer(goals, weights)
	scorer.layered = true
	if weights.SearchEditing == nil || scorer.invalid || scorer.orderError != nil {
		return scorer
	}
	count := len(scorer.texts)
	scorer.costs = make([]float64, 2*count)
	scorer.stamps = make([]uint64, 2*count)
	for layer := 0; layer < 2; layer++ {
		scorer.layerCharacterKeys[layer] = make([]int, len(scorer.characters))
	}
	scorer.prefixCosts = make([]float64, 2*len(scorer.prefixCosts))
	for g := range scorer.groups {
		group := &scorer.groups[g]
		original := group.candidates
		var candidates []sequenceCandidate
		for stage := range group.stages {
			craft := &group.stages[stage]
			start, end := craft.start, craft.end
			craft.start = len(candidates)
			for layer := 0; layer < 2; layer++ {
				for c := start; c < end; c++ {
					candidate := original[c]
					candidate.layer = layer
					candidate.penalty = craftImportance(craft.item, weights) * layerPenalty(craft.item, layerName(layer), weights)
					candidate.text += layer * count
					candidate.suffixes = append([]int(nil), candidate.suffixes...)
					for i := range candidate.suffixes {
						candidate.suffixes[i] += layer * count
					}
					candidates = append(candidates, candidate)
				}
			}
			craft.end = len(candidates)
			craft.active = make([]int, 0, craft.end-craft.start)
		}
		group.candidates = candidates
		group.dp = make([]float64, (1<<len(group.stages))*len(candidates))
	}
	return scorer
}

func (s *sequenceScorer) evaluateLayers(bindings LayeredLayout, report bool) (float64, []GroupPlan) {
	if s.weights.SearchEditing == nil {
		return 0, nil
	}
	if s.invalid || s.orderError != nil {
		return math.Inf(1), nil
	}
	s.generation++
	for layer, assigned := range []Layout{bindings.NonShift, bindings.Shift} {
		for i, ch := range s.characters {
			key, present := assigned[ch]
			id, known := s.keyIDs[key]
			if !present || !known {
				id = -1
			}
			s.layerCharacterKeys[layer][i] = id
		}
	}
	return s.evaluatePrepared(nil, report)
}

// PlanLayeredGroups chooses craft order, searches, layers and editing actions.
func PlanLayeredGroups(bindings LayeredLayout, goals []Goal, weights Weights) ([]GroupPlan, error) {
	goals, err := WithPreferredSearches(goals, nil)
	if err != nil {
		return nil, err
	}
	if err := validateLayerWeights(weights); err != nil {
		return nil, err
	}
	scorer := newLayerSequenceScorer(goals, weights)
	if scorer.orderError != nil {
		return nil, scorer.orderError
	}
	if scorer.invalid {
		return nil, fmt.Errorf("groups support at most %d active unique crafts", maxGroupCrafts)
	}
	cost, plans := scorer.evaluateLayers(bindings, true)
	if math.IsInf(cost, 1) {
		return nil, fmt.Errorf("group has no typeable layered search sequence")
	}
	return plans, nil
}

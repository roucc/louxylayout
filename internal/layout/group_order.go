package layout

import "fmt"

// validateGroupOrder rejects misspelled members and dependency cycles once,
// before optimization. Precedence is about visits, not recipe inventory state.
func validateGroupOrder(group CraftGroup) error {
	members := uniqueGroupItems(group.Crafts)
	index := make(map[string]int, len(members))
	for i, item := range members {
		index[item] = i
	}
	edges := make([][]int, len(members))
	degree := make([]int, len(members))
	for before, targets := range group.Before {
		from, ok := index[before]
		if !ok {
			return fmt.Errorf("group %q order rule refers to non-member %q", group.Name, before)
		}
		seen := make(map[string]bool, len(targets))
		for _, after := range targets {
			to, ok := index[after]
			if !ok {
				return fmt.Errorf("group %q order rule refers to non-member %q", group.Name, after)
			}
			if seen[after] {
				continue
			}
			seen[after] = true
			edges[from] = append(edges[from], to)
			degree[to]++
		}
	}
	var ready []int
	for i, count := range degree {
		if count == 0 {
			ready = append(ready, i)
		}
	}
	for head := 0; head < len(ready); head++ {
		for _, to := range edges[ready[head]] {
			degree[to]--
			if degree[to] == 0 {
				ready = append(ready, to)
			}
		}
	}
	if len(ready) != len(members) {
		return fmt.Errorf("group %q order rules contain a cycle", group.Name)
	}
	return nil
}

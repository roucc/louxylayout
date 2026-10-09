// Package profile loads independent personal configurations from JSON files.
package profile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"louxylayout/internal/layout"
	"louxylayout/profiles"
)

// Profile is self-contained; omitted personal maps do not inherit from another
// person's profile. Optimizer controls have the same defaults as the CLI.
type Profile struct {
	Name                   string              `json:"name"`
	Language               string              `json:"language"`
	AllowGoodJunk          bool                `json:"allow_good_junk"`
	Inventory              []string            `json:"inventory"`
	Goals                  []string            `json:"goals"`
	PreferredSearches      map[string][]string `json:"preferred_searches"`
	PreferredBindings      map[string]string   `json:"preferred_bindings"`
	PreferredShiftBindings map[string]string   `json:"preferred_shift_bindings"`
	FixedBindings          map[string]string   `json:"fixed_bindings"`
	FixedShiftBindings     map[string]string   `json:"fixed_shift_bindings"`
	Weights                Weights             `json:"weights"`
	Optimizer              Optimizer           `json:"optimizer"`
}

type Optimizer struct {
	Seed             *int64 `json:"seed"`
	Restarts         int    `json:"restarts"`
	Iterations       int    `json:"iterations"`
	EnableShiftLayer *bool  `json:"enable_shift_layer"`
}

type Weights struct {
	Keys                     map[string]KeyWeight `json:"keys"`
	Crafts                   map[string]float64   `json:"crafts"`
	PreferredLayers          map[string]string    `json:"preferred_layers"`
	LayerPreferencePenalty   float64              `json:"layer_preference_penalty"`
	LayerSwitchPenalty       float64              `json:"layer_switch_penalty"`
	PreferBothLayers         map[string]float64   `json:"prefer_both_layers"`
	Groups                   []Group              `json:"groups"`
	SearchEditing            *SearchEditing       `json:"search_editing"`
	DistancePenalty          float64              `json:"distance_penalty"`
	FingerReusePenalty       float64              `json:"finger_reuse_penalty"`
	SameFingerPenalty        float64              `json:"same_finger_penalty"`
	DefaultTransitionPenalty float64              `json:"default_transition_penalty"`
	Transitions              []Transition         `json:"transitions"`
}

type KeyWeight struct {
	Effort float64 `json:"effort"`
	Finger string  `json:"finger"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
}

type Group struct {
	Name     string              `json:"name"`
	Priority float64             `json:"priority"`
	Crafts   []string            `json:"crafts"`
	Before   map[string][]string `json:"before,omitempty"`
}

type SearchEditing struct {
	ShiftHome float64 `json:"shift_home"`
	Backspace float64 `json:"backspace"`
}

type Transition struct {
	From string  `json:"from"`
	To   string  `json:"to"`
	Cost float64 `json:"cost"`
}

// Load accepts a profile name (profiles/<name>.json), or an explicit JSON path.
// Named local files override bundled copies. Explicit paths never fall back.
func Load(nameOrPath string) (Profile, error) {
	var data []byte
	var err error
	if nameOrPath == "" {
		return Profile{}, fmt.Errorf("profile name or path is required")
	}
	path := nameOrPath
	if strings.ContainsAny(nameOrPath, "/\\") || strings.HasSuffix(nameOrPath, ".json") {
		data, err = os.ReadFile(path)
	} else {
		path = filepath.Join("profiles", nameOrPath+".json")
		data, err = os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			data, err = profiles.Files.ReadFile(nameOrPath + ".json")
		}
	}
	if err != nil {
		return Profile{}, fmt.Errorf("load profile %q: %w", nameOrPath, err)
	}
	p, err := Parse(data)
	if err != nil {
		return Profile{}, fmt.Errorf("profile %q: %w", path, err)
	}
	if p.Name == "" {
		p.Name = strings.TrimSuffix(filepath.Base(path), ".json")
	}
	return p, nil
}

func Parse(data []byte) (Profile, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var p Profile
	if err := decoder.Decode(&p); err != nil {
		return Profile{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple JSON values")
		}
		return Profile{}, err
	}
	if p.Language == "" {
		return Profile{}, fmt.Errorf("language is required")
	}
	if len(p.Goals) == 0 {
		return Profile{}, fmt.Errorf("at least one goal is required")
	}
	seen := make(map[string]bool, len(p.Goals))
	for _, goal := range p.Goals {
		if goal == "" || seen[goal] {
			return Profile{}, fmt.Errorf("goals must be nonempty and distinct: %q", goal)
		}
		seen[goal] = true
	}
	if len(p.Weights.Keys) == 0 {
		return Profile{}, fmt.Errorf("weights.keys must define the physical keyboard")
	}
	if p.Optimizer.Restarts < 0 || p.Optimizer.Iterations < 0 {
		return Profile{}, fmt.Errorf("optimizer restarts and iterations must be nonnegative")
	}
	if _, _, err := p.Settings(); err != nil {
		return Profile{}, err
	}
	return p, nil
}

// Character uses a literal character, including Unicode and spaces. "Space" is
// also accepted for convenience. Search strings are never trimmed or normalized.
func character(value string) (rune, error) {
	if value == "Space" {
		return ' ', nil
	}
	chars := []rune(value)
	if len(chars) != 1 {
		return 0, fmt.Errorf("expected one typed character or Space, got %q", value)
	}
	return unicode.ToLower(chars[0]), nil
}

func bindings(values map[string]string, keys map[string]KeyWeight) (map[string]rune, error) {
	result := make(map[string]rune, len(values))
	used := make(map[rune]string, len(values))
	for key, value := range values {
		if _, known := keys[key]; !known {
			return nil, fmt.Errorf("binding refers to unknown physical key %q", key)
		}
		ch, err := character(value)
		if err != nil {
			return nil, fmt.Errorf("binding for %q: %w", key, err)
		}
		if previous, duplicate := used[ch]; duplicate {
			return nil, fmt.Errorf("typed character %q is assigned to both %q and %q in one layer", ch, previous, key)
		}
		used[ch], result[key] = key, ch
	}
	return result, nil
}

func fixedBindings(values map[string]string, keys map[string]KeyWeight) (layout.Layout, error) {
	if values == nil {
		return nil, nil
	}
	decoded, err := bindings(values, keys)
	if err != nil {
		return nil, err
	}
	result := make(layout.Layout, len(decoded))
	for key, ch := range decoded {
		result[ch] = key
	}
	return result, nil
}

// Settings converts JSON-friendly maps and arrays into optimizer types.
func (p Profile) Settings() (layout.Weights, layout.OptimizeOptions, error) {
	w := layout.Weights{
		Keys: make(map[string]layout.KeyWeight, len(p.Weights.Keys)), Crafts: p.Weights.Crafts,
		PreferredLayers: make(map[string]layout.CraftLayer, len(p.Weights.PreferredLayers)), PreferBothLayers: make(map[rune]float64, len(p.Weights.PreferBothLayers)),
		LayerPreferencePenalty: p.Weights.LayerPreferencePenalty, LayerSwitchPenalty: p.Weights.LayerSwitchPenalty,
		DistancePenalty: p.Weights.DistancePenalty, FingerReusePenalty: p.Weights.FingerReusePenalty, SameFingerPenalty: p.Weights.SameFingerPenalty, DefaultTransitionPenalty: p.Weights.DefaultTransitionPenalty,
		Transitions: make(map[layout.Transition]float64, len(p.Weights.Transitions)),
	}
	for key, weight := range p.Weights.Keys {
		w.Keys[key] = layout.KeyWeight{Effort: weight.Effort, Finger: weight.Finger, X: weight.X, Y: weight.Y}
	}
	for item, value := range p.Weights.PreferredLayers {
		if value == "any" {
			value = ""
		}
		layer := layout.CraftLayer(value)
		if layer != layout.AnyLayer && layer != layout.NonShiftLayer && layer != layout.ShiftLayer {
			return w, layout.OptimizeOptions{}, fmt.Errorf("invalid preferred layer %q for %q", value, item)
		}
		w.PreferredLayers[item] = layer
	}
	for value, penalty := range p.Weights.PreferBothLayers {
		ch, err := character(value)
		if err != nil {
			return w, layout.OptimizeOptions{}, fmt.Errorf("prefer_both_layers: %w", err)
		}
		if _, duplicate := w.PreferBothLayers[ch]; duplicate {
			return w, layout.OptimizeOptions{}, fmt.Errorf("duplicate shared character preference for %q", ch)
		}
		w.PreferBothLayers[ch] = penalty
	}
	for _, group := range p.Weights.Groups {
		w.Groups = append(w.Groups, layout.CraftGroup{Name: group.Name, Priority: group.Priority, Crafts: group.Crafts, Before: group.Before})
	}
	if p.Weights.SearchEditing != nil {
		w.SearchEditing = &layout.SearchEditCosts{ShiftHome: p.Weights.SearchEditing.ShiftHome, Backspace: p.Weights.SearchEditing.Backspace}
	}
	for _, transition := range p.Weights.Transitions {
		pair := layout.Transition{From: transition.From, To: transition.To}
		if _, known := w.Keys[pair.From]; !known {
			return w, layout.OptimizeOptions{}, fmt.Errorf("transition refers to unknown physical key %q", pair.From)
		}
		if _, known := w.Keys[pair.To]; !known {
			return w, layout.OptimizeOptions{}, fmt.Errorf("transition refers to unknown physical key %q", pair.To)
		}
		if _, duplicate := w.Transitions[pair]; duplicate {
			return w, layout.OptimizeOptions{}, fmt.Errorf("duplicate transition %s -> %s", pair.From, pair.To)
		}
		w.Transitions[pair] = transition.Cost
	}
	options := layout.OptimizeOptions{Seed: 42, Restarts: p.Optimizer.Restarts, Iterations: p.Optimizer.Iterations, EnableShiftLayer: true}
	if p.Optimizer.Seed != nil {
		options.Seed = *p.Optimizer.Seed
	}
	if p.Optimizer.EnableShiftLayer != nil {
		options.EnableShiftLayer = *p.Optimizer.EnableShiftLayer
	}
	var err error
	options.Fixed, err = fixedBindings(p.FixedBindings, p.Weights.Keys)
	if err != nil {
		return w, options, fmt.Errorf("fixed_bindings: %w", err)
	}
	options.FixedShift, err = fixedBindings(p.FixedShiftBindings, p.Weights.Keys)
	if err != nil {
		return w, options, fmt.Errorf("fixed_shift_bindings: %w", err)
	}
	options.InitialBindings, err = bindings(p.PreferredBindings, p.Weights.Keys)
	if err != nil {
		return w, options, fmt.Errorf("preferred_bindings: %w", err)
	}
	options.InitialShiftBindings, err = bindings(p.PreferredShiftBindings, p.Weights.Keys)
	if err != nil {
		return w, options, fmt.Errorf("preferred_shift_bindings: %w", err)
	}
	shiftFixed := options.FixedShift
	if shiftFixed == nil && options.EnableShiftLayer {
		if key, locked := options.Fixed[' ']; locked {
			shiftFixed = layout.Layout{' ': key}
		}
	}
	for _, layer := range []struct {
		fixed     layout.Layout
		preferred map[string]rune
	}{{options.Fixed, options.InitialBindings}, {shiftFixed, options.InitialShiftBindings}} {
		for key, ch := range layer.preferred {
			if fixedKey, locked := layer.fixed[ch]; locked && fixedKey != key {
				return w, options, fmt.Errorf("preferred binding for %q conflicts with fixed key %q", ch, fixedKey)
			}
			for fixedCh, fixedKey := range layer.fixed {
				if fixedKey == key && fixedCh != ch {
					return w, options, fmt.Errorf("preferred key %q conflicts with fixed character %q", key, fixedCh)
				}
			}
		}
	}
	if err := layout.ValidateWeights(w); err != nil {
		return w, options, err
	}
	return w, options, nil
}

// List lists bundled profiles plus JSON files in the local profiles directory.
func List() ([]string, error) {
	seen := make(map[string]bool)
	bundled, err := profiles.Files.ReadDir(".")
	if err != nil {
		return nil, err
	}
	local, err := os.ReadDir("profiles")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	for _, entries := range [][]os.DirEntry{bundled, local} {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
				seen[strings.TrimSuffix(entry.Name(), ".json")] = true
			}
		}
	}
	result := make([]string, 0, len(seen))
	for name := range seen {
		result = append(result, name)
	}
	sort.Strings(result)
	return result, nil
}

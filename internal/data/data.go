package data

import (
	"embed"
	"encoding/json"
	"fmt"
)

type Recipe struct {
	Output      string     `json:"output"`
	Size        int        `json:"size"`
	Ingredients [][]string `json:"ingredients"`
}

//go:embed recipes.json lang/*.json
var files embed.FS

func Load(language string) ([][]Recipe, map[string]string, error) {
	recipesJSON, err := files.ReadFile("recipes.json")
	if err != nil {
		return nil, nil, err
	}
	itemsJSON, err := files.ReadFile("lang/" + language + ".json")
	if err != nil {
		return nil, nil, fmt.Errorf("load language %q: %w", language, err)
	}
	var groups [][]Recipe
	if err := json.Unmarshal(recipesJSON, &groups); err != nil {
		return nil, nil, fmt.Errorf("decode recipes: %w", err)
	}
	var items map[string]string
	if err := json.Unmarshal(itemsJSON, &items); err != nil {
		return nil, nil, fmt.Errorf("decode language %q: %w", language, err)
	}
	return groups, items, nil
}

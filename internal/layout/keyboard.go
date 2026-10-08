package layout

var KeyboardWeights = Weights{
	// Craft priority: 5 essential, 3 very important, 2 regular, 1 occasional,
	// 0.5 rare, 0 ignored. Unlisted crafts default to 1.
	Crafts: map[string]float64{
		"block.minecraft.white_bed":      5,
		"block.minecraft.respawn_anchor": 5,
		"item.minecraft.bow":             5,
		"item.minecraft.golden_pickaxe":  5,
		"item.minecraft.iron_axe":        4,
		"item.minecraft.stone_axe":       4,
		"item.minecraft.iron_sword":      4,
		"item.minecraft.stone_sword":     4,
		"item.minecraft.gold_ingot":      3,
		"item.minecraft.iron_ingot":      3,
		"block.minecraft.white_wool":     3,
		"block.minecraft.glowstone":      3,
		"block.minecraft.nether_bricks":  3,
		"item.minecraft.stick":           3,
		"item.minecraft.golden_carrot":   2,
		"item.minecraft.golden_helmet":   2,
		"item.minecraft.flint_and_steel": 2,
		"item.minecraft.bread":           2,
		"item.minecraft.iron_pickaxe":    2,
		"item.minecraft.stone_hoe":       2,
		"item.minecraft.blaze_powder":    1,
		"item.minecraft.ender_eye":       1,
		"item.minecraft.bucket":          1,
		"item.minecraft.diamond_sword":   0.5,
		"item.minecraft.stone_shovel":    0.5,
		"item.minecraft.oak_boat":        0.5,
	},
	// Higher group priority pulls consecutive crafts' search keys closer together.
	// Add sequences here using the same item IDs as Crafts above.
	// Existing controls: Shift+Home on MB4 and Backspace on MB5.
	// Costs are ergonomic estimates; these controls are not rebound by Optimize.
	SearchEditing: &SearchEditCosts{ShiftHome: 1, Backspace: 0.5},
	Groups: []CraftGroup{
		{
			Name:     "fortress",
			Priority: 4,
			Crafts: []string{
				"item.minecraft.bow",
				"block.minecraft.respawn_anchor",
				"block.minecraft.white_bed",
				"item.minecraft.golden_pickaxe",
			},
		},
		{
			Name:     "bastion",
			Priority: 3,
			Crafts: []string{
				"block.minecraft.white_bed",
				"item.minecraft.iron_ingot",
				"item.minecraft.iron_axe",
				"item.minecraft.iron_sword",
				"item.minecraft.stone_axe",
				"item.minecraft.stone_sword",
			},
		},
		{
			Name:     "2x2",
			Priority: 2,
			Crafts: []string{
				"block.minecraft.white_wool",
				"block.minecraft.nether_bricks",
				"block.minecraft.glowstone",
				"item.minecraft.stick",
			},
		},
		{
			Name:     "eyes",
			Priority: 1,
			Crafts: []string{
				"item.minecraft.blaze_powder",
				"item.minecraft.ender_eye",
			},
		},
		{
			Name:     "ow1",
			Priority: 1,
			Crafts: []string{
				"item.minecraft.stone_shovel",
				"item.minecraft.stone_axe",
				"item.minecraft.stone_hoe",
			},
		},
		{
			Name:     "ow2",
			Priority: 1,
			Crafts: []string{
				"item.minecraft.bucket",
				"item.minecraft.flint_and_steel",
				"item.minecraft.oak_boat",
			},
		},
	},
	Keys: map[string]KeyWeight{
		// higher weighting = harder to press
		"Space": {Effort: 1, Finger: "thumb", X: 3, Y: 4},
		"A":     {Effort: 1, Finger: "pinky", X: 0.25, Y: 2},
		"S":     {Effort: 1.2, Finger: "middle", X: 1.25, Y: 2},
		"D":     {Effort: 1, Finger: "index", X: 2.25, Y: 2},
		"W":     {Effort: 1, Finger: "middle", X: 1, Y: 1},
		"Q":     {Effort: 1.4, Finger: "ring", X: 0, Y: 1},
		"E":     {Effort: 1.5, Finger: "index", X: 2, Y: 1},
		"R":     {Effort: 1.6, Finger: "index", X: 3, Y: 1},
		"F":     {Effort: 1.5, Finger: "index", X: 3.25, Y: 2},
		"G":     {Effort: 2.5, Finger: "index", X: 4.25, Y: 2},
		"Z":     {Effort: 2, Finger: "ring", X: 0.75, Y: 3},
		"X":     {Effort: 2, Finger: "middle", X: 1.75, Y: 3},
		"C":     {Effort: 1.5, Finger: "index", X: 2.75, Y: 3},
		"V":     {Effort: 2.5, Finger: "index", X: 3.75, Y: 3},
		"1":     {Effort: 2.5, Finger: "middle", X: 0, Y: 0},
		"2":     {Effort: 1.8, Finger: "middle", X: 1, Y: 0},
		"3":     {Effort: 2, Finger: "index", X: 2, Y: 0},
		"4":     {Effort: 2, Finger: "index", X: 3, Y: 0},
	},
	// Within-craft comfort: raise these to prefer compact searches and more
	// finger variety. Zero disables either extra penalty.
	DistancePenalty:          1,
	FingerReusePenalty:       1,
	SameFingerPenalty:        1,
	DefaultTransitionPenalty: 0.5,
	Transitions: map[Transition]float64{
		// how easy it is to go from one key to another
		{From: "1", To: "2"}: 0.2,
		{From: "2", To: "1"}: 0.7,
		{From: "2", To: "3"}: 0.2,
		{From: "3", To: "2"}: 0.2,
		{From: "3", To: "4"}: 0.3,
		{From: "4", To: "3"}: 0.3,
		{From: "A", To: "S"}: 0.1,
		{From: "S", To: "A"}: 0.1,
		{From: "S", To: "D"}: 0.1,
		{From: "D", To: "S"}: 0.4,
		{From: "D", To: "F"}: 0.2,
		{From: "F", To: "D"}: 0.2,
		{From: "A", To: "W"}: 0.2,
		{From: "W", To: "A"}: 0.1,
		{From: "W", To: "D"}: 0.1,
		{From: "D", To: "W"}: 0.1,
		{From: "Q", To: "W"}: 0.3,
		{From: "W", To: "Q"}: 0.7,
		{From: "W", To: "E"}: 0.1,
		{From: "E", To: "W"}: 0.1,
		{From: "E", To: "R"}: 0.3,
		{From: "R", To: "E"}: 0.4,
	},
}

// PreferredBindings seeds the first optimizer restart; these keys can move.
// Maps physical key -> typed character (uppercase or lowercase).
// Leave empty for random starts.
var PreferredBindings = map[string]rune{
	// "A": 'N',
	// "Q": 'L', "W": 'H', "E": 'A',
}

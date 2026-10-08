package layout

// CraftLayer expresses the desired Shift state when clicking a craft result.
// Layer preferences are soft costs, not assignment restrictions.
type CraftLayer string

const (
	AnyLayer      CraftLayer = ""          // no preference
	NonShiftLayer CraftLayer = "non-shift" // ordinary click, e.g. sticks
	ShiftLayer    CraftLayer = "shift"     // shift-click to craft all, e.g. beds
)

// LayeredLayout assigns typed characters independently on each physical layer.
// The same character may occur on both layers, on the same or different keys.
type LayeredLayout struct {
	NonShift Layout
	Shift    Layout
}

func (l LayeredLayout) Bindings(layer CraftLayer) Layout {
	if layer == ShiftLayer {
		return l.Shift
	}
	return l.NonShift
}

func layerName(index int) CraftLayer {
	if index == 1 {
		return ShiftLayer
	}
	return NonShiftLayer
}

// LayeredSearch is a complete search typed while holding its chosen layer.
// That Shift state is retained for the crafting click.
type LayeredSearch struct {
	Item   string
	Search string
	Layer  CraftLayer
	Keys   []string
	Cost   float64 // typing, layer preference, and initial Shift press, before priority
}

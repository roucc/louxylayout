# TODO:
shift layer

# Louxy Layout

A Go tool for optimizing Minecraft search-crafting keyboard layouts.

Run the layout optimizer with `go run .`, or build an executable with `go build .`.
The report shows the minimum character set, optimized physical key bindings,
weighted cost, and the selected search for each craft. Lower cost is better.

## Structure

- `main.go`: loads data, constructs the search engine, and prints results.
- `internal/config/defaults.go`: language, inventory, and crafting goals.
- `internal/data/data.go`: recipe types and embedded JSON loading.
- `internal/data/recipes.json`: recipe groups and ingredients.
- `internal/data/lang/`: language JSON and reference tooltip JavaScript.
- `internal/search/search.go`: search state and constructor.
- `internal/search/craftable.go`: inventory checks and craftability indexes.
- `internal/search/groups.go`: group matching and visibility.
- `internal/search/substring.go`: substring generation, pure searches, and good junk.
- `internal/search/find.go`: direct item-name lookup helpers.

Add search helpers in more files with `package search` in `internal/search`.
Layout selection and scoring live in `internal/layout`:

- `layout.go`: exact minimum-character selection and random assignments.
- `keyboard.go`: editable key efforts, finger assignments, transition penalties,
  craft priorities, and ordered craft groups. Unlisted craft priorities default to 1; zero ignores a craft.
- `cost.go`: combines weighted typing costs with craft-group proximity costs.
- `optimize.go`: tries character swaps, moves, additions, replacements, and removals
  across random starts. This is a heuristic search, not a global optimum guarantee.

Tune `OptimizeOptions` in `main.go` for the seed, restarts, and iterations.
Space remains bound to the physical Space key. The minimum character set seeds
optimization; all original valid candidates remain available, allowing larger sets.

`config.AllowGoodJunk` defaults to false, rejecting avoidable matches with other
goal crafts. Unavoidable matches remain allowed in either mode. Candidate filtering
uses the 2x2 inventory grid when the target fits, otherwise the 3x3 table. Set
`Search.GridSize` to 2 or 3 to override the context.

Set `config.Language` to `"en_gb"` for English, `"no_no"` for Norwegian, or
`"ovd"` for Elfdalian. Language JSON is embedded
at build time. Search state is constructed explicitly with `search.New`,
allowing separate inventories, languages, and goal lists. Treat the recipe
groups and item-name map passed to the constructor as read-only.

## Craft groups

Add ordered sequences to `KeyboardWeights.Groups` in `internal/layout/keyboard.go`:

```go
Groups: []CraftGroup{
    {
        Name: "fortress",
        Priority: 3,
        Crafts: []string{
            "item.minecraft.bow",
            "block.minecraft.respawn_anchor",
            "block.minecraft.white_bed",
            "item.minecraft.golden_pickaxe",
        },
    },
},
```

Higher `Priority` gives the optimizer more incentive to put consecutive crafts'
search keys nearby. Zero disables a group. Crafts can appear in multiple groups;
their penalties add together. Individual `Crafts` priorities still control typing
effort independently. Groups use existing goals; they do not add crafting goals.
Absent goals and crafts with individual priority zero skip their incident pairs
without joining their neighbours.

Each craft uses its cheapest search among its allowed candidates (ties choose alphabetically). For each
consecutive pair in a group, the proximity penalty is `Priority` times the average
Euclidean distance between all keys in the two searches. `KeyWeight.X` and `Y`
define physical positions in key-width units, including approximate row stagger.
A priority of 3 therefore adds 3 cost units for a pair of single-key searches one
key-width apart. This is a soft preference: shared letters and typing effort can
prevent every group from fitting together. Search choices minimize typing effort among allowed searches;
group proximity then guides physical key assignment.

## Comfort within a craft

Tune these values in `KeyboardWeights` (both default to 1):

```go
DistancePenalty:    1,
FingerReusePenalty: 1,
```

`DistancePenalty` charges per key-width travelled between consecutive presses,
so a jump from C to 2 costs more than C to X. `FingerReusePenalty` charges for
all pairs of presses using the same named finger in a search: X → C → 2 reuses
the middle finger, even though those presses are separated. Repeated characters
also incur this penalty. Empty finger assignments skip the finger penalty.

These penalties add to key effort and the existing transition costs, including
explicit transition overrides. `SameFingerPenalty` still applies only to default
consecutive transitions. The craft priority multiplies the entire search cost,
so a priority-5 craft pays five times each comfort penalty. Raise `DistancePenalty`
to prefer nearby keys, or `FingerReusePenalty` to prefer different fingers; zero
disables either. Priorities remain soft preferences across the whole layout.
Physical key distances are precomputed once during optimization for speed.

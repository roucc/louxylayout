# Louxy Layout

A Go tool for optimizing Minecraft search-crafting keyboard layouts.

Run `go run .`, or build an executable with `go build .`. The report shows the
minimum character set, generation time, physical key bindings on both layers,
weighted cost, searches, and group crafting sequences. Lower cost is better.

## Personal profiles

The CLI uses **roux** by default. Personal settings live in
[`profiles/roux.json`](profiles/roux.json):

```sh
go run .                              # your profile
go run . -profile roux                 # select it explicitly
go run . -list-profiles                # list available profiles
go run . -profile lily                 # Elfdalian, ESDF, preferred searches
go run . -profile roux-actual          # Roux with specified actual searches and junk
go run . -profile friend               # profiles/friend.json
go run . -profile /path/to/friend.json # load a file elsewhere
```

To add another person, copy `profiles/roux.json` to `profiles/friend.json`, change
`name`, and edit their settings. Each file is self-contained; loading a profile
never merges in another person's preferences. Local JSON files are read on each
run, including by a built executable. Bundled files provide a fallback when the
executable runs outside the project directory. Unknown names, malformed JSON,
and misspelled fields produce errors rather than silently selecting your profile.

A profile contains:

- `language`, `inventory`, `goals`, and `allow_good_junk`.
- `preferred_searches`: allowed searches per item; preserve trailing spaces.
- `allowed_junk_searches`: exact per-item exceptions that may match non-goal crafts.
- `weights`: physical key costs and positions, transitions, craft priorities,
  layer preferences, shared-character preferences, groups, and `before` rules.
- `preferred_bindings` and `preferred_shift_bindings`: movable starting bindings.
- `fixed_bindings` and `fixed_shift_bindings`: assignments that must stay fixed.
- `optimizer`: `seed`, `restarts`, `iterations`, and `enable_shift_layer`.

Binding maps use **physical key → typed character**:

```json
"preferred_bindings": {"A": "n", "W": "u"},
"preferred_shift_bindings": {"A": "n", "W": "h"},
"fixed_bindings": {"Space": "Space"},
"fixed_shift_bindings": {"Space": "Space"}
```

Use literal Unicode characters such as `ø` and `å`; `"Space"` or `" "` represents
a space in binding maps. Starting bindings seed the first restart, then can move
or disappear. Later restarts explore random layouts. Fixed bindings stay in
place. Duplicate characters within a layer, unknown keys, and conflicts between
fixed and starting bindings produce errors. Characters can repeat across layers.
An omitted `fixed_shift_bindings` inherits a fixed Space binding from non-shift;
an explicitly empty map leaves that layer unrestricted.

Tune `optimizer` for search effort. The default profile uses seed 42, eight
restarts and 4,000 iterations per restart. The minimum character set seeds
optimization, which can add or replace characters. Results are heuristic, not a
guarantee of a globally optimal layout. Profile loading adds no optimizer states.

Set `language` to `"en_gb"` for English, `"no_no"` for Norwegian, or `"ovd"` for
Elfdalian. Language and recipe JSON are embedded at build time.

### Lily's ESDF profile

[`profiles/lily.json`](profiles/lily.json) uses Elfdalian (`ovd`), the same goals
and inventory as Roux, and the supplied ESDF key efforts and finger assignments.
Space retains its existing cost. Coordinates follow physical QWERTY rows.
Transition overrides were shifted one key right within each row; missing
horizontal-neighbour costs use the nearest transferred directional cost, with
same-finger penalties for unmeasured same-finger pairs. Other transitions use the fallback costs and physical-distance penalty, with
the additional left-side row-change overrides described below.

`_` in the supplied searches was converted to a literal space. Searches for items
outside the current goals were omitted. Unspecified groups, priorities, layer
preferences, penalties and optimizer controls were copied into Lily's file when
it was created. These files are independent: missing fields do not inherit from
Roux, and later edits to Roux do not alter Lily.

## Search preferences and junk

`preferred_searches` restricts a goal to the listed valid searches:

```json
"preferred_searches": {
  "block.minecraft.white_bed": ["l ", "n "]
}
```

An empty or omitted list permits all valid searches. The optimizer chooses the
cheapest typeable allowed search. Case is ignored, but spaces are significant.
Invalid searches and unknown goal IDs produce errors. Restrictions apply to
minimum-character selection, optimization, and reporting.

`allow_good_junk` defaults to false, rejecting avoidable matches with other goal
crafts. Unavoidable matches remain allowed in either mode. Candidate filtering
uses the 2x2 inventory grid when the target fits, otherwise the 3x3 table.
For explicitly accepted shortcuts that also match non-goal items, use a narrow
per-item exception in the profile:

```json
"allow_good_junk": true,
"allowed_junk_searches": {
  "item.minecraft.bow": ["vb"],
  "item.minecraft.iron_ingot": ["nta"],
  "block.minecraft.nether_bricks": ["h"]
}
```

These exceptions apply only to the listed searches and must still match the
target's recipe-group names. Other searches keep the normal junk policy.
`preferred_searches` still controls which searches the optimizer may select.
Lily uses these three exceptions to retain the supplied Elfdalian shortcuts.

Planning uses the configured inventory; it does not simulate changing ingredients
as crafts are performed. A shortcut rejected by candidate filtering needs an explicit exception; prefix
overlap alone does not make it valid.

## Craft priorities and key comfort

Under `weights.crafts`, higher numbers give a craft more influence on typing
cost. Unlisted goals default to 1; zero ignores a craft. Each craft's priority
multiplies its full search cost, including its layer preference penalty.

`weights.keys` describes physical keys independently of their assigned letters:

```json
"keys": {
  "Space": {"effort": 1, "finger": "thumb", "x": 3, "y": 4}
}
```

Higher `effort` means harder to press. `x` and `y` approximate physical positions
in key-width units, including keyboard row stagger. They are used for travel
within searches and proximity between crafts. Costs and penalties must be
finite and nonnegative; key coordinates may be negative.

The profile's `weights` also contains:

- `distance_penalty`: cost per key-width travelled between consecutive presses.
- `finger_reuse_penalty`: cost for every pair of presses using the same finger
  within a search, including nonconsecutive and repeated-key presses.
- `same_finger_penalty`: extra cost for consecutive same-finger presses when
  their transition has no explicit override.
- `default_transition_penalty`: base cost for transitions without an override.
- `transitions`: directional base-cost overrides, such as
  `[{"from": "Q", "to": "W", "cost": 0.3}]`.

Distance and finger-reuse penalties still apply to explicit transition overrides.
An empty finger assignment skips finger penalties. Zero disables an extra
penalty. Physical key costs and transition tables are shared by both layers.

### Left-side row changes

All bundled profiles give staggered cross-row moves on the ring/middle side of
the keyboard stronger explicit transition costs. This includes A ↔ Q. Roux
profiles cover the left cluster through W/S/X, while Lily profiles extend the
region one column right through E/D/C for ESDF. Space and horizontal row rolls
are excluded. Same-finger charges are preserved in the new overrides.

These transition costs are at least `default_transition_penalty + 0.5`, plus the
same-finger penalty when applicable. A → Q costs 1.0 in Roux profiles and 2.0 in
Lily profiles (A and Q both use Lily's ring finger). Key efforts, distance costs,
and finger-reuse penalties still apply. A priority-5 craft therefore pays 2.5
additional weighted cost units for an A → Q move compared with the old weights.

These are soft penalties, so the optimizer can trade them against the rest of
the layout. They use the existing transition lookup and shared physical-key
tables on both layers, adding no optimizer states or iterations. Edit individual
entries under `weights.transitions` to tune particular moves.

## Craft groups and required order

Groups under `weights.groups` are unordered sets of crafts performed together:

```json
"groups": [
  {
    "name": "bastion",
    "priority": 3,
    "crafts": [
      "block.minecraft.white_bed",
      "item.minecraft.iron_ingot",
      "item.minecraft.iron_axe",
      "item.minecraft.iron_sword"
    ],
    "before": {
      "item.minecraft.iron_ingot": [
        "item.minecraft.iron_axe",
        "item.minecraft.iron_sword"
      ]
    }
  }
]
```

Higher group `priority` encourages nearby search keys and cheaper group editing
sequences. Zero disables the group. Groups may overlap; their costs add together.
Missing or ignored goals are omitted, and duplicate item IDs count once.
List order has no meaning. The planner chooses craft order and searches together.

`before` requires its source craft to precede each target. Both must belong to
the group. The example requires iron ingots before iron tools, leaving the bed
free to move. Chains are supported; unknown members and cycles produce errors.
Missing or ignored crafts are omitted along with their incident rules.
Precedence reuses the visited-craft mask: it adds no search states and skips
moves whose prerequisites have not been visited.

Proximity uses all active craft pairs, based on each craft's cheapest standalone
search. Their mean key distances are summed, scaled by `2 / activeCraftCount`,
and multiplied by group priority. This keeps the scale similar to `n-1` edges
while ensuring that rearranging a group's list cannot change its proximity score.

## Backspace overlap

Configure existing control costs under `weights.search_editing`:

```json
"search_editing": {"shift_home": 1, "backspace": 0.5}
```

These are action costs, not new bindings. Home on MB4 and Backspace on MB5 remain
outside the keyboard assignments. `shift_home` includes selecting the entire old
search before replacing it. Each transition permits **at most one Backspace**;
two or more deletions require Shift+Home. Set `search_editing` to `null` to disable
sequence planning.

For `l ` → `lha`, the planner compares `SH lha` against `BS ha`: the latter deletes
the trailing space, keeps `l`, and types only `ha`. It can also append without
deleting, delete without typing, or keep an identical search. Unicode characters
such as `ø` take one Backspace.

Scoring adds each group's priority times its cheapest editing sequence cost to
standalone craft costs and proximity. A group may use a different search from a
standalone craft or another group. All allowed candidates remain available to
sequence planning, including longer searches that help subsequent crafts.

Newly typed text incurs its typing costs and finger-reuse penalties. Mouse
controls incur their configured action costs, without keyboard travel or
cross-action finger penalties. Shared-prefix indexes and cached typing costs
avoid comparing every candidate pair. Exact order planning supports up to ten
active unique crafts per group; larger groups produce a configuration error.

## Shift layers

Use `weights.preferred_layers` for the preferred Shift state at the crafting click:

```json
"preferred_layers": {
  "block.minecraft.white_bed": "shift",
  "item.minecraft.stick": "non-shift"
},
"layer_preference_penalty": 5,
"layer_switch_penalty": 0.5,
"prefer_both_layers": {"n": 5}
```

`"any"` or an omitted craft means no preference. Using the wrong layer incurs
`layer_preference_penalty` multiplied by craft priority. This is a soft cost, not
a restriction. Group plans include that cost before group priority weights the
plan. Pressing or releasing Shift incurs `layer_switch_penalty`.

Each physical key has an independent assignment on each layer, but key efforts,
finger assignments, distances, and transitions are shared. A Q → W → E roll has
the same typing cost on both layers. Every character can appear on both layers,
on the same or different physical keys. `prefer_both_layers` adds a cost for each
layer missing an especially useful character; this also remains a soft preference.

Currently each complete search is typed on **one layer throughout**, retaining
that Shift state for the crafting click. Mid-search mixed-layer words such as
`LHj` are not supported yet. Backspace overlap can retain text from a previous
craft on the other layer. Groups start with Shift released and jointly choose
order, searches, layers, and editing actions.

The CLI shows both assignments for each physical key, the chosen layer and click
behaviour for each craft, and Shift changes in group plans. Disable
`optimizer.enable_shift_layer` for single-layer optimization.

## Code structure

- `main.go`: selects a profile, loads data, and prints results.
- `internal/profile/`: JSON loading, validation, and conversion to optimizer types.
- `profiles/`: personal settings and bundled fallback copies.
- `internal/data/`: embedded recipes and language JSON.
- `internal/search/`: craftability, visibility, substring candidates, and junk rules.
- `internal/layout/`: character selection, cost models, optimization, and group plans.

For Go callers, `profile.Profile.Settings` returns `layout.Weights` and
`layout.OptimizeOptions`. `Optimized.Bindings` holds non-shift assignments,
`ShiftBindings` holds shifted assignments, and `LayeredSearches` includes chosen
layers and physical keys. Use `LayeredCost` and `PlanLayeredGroups` to inspect a
`LayeredLayout`. Search state is constructed explicitly with `search.New`; treat
its recipe groups and item-name maps as read-only.

Unit tests use small local fixtures. Profile integration tests and benchmarks
load the bundled default JSON, so personal configuration is not duplicated in Go.

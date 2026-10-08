# Louxy Layout

A Go tool for optimizing Minecraft search-crafting keyboard layouts.

Run the layout optimizer with `go run .`, or build an executable with `go build .`.
The report shows the minimum character set, optimized physical key bindings,
weighted typing cost, and the cheapest search for each craft. Lower cost is better.

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
  and craft priorities. Unlisted craft priorities default to 1; zero ignores a craft.
- `cost.go`: sums each craft's priority times its cheapest typeable search cost.
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

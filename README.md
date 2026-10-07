# Louxy Layout

A Go tool for optimizing Minecraft search-crafting keyboard layouts.

Run the current search report with `go run .`.

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
Future keyboard models and scoring can live in `internal/layout`.

Set `config.Language` to `"en_gb"` for English. Language JSON is embedded
at build time. Search state is constructed explicitly with `search.New`,
allowing separate inventories, languages, and goal lists. Treat the recipe
groups and item-name map passed to the constructor as read-only.

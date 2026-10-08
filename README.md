# Louxy Layout

A Go tool for optimizing Minecraft search-crafting keyboard layouts. Run `go run .`.

Groups are unordered: the optimizer jointly chooses craft order and searches,
using shared-prefix indexes and a visited-craft mask. `CraftGroup.Before` adds
precedence rules without adding states. Missing/ignored crafts are omitted.
Exact planning supports up to ten active unique crafts per group. Each transition
allows at most one Backspace; longer deletions use Shift+Home.

`OptimizeOptions.EnableShiftLayer` enables independent assignments on both layers.
Physical key effort, fingers, distances and transitions are shared. Craft layer
preferences are soft costs; unlisted goals can use either layer. Characters can
appear on both layers, and `PreferBothLayers` encourages important duplicates.
Each complete search uses one layer throughout, with its Shift state retained
for the craft click. Group plans also choose layers and Shift changes.

The CLI prints keyboard rows, both layers, selected searches, group plans and
layout generation time. Set language, inventory and goals in `internal/config`;
keyboard and craft costs are in `internal/layout/keyboard.go`.

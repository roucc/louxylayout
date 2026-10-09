// Package profiles contains bundled profile files for standalone executables.
package profiles

import "embed"

const Default = "roux"

// Files provides fallback profiles when no local profile file exists.
// Local profiles are read at runtime so edits do not require rebuilding.
//
//go:embed *.json
var Files embed.FS

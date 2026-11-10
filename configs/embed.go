// Package configs holds the configuration files bundled into the binary.
package configs

import "embed"

// Themes holds the bundled themes as themes/<name>.toml.
//
//go:embed themes/*.toml
var Themes embed.FS

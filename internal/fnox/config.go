// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package fnox

import (
	"fmt"
	"os"
	"path/filepath"
)

// ConfigFile is the name of fnox's global configuration file inside its
// config directory.
const ConfigFile = "config.toml"

// GlobalConfigPath resolves fnox's global config file the way fnox itself
// does (crates/fnox-core/src/env.rs): $FNOX_CONFIG_DIR, else
// $XDG_CONFIG_HOME/fnox, else ~/.config/fnox, each joined with config.toml.
func GlobalConfigPath() (string, error) {
	if dir := os.Getenv("FNOX_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, ConfigFile), nil
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "fnox", ConfigFile), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home: %w", err)
	}
	return filepath.Join(home, ".config", "fnox", ConfigFile), nil
}

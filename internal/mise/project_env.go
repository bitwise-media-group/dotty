// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package mise

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// ProjectConfig is the per-directory mise config file plain environment
// variables are written to — the committed file, not mise.local.toml.
const ProjectConfig = "mise.toml"

// TableEnv is the table holding a config file's environment variables.
const TableEnv = "env"

// ProjectEnv writes environment variables into project config files — the
// mise.toml beside a codebase — rather than the profile's mise directory, so
// unlike Client it carries no retargeted global config.
type ProjectEnv struct {
	bin string
	r   Runner
}

// NewProjectEnv returns a ProjectEnv driving bin. An empty bin resolves
// "mise" through PATH.
func NewProjectEnv(r Runner, bin string) *ProjectEnv {
	if bin == "" {
		bin = "mise"
	}
	return &ProjectEnv{bin: bin, r: r}
}

// Keys returns the variables the config file at path declares in its env
// table, in file order. Directives (`_.file`, `_.path`) are not variables
// and are left out. A missing file declares none.
func (p *ProjectEnv) Keys(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var keys []string
	for _, e := range scanEntries(data, path) {
		if e.Table == TableEnv && e.ID != "_" {
			keys = append(keys, e.ID)
		}
	}
	return keys, nil
}

// Set writes key=value into the env table of the config file at path,
// creating the file when it does not exist. mise renders env values as
// templates and expands $VARS in them, so callers warn about values that
// carry either before calling. The value goes in argv: callers pass only
// values that were already plaintext on disk.
func (p *ProjectEnv) Set(ctx context.Context, path, key, value string) error {
	if err := p.r.RunEnv(ctx, nil, p.bin, "set", "--file", path, key+"="+value); err != nil {
		return fmt.Errorf("set %s in %s: %w", key, path, err)
	}
	return nil
}

// Trust marks the config file at path as trusted so mise loads it without
// prompting. Only call it for a file dotty itself wrote.
func (p *ProjectEnv) Trust(ctx context.Context, path string) error {
	if err := p.r.RunEnv(ctx, nil, p.bin, "trust", path); err != nil {
		return fmt.Errorf("trust %s: %w", path, err)
	}
	return nil
}

// Expands reports whether mise would rewrite value when loading it from an
// env table — a tera tag or a $ reference — so a literal carried over from a
// .env file would no longer reach the process verbatim.
func Expands(value string) bool {
	return strings.Contains(value, "$") ||
		strings.Contains(value, "{{") || strings.Contains(value, "{%") || strings.Contains(value, "{#")
}

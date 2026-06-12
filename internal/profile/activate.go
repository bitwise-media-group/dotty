// MIT License
//
// Copyright (c) 2026 Bitwise Media Group
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package profile

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/bitwise-media-group/dotty/internal/brewfile"
)

// Activate points the active-profile symlink at the named profile and returns
// the profile's directory. The swap is atomic — a temp symlink renamed over
// the old one — so the link never dangles mid-switch. The symlink target is
// the bare profile name (relative), which survives a home-directory move and
// reads cleanly in a dotfiles repository.
//
// A profile activated without a Brewfile gets one dumped from the currently
// installed brews, so `dotty brewfile ...` works immediately after.
func Activate(ctx context.Context, r brewfile.Runner, configDir, name string) (string, error) {
	if !Exists(configDir, name) {
		return "", fmt.Errorf("profile %q: %w", name, ErrNotFound)
	}
	tmp := filepath.Join(configDir, ".active-profile.tmp")
	if err := os.Remove(tmp); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("clear stale temp symlink: %w", err)
	}
	if err := os.Symlink(name, tmp); err != nil {
		return "", fmt.Errorf("create temp symlink: %w", err)
	}
	if err := os.Rename(tmp, filepath.Join(configDir, activeLink)); err != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("swap active-profile symlink: %w", err)
	}

	dir := Dir(configDir, name)
	if _, err := os.Stat(BrewfilePath(dir)); errors.Is(err, fs.ErrNotExist) {
		if err := brewfile.Dump(ctx, r, BrewfilePath(dir), false, false); err != nil {
			return dir, fmt.Errorf("dump Brewfile for fresh profile: %w", err)
		}
	}
	return dir, nil
}

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
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bitwise-media-group/dotty/internal/cli"
)

// activeLink is the symlink under the config dir that names the active profile.
const activeLink = "active-profile"

// metadataFile holds a profile's metadata inside its directory.
const metadataFile = "profile.json"

var (
	// ErrExists reports a profile name that is already taken.
	ErrExists = errors.New("profile already exists")
	// ErrNotFound reports a profile name with no directory behind it.
	ErrNotFound = errors.New("profile not found")
	// ErrNoActiveProfile reports that the active-profile symlink is missing.
	ErrNoActiveProfile = errors.New("no active profile (run `dotty profile activate`)")
)

// Profile is the metadata persisted as profile.json inside a profile directory.
type Profile struct {
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// ValidateName rejects names that cannot be a directory under the config dir
// or would collide with the active-profile symlink.
func ValidateName(name string) error {
	switch {
	case name == "":
		return errors.New("profile name must not be empty")
	case name == activeLink:
		return fmt.Errorf("%q is reserved for the active-profile symlink", name)
	case name == "." || name == "..":
		return fmt.Errorf("%q is not a valid profile name", name)
	case strings.ContainsAny(name, "/\\"):
		return errors.New("profile name must not contain path separators")
	case strings.HasPrefix(name, "."):
		return errors.New("profile name must not start with a dot")
	}
	return nil
}

// Dir returns the directory a profile of the given name lives in.
func Dir(configDir, name string) string {
	return filepath.Join(configDir, name)
}

// BrewfilePath returns the Brewfile location inside a profile directory.
func BrewfilePath(profileDir string) string {
	return filepath.Join(profileDir, "Brewfile")
}

// Exists reports whether a profile directory of the given name exists.
func Exists(configDir, name string) bool {
	info, err := os.Stat(Dir(configDir, name))
	return err == nil && info.IsDir()
}

// Create makes the profile directory and writes its metadata. It fails with
// ErrExists when the name is already taken.
func Create(configDir, name, description string) (Profile, error) {
	if err := ValidateName(name); err != nil {
		return Profile{}, err
	}
	if Exists(configDir, name) {
		return Profile{}, fmt.Errorf("profile %q: %w", name, ErrExists)
	}
	dir := Dir(configDir, name)
	if err := cli.EnsureDir(dir, 0o755); err != nil {
		return Profile{}, err
	}
	p := Profile{Name: name, Description: description, CreatedAt: time.Now().UTC()}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return Profile{}, fmt.Errorf("encode profile metadata: %w", err)
	}
	if err := cli.AtomicWriteFile(filepath.Join(dir, metadataFile), append(data, '\n'), 0o644); err != nil {
		return Profile{}, err
	}
	return p, nil
}

// Load reads a profile's metadata. A directory without profile.json is still
// a usable profile (e.g. hand-copied), so the metadata file is optional.
func Load(configDir, name string) (Profile, error) {
	if !Exists(configDir, name) {
		return Profile{}, fmt.Errorf("profile %q: %w", name, ErrNotFound)
	}
	data, err := os.ReadFile(filepath.Join(Dir(configDir, name), metadataFile))
	if errors.Is(err, fs.ErrNotExist) {
		return Profile{Name: name}, nil
	}
	if err != nil {
		return Profile{}, fmt.Errorf("read profile metadata: %w", err)
	}
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return Profile{}, fmt.Errorf("decode %s: %w", filepath.Join(name, metadataFile), err)
	}
	p.Name = name // the directory is authoritative
	return p, nil
}

// List returns all profiles under the config dir, sorted by name. The
// active-profile symlink and hidden entries are not profiles.
func List(configDir string) ([]Profile, error) {
	entries, err := os.ReadDir(configDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", configDir, err)
	}
	var profiles []Profile
	for _, e := range entries {
		// Symlinks (active-profile) report !IsDir from ReadDir.
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		p, err := Load(configDir, e.Name())
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].Name < profiles[j].Name })
	return profiles, nil
}

// ActiveDir resolves the active-profile symlink to the active profile's
// directory, returning ErrNoActiveProfile when none is set.
func ActiveDir(configDir string) (string, error) {
	target, err := os.Readlink(filepath.Join(configDir, activeLink))
	if errors.Is(err, fs.ErrNotExist) {
		return "", ErrNoActiveProfile
	}
	if err != nil {
		return "", fmt.Errorf("read active-profile symlink: %w", err)
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(configDir, target)
	}
	return target, nil
}

// ActiveName returns the name of the active profile.
func ActiveName(configDir string) (string, error) {
	dir, err := ActiveDir(configDir)
	if err != nil {
		return "", err
	}
	return filepath.Base(dir), nil
}

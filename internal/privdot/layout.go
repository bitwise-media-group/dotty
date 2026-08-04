// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package privdot

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bitwise-media-group/dotty/internal/scaffold"
)

// CipherExt marks a repository file as age ciphertext; the live path drops
// the suffix. Files without it are deployed as-is — the suffix is the
// per-file encryption switch.
const CipherExt = ".age"

// RecipientsFile is the per-profile recipients list, authoritative for
// encryption: one age recipient per line, comments naming the hardware.
const RecipientsFile = "recipients.txt"

// identityPrefix and identityExt frame the per-serial identity stubs beside
// the recipients file. The stubs are not secrets — age-plugin-yubikey
// regenerates them from the token — so they commit with the repository.
const (
	identityPrefix = "identity-"
	identityExt    = ".txt"
)

// IsRepo reports whether dir is a private dotfiles repository — marked by
// scaffold.PrivateMarker.
func IsRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, scaffold.PrivateMarker))
	return err == nil
}

// EnclosingRepo walks up from the working directory looking for a private
// repository; "" when there is none.
func EnclosingRepo() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if IsRepo(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// ProfilesDir returns the directory the repository's private profiles live in.
func ProfilesDir(repo string) string { return filepath.Join(repo, "profiles") }

// ProfileDir returns a named profile's directory inside the repository.
func ProfileDir(repo, name string) string { return filepath.Join(ProfilesDir(repo), name) }

// HomeDir returns a profile's $HOME-shaped tree: the entries deployed into
// the home directory, ciphertext carrying the CipherExt suffix.
func HomeDir(repo, name string) string { return filepath.Join(ProfileDir(repo, name), "home") }

// AgeDir returns a profile's age directory: recipients and identity stubs.
func AgeDir(repo, name string) string { return filepath.Join(ProfileDir(repo, name), "age") }

// RecipientsPath returns a profile's recipients file location.
func RecipientsPath(repo, name string) string {
	return filepath.Join(AgeDir(repo, name), RecipientsFile)
}

// IdentityPath returns the identity stub location for one hardware serial.
func IdentityPath(repo, name, serial string) string {
	return filepath.Join(AgeDir(repo, name), identityPrefix+serial+identityExt)
}

// Identities returns a profile's identity stub paths, sorted, so decryption
// can offer every enrolled key and succeed with whichever is plugged in.
func Identities(repo, name string) ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(AgeDir(repo, name), identityPrefix+"*"+identityExt))
	if err != nil {
		return nil, fmt.Errorf("list identities: %w", err)
	}
	sort.Strings(paths)
	return paths, nil
}

// File is one $HOME-relative entry a private profile carries.
type File struct {
	// Rel is the live path relative to $HOME — the CipherExt suffix already
	// dropped for encrypted entries.
	Rel string
	// Encrypted reports whether the repository stores the entry as age
	// ciphertext.
	Encrypted bool
	// RepoPath is the entry's absolute path inside the repository.
	RepoPath string
}

// gitkeepName is the committable placeholder Scaffold leaves in empty
// directories; it is repository furniture, never a deployable entry.
const gitkeepName = ".gitkeep"

// ListFiles walks a profile's home tree and returns its entries sorted by
// Rel. A missing tree is an empty profile, not an error — a work machine's
// clone simply never carries the personal profile.
func ListFiles(repo, name string) ([]File, error) {
	root := HomeDir(repo, name)
	var files []File
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() == gitkeepName {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("relativize %s: %w", path, err)
		}
		f := File{Rel: rel, RepoPath: path}
		if cut, ok := strings.CutSuffix(rel, CipherExt); ok {
			f.Rel, f.Encrypted = cut, true
		}
		files = append(files, f)
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("walk %s: %w", root, err)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Rel < files[j].Rel })
	return files, nil
}

// ListProfiles returns the names of the private profiles the repository
// carries, sorted.
func ListProfiles(repo string) ([]string, error) {
	entries, err := os.ReadDir(ProfilesDir(repo))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read private profiles: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// DataRoot returns the decrypted private area under the dotty data
// directory. Everything beneath it is created 0700/0600 — it holds live
// plaintext.
func DataRoot(dataDir string) string { return filepath.Join(dataDir, "private") }

// ActiveLink returns the data-dir symlink naming the active private profile.
// $HOME links route through it, so retargeting it swaps the whole private
// identity atomically — the same mechanism as the config-dir active-profile.
func ActiveLink(dataDir string) string { return filepath.Join(DataRoot(dataDir), "active-profile") }

// PlainDir returns a profile's decrypted home tree in the data directory.
func PlainDir(dataDir, name string) string {
	return filepath.Join(DataRoot(dataDir), name, "home")
}

// PlainPath returns one entry's decrypted location in the data directory.
func PlainPath(dataDir, name, rel string) string {
	return filepath.Join(PlainDir(dataDir, name), rel)
}

// ManifestPath returns a profile's decrypt manifest location.
func ManifestPath(dataDir, name string) string {
	return filepath.Join(DataRoot(dataDir), name, "manifest.json")
}

// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package mise

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
)

// installerSigURL is the official mise installer as an OpenPGP-signed
// message: the script wrapped in a signature by the mise release key. The
// script itself carries the per-platform checksums of the binary it
// downloads, so verifying the script is the whole chain of trust.
const installerSigURL = "https://mise.jdx.dev/install.sh.sig"

// releaseKeyFingerprint is the mise release key (mise releases
// <release@mise.jdx.dev>), as published in the mise installation docs. The
// armored key is embedded from mise-release-key.asc; the fingerprint pins
// which key in it may sign.
const releaseKeyFingerprint = "24853EC9F655CE80B48E6C3A8B81C9D17413A06D"

//go:embed mise-release-key.asc
var releaseKey []byte

// ErrBadSignature reports an installer whose signature does not verify
// against the mise release key.
var ErrBadSignature = errors.New("mise installer signature does not verify")

// brewPrefixes are the Homebrew prefixes a `brew install mise` lands under.
// A mise found there is not used: the keg is not a declared bootstrap
// package, so Sync prunes it — pulling the binary out from under the process
// running the prune.
var brewPrefixes = []string{"/opt/homebrew/", "/home/linuxbrew/"}

// LocalBin returns the mise binary path the installer writes:
// ~/.local/bin/mise.
func LocalBin(home string) string { return filepath.Join(home, ".local", "bin", "mise") }

// Lookup finds a usable mise binary without installing one: the installer's
// ~/.local/bin/mise first, else a PATH hit outside the Homebrew prefixes.
// It returns ErrNotInstalled when neither exists.
func Lookup(lookPath func(string) (string, error), home string) (string, error) {
	local := LocalBin(home)
	if info, err := os.Stat(local); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
		return local, nil
	}
	path, err := lookPath("mise")
	if err != nil {
		return "", ErrNotInstalled
	}
	for _, prefix := range brewPrefixes {
		if strings.HasPrefix(path, prefix) {
			return "", fmt.Errorf("%w outside Homebrew (found %s)", ErrNotInstalled, path)
		}
	}
	return path, nil
}

// Fetcher downloads one URL; Fetch is the production implementation and
// tests substitute canned bytes.
type Fetcher func(ctx context.Context, url string) ([]byte, error)

// Fetch downloads url over HTTPS.
func Fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("request %s: %w", url, err)
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	return data, nil
}

// VerifyInstaller checks that sig is an OpenPGP-signed message from the mise
// release key and returns the script it wraps. Anything else — a bad
// signature, a different signer, an unsigned message — is ErrBadSignature:
// an installer that does not verify is never run.
func VerifyInstaller(sig []byte) ([]byte, error) {
	keyring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(releaseKey))
	if err != nil {
		return nil, fmt.Errorf("read embedded mise release key: %w", err)
	}
	md, err := openpgp.ReadMessage(bytes.NewReader(sig), keyring, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadSignature, err)
	}
	if !md.IsSigned {
		return nil, fmt.Errorf("%w: message is not signed", ErrBadSignature)
	}
	// The signature trails the body, so it is only checked once the body
	// has been read to the end.
	body, err := io.ReadAll(md.UnverifiedBody)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadSignature, err)
	}
	if md.SignatureError != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadSignature, md.SignatureError)
	}
	if md.SignedBy == nil {
		return nil, fmt.Errorf("%w: signed by a key that is not the mise release key", ErrBadSignature)
	}
	fingerprint := strings.ToUpper(hex.EncodeToString(md.SignedBy.Entity.PrimaryKey.Fingerprint))
	if fingerprint != releaseKeyFingerprint {
		return nil, fmt.Errorf("%w: signed by %s, want %s", ErrBadSignature, fingerprint, releaseKeyFingerprint)
	}
	return body, nil
}

// EnsureInstalled returns the mise binary to drive, installing it into
// ~/.local/bin when Lookup finds none: the signed installer is downloaded,
// verified against the embedded release key, written to a scratch file, and
// run attached to the terminal so its progress is visible. It carries no
// MISE_CONFIG_DIR because no profile is involved yet.
func EnsureInstalled(ctx context.Context, r InstallRunner, lookPath func(string) (string, error),
	fetch Fetcher, home string) (string, error) {
	if bin, err := Lookup(lookPath, home); err == nil {
		return bin, nil
	}
	sig, err := fetch(ctx, installerSigURL)
	if err != nil {
		return "", fmt.Errorf("install mise: %w", err)
	}
	script, err := VerifyInstaller(sig)
	if err != nil {
		return "", fmt.Errorf("install mise: %w", err)
	}

	local := LocalBin(home)
	if err := os.MkdirAll(filepath.Dir(local), 0o755); err != nil {
		return "", fmt.Errorf("create %s: %w", filepath.Dir(local), err)
	}
	tmp, err := os.CreateTemp("", "mise-install-*.sh")
	if err != nil {
		return "", fmt.Errorf("create scratch installer: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := tmp.Write(script); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("write scratch installer: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("write scratch installer: %w", err)
	}
	env := []string{"MISE_INSTALL_PATH=" + local}
	if err := r.RunInteractiveEnv(ctx, env, "sh", tmpPath); err != nil {
		return "", fmt.Errorf("install mise: %w", err)
	}
	return local, nil
}

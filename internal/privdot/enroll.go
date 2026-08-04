// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package privdot

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/bitwise-media-group/dotty/internal/cli"
)

// yubikeyPlugin is the enrollment backend: age identities in the PIV
// "retired" slots (the plugin numbers them 1–20). Everything after
// enrollment treats its output as opaque age files, so other plugins can
// join later without touching the crypto layer.
const yubikeyPlugin = "age-plugin-yubikey"

// PluginSlots is how many identities one YubiKey can hold — the 20 retired
// PIV slots. The standard slots (login, signing) are never touched, but the
// PIV PIN and its retry counter are shared with them.
const PluginSlots = 20

// EnrollRunner is the slice of ExecRunner enrollment consumes. --generate
// runs interactively (the plugin prompts for touch and the PIV management
// key on the terminal); the read-only listing and identity queries capture
// output.
type EnrollRunner interface {
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
	RunInteractive(ctx context.Context, name string, args ...string) error
	LookPath(name string) (string, error)
}

// EnrollOptions steer one enrollment.
type EnrollOptions struct {
	// Serial is the YubiKey to enroll; required.
	Serial string
	// Slot is the plugin slot (1–20); 0 picks the first free one.
	Slot int
	// PINPolicy and TouchPolicy default to "once" and "cached": one PIN per
	// PIV session and one touch per 15-second window, so a whole-tree
	// decrypt costs a single PIN+touch burst. Note the PIN cache is cleared
	// whenever another applet is used — a FIDO2 ssh signature between
	// decrypts re-triggers the prompt.
	PINPolicy   string
	TouchPolicy string
	// Name labels the identity on the token; default dotty-<profile>.
	Name string
}

// Enroll creates an age identity for profile on a YubiKey, writes the
// identity stub beside the profile's recipients, and appends the recipient.
// It returns the recipient and the chosen slot. The stub is not a secret —
// the plugin regenerates it from the token — so it commits with the
// repository, letting any machine with the key plugged in decrypt.
func Enroll(ctx context.Context, r EnrollRunner, repo, profile string, opts EnrollOptions) (string, int, error) {
	if _, err := r.LookPath(yubikeyPlugin); err != nil {
		return "", 0, err
	}
	if opts.PINPolicy == "" {
		opts.PINPolicy = "once"
	}
	if opts.TouchPolicy == "" {
		opts.TouchPolicy = "cached"
	}
	if opts.Name == "" {
		opts.Name = "dotty-" + profile
	}

	listing, err := r.Output(ctx, yubikeyPlugin, "--list-all")
	if err != nil {
		return "", 0, fmt.Errorf("list existing identities: %w", err)
	}
	used := ParseUsedSlots(listing)[opts.Serial]
	slot := opts.Slot
	if slot == 0 {
		if slot = firstFreeSlot(used); slot == 0 {
			return "", 0, fmt.Errorf("YubiKey %s has no free identity slots (all %d in use)", opts.Serial, PluginSlots)
		}
	} else if slices.Contains(used, slot) {
		return "", 0, fmt.Errorf("slot %d on YubiKey %s already holds an identity (%s --list-all shows them)",
			slot, opts.Serial, yubikeyPlugin)
	}

	if err := r.RunInteractive(ctx, yubikeyPlugin, "--generate",
		"--serial", opts.Serial, "--slot", strconv.Itoa(slot), "--name", opts.Name,
		"--pin-policy", opts.PINPolicy, "--touch-policy", opts.TouchPolicy); err != nil {
		return "", 0, fmt.Errorf("generate identity on YubiKey %s: %w", opts.Serial, err)
	}

	// Regenerate the identity stub from the token — the canonical read-only
	// way to capture what --generate just created.
	identity, err := r.Output(ctx, yubikeyPlugin, "--identity", "--serial", opts.Serial, "--slot", strconv.Itoa(slot))
	if err != nil {
		return "", 0, fmt.Errorf("read back identity: %w", err)
	}
	recipient, err := ParseRecipient(identity)
	if err != nil {
		return "", 0, err
	}

	if err := cli.EnsureDir(AgeDir(repo, profile), 0o755); err != nil {
		return "", 0, err
	}
	if err := cli.AtomicWriteFile(IdentityPath(repo, profile, opts.Serial), identity, 0o644); err != nil {
		return "", 0, err
	}
	if err := AppendRecipient(repo, profile, opts.Serial, slot, recipient); err != nil {
		return "", 0, err
	}
	return recipient, slot, nil
}

// firstFreeSlot returns the lowest plugin slot not in used, 0 when full.
func firstFreeSlot(used []int) int {
	for slot := 1; slot <= PluginSlots; slot++ {
		if !slices.Contains(used, slot) {
			return slot
		}
	}
	return 0
}

// ParseUsedSlots reads the occupied slots per serial out of
// `age-plugin-yubikey --list-all` output. The format is comment blocks like
// "#       Serial: 20444152, Slot: 1"; the parser is deliberately loose —
// the format is stable but unversioned.
func ParseUsedSlots(listing []byte) map[string][]int {
	used := map[string][]int{}
	for line := range strings.Lines(string(listing)) {
		rest, ok := textAfter(line, "Serial:")
		if !ok {
			continue
		}
		serialPart, slotPart, ok := strings.Cut(rest, ",")
		if !ok {
			continue
		}
		slotText, ok := textAfter(slotPart, "Slot:")
		if !ok {
			continue
		}
		serial := strings.TrimSpace(serialPart)
		slot, err := strconv.Atoi(strings.TrimSpace(slotText))
		if err != nil || serial == "" {
			continue
		}
		if !slices.Contains(used[serial], slot) {
			used[serial] = append(used[serial], slot)
		}
	}
	return used
}

// textAfter returns what follows the first occurrence of marker in s.
func textAfter(s, marker string) (string, bool) {
	_, after, ok := strings.Cut(s, marker)
	return after, ok
}

// ParseRecipient extracts the age recipient from an identity file: the
// "# Recipient: age1..." comment the plugin writes, or any bare age1 line.
func ParseRecipient(identity []byte) (string, error) {
	for line := range strings.Lines(string(identity)) {
		line = strings.TrimSpace(line)
		if rest, ok := textAfter(line, "Recipient:"); ok {
			if recipient := strings.TrimSpace(rest); strings.HasPrefix(recipient, "age1") {
				return recipient, nil
			}
		}
		if !strings.HasPrefix(line, "#") && strings.HasPrefix(line, "age1") {
			return line, nil
		}
	}
	return "", fmt.Errorf("no age recipient found in the identity output")
}

// AppendRecipient adds recipient to the profile's recipients file with a
// comment naming the hardware, once — re-enrolling the same key twice must
// not double-encrypt to it.
func AppendRecipient(repo, profile, serial string, slot int, recipient string) error {
	path := RecipientsPath(repo, profile)
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if slices.Contains(strings.Fields(string(existing)), recipient) {
		return nil
	}
	var b strings.Builder
	b.Write(existing)
	if len(existing) > 0 && existing[len(existing)-1] != '\n' {
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "# yubikey %s slot %d\n%s\n", serial, slot, recipient)
	return cli.AtomicWriteFile(path, []byte(b.String()), 0o644)
}

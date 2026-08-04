// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package privdot

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

// listAllOutput is a canned age-plugin-yubikey --list-all listing: two
// identities on one key, one on another.
const listAllOutput = `#       Serial: 17741369, Slot: 1
#         Name: age identity 8d24a1f3
#      Created: Sat, 05 Mar 2022 10:01:22 +0000
#   PIN policy: Once   (A PIN is required once per session, if set)
# Touch policy: Cached (A physical touch is required for decryption, and is cached for 15 seconds)
age1yubikey1q0exampleexampleexampleexampleexampleexampleexampleexample

#       Serial: 17741369, Slot: 3
#         Name: dotty-personal
age1yubikey1q1exampleexampleexampleexampleexampleexampleexampleexample

#       Serial: 23899167, Slot: 1
#         Name: work
age1yubikey1q2exampleexampleexampleexampleexampleexampleexampleexample
`

const identityOutput = `#       Serial: 17741369, Slot: 2
#         Name: dotty-personal
#      Created: Tue, 04 Aug 2026 10:00:00 +0000
#   PIN policy: Once
# Touch policy: Cached
#    Recipient: age1yubikey1qfreshrecipientexampleexampleexampleexampleexample
AGE-PLUGIN-YUBIKEY-1STUBEXAMPLE
`

// enrollFake scripts the three plugin invocations Enroll makes.
type enrollFake struct {
	calls   [][]string
	missing bool
}

func (r *enrollFake) LookPath(name string) (string, error) {
	if r.missing {
		return "", errors.New(name + " not found in PATH")
	}
	return "/fake/" + name, nil
}

func (r *enrollFake) Output(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	if args[0] == "--list-all" {
		return []byte(listAllOutput), nil
	}
	return []byte(identityOutput), nil
}

func (r *enrollFake) RunInteractive(_ context.Context, name string, args ...string) error {
	r.calls = append(r.calls, append([]string{name}, args...))
	return nil
}

func TestParseUsedSlots(t *testing.T) {
	used := ParseUsedSlots([]byte(listAllOutput))
	want := map[string][]int{"17741369": {1, 3}, "23899167": {1}}
	if !reflect.DeepEqual(used, want) {
		t.Errorf("ParseUsedSlots = %v, want %v", used, want)
	}
	if got := ParseUsedSlots(nil); len(got) != 0 {
		t.Errorf("empty listing should parse empty, got %v", got)
	}
}

func TestParseRecipient(t *testing.T) {
	got, err := ParseRecipient([]byte(identityOutput))
	if err != nil {
		t.Fatalf("ParseRecipient: %v", err)
	}
	if want := "age1yubikey1qfreshrecipientexampleexampleexampleexampleexample"; got != want {
		t.Errorf("recipient = %q, want %q", got, want)
	}
	if _, err := ParseRecipient([]byte("# nothing here\n")); err == nil {
		t.Error("no recipient should be an error")
	}
}

func TestEnroll(t *testing.T) {
	repo := seedRepo(t, nil)
	r := &enrollFake{}
	recipient, slot, err := Enroll(context.Background(), r, repo, "personal",
		EnrollOptions{Serial: "17741369"})
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	// Slots 1 and 3 are taken on this serial; the first free is 2.
	if slot != 2 {
		t.Errorf("slot = %d, want 2", slot)
	}
	generate := r.calls[1]
	want := []string{yubikeyPlugin, "--generate", "--serial", "17741369", "--slot", "2",
		"--name", "dotty-personal", "--pin-policy", "once", "--touch-policy", "cached"}
	if !reflect.DeepEqual(generate, want) {
		t.Errorf("generate argv = %v, want %v", generate, want)
	}

	stub, err := os.ReadFile(IdentityPath(repo, "personal", "17741369"))
	if err != nil || !strings.Contains(string(stub), "AGE-PLUGIN-YUBIKEY-") {
		t.Errorf("identity stub = %q, %v", stub, err)
	}
	recipients, err := os.ReadFile(RecipientsPath(repo, "personal"))
	if err != nil || !strings.Contains(string(recipients), recipient) {
		t.Errorf("recipients = %q, %v", recipients, err)
	}
	if !strings.Contains(string(recipients), "# yubikey 17741369 slot 2") {
		t.Errorf("recipients missing the hardware comment: %q", recipients)
	}

	// Re-enrolling the same recipient must not duplicate it.
	if err := AppendRecipient(repo, "personal", "17741369", 2, recipient); err != nil {
		t.Fatal(err)
	}
	recipients2, _ := os.ReadFile(RecipientsPath(repo, "personal"))
	if strings.Count(string(recipients2), recipient) != 1 {
		t.Errorf("recipient duplicated: %q", recipients2)
	}
}

func TestEnrollOccupiedSlot(t *testing.T) {
	repo := seedRepo(t, nil)
	_, _, err := Enroll(context.Background(), &enrollFake{}, repo, "personal",
		EnrollOptions{Serial: "17741369", Slot: 3})
	if err == nil || !strings.Contains(err.Error(), "already holds") {
		t.Errorf("occupied slot should refuse, got %v", err)
	}
}

func TestEnrollMissingPlugin(t *testing.T) {
	repo := seedRepo(t, nil)
	r := &enrollFake{missing: true}
	if _, _, err := Enroll(context.Background(), r, repo, "personal",
		EnrollOptions{Serial: "17741369"}); err == nil {
		t.Error("missing plugin should fail before any exec")
	}
	if len(r.calls) != 0 {
		t.Errorf("no exec should happen without the plugin, got %v", r.calls)
	}
}

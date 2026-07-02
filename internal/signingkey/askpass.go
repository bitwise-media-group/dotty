// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package signingkey

import (
	"context"
	"strings"
)

// PinentryPath is the pinentry-mac binary the ask-pass bridge drives. The
// bridge runs in the minimal environment ssh hands $SSH_ASKPASS, so the path is
// absolute rather than resolved on PATH; it is a var so tests can point it
// elsewhere.
var PinentryPath = "/opt/homebrew/bin/pinentry-mac"

// assuanRunner feeds an Assuan command block to pinentry's stdin and returns
// its stdout. A non-zero exit is not an error: pinentry exits non-zero when the
// user cancels, which surfaces here as an empty PIN.
type assuanRunner interface {
	RunAssuan(ctx context.Context, stdin, name string, args ...string) (string, error)
}

// AskPassReply computes the reply for an OpenSSH SSH_ASKPASS prompt. FIDO2
// user-presence prompts only need a non-error reply, so the reply is empty and
// pinentry is never run; PIN prompts are forwarded to pinentry-mac and the
// entered PIN is returned. keyinfo is the signing key's SHA256 fingerprint
// (from $DOTTY_SSH_KEYINFO) and read the file probe resolveKeyInfo uses to
// fingerprint the key file a client-auth prompt names — together they identify
// the key so pinentry can cache its PIN in the macOS keychain.
func AskPassReply(
	ctx context.Context,
	r assuanRunner,
	prompt, keyinfo string,
	read func(string) ([]byte, error),
) string {
	// FIDO2 user-presence: ssh only needs a non-error reply, not a PIN.
	if strings.HasPrefix(prompt, "Confirm user presence") {
		return ""
	}
	// Ignore pinentry's exit status: a cancel yields no data line and an empty
	// PIN, which ssh treats as a failed unlock.
	out, _ := r.RunAssuan(ctx, buildAssuan(prompt, resolveKeyInfo(prompt, keyinfo, read)), PinentryPath)
	return extractPin(out)
}

// resolveKeyInfo returns the SHA256 fingerprint of the key a PIN prompt
// unlocks, or "" when the key can't be identified. The fingerprint comes from
// the first of: keyinfo, supplied out-of-band by `signing-key sign` because
// ssh-keygen's agentless prompt names no key; the prompt text, where
// agent-style prompts embed "SHA256:<fp>"; or the prompt's key path — ssh's
// client-auth prompt names the key file, whose .pub sidecar is fingerprinted.
// read keeps the file probe injectable for tests.
func resolveKeyInfo(prompt, keyinfo string, read func(string) ([]byte, error)) string {
	if keyinfo != "" {
		return keyinfo
	}
	if fp := promptFingerprint(prompt); fp != "" {
		return fp
	}
	path := promptKeyPath(prompt)
	if path == "" {
		return ""
	}
	pub, err := read(path + ".pub")
	if err != nil {
		return "" // no caching, but the prompt still works
	}
	return fingerprintB64(pubKeyBlob(string(pub)))
}

// promptKeyPath extracts the key file path from ssh's client-auth PIN prompt
// ("Enter PIN for <TYPE> key <path>: "), or "" for any other prompt shape —
// ssh-keygen's agentless variant ends at "key:" and names no path.
func promptKeyPath(prompt string) string {
	rest, ok := strings.CutPrefix(prompt, "Enter PIN for ")
	if !ok {
		return ""
	}
	_, path, ok := strings.Cut(rest, " key ")
	if !ok {
		return ""
	}
	return strings.TrimSuffix(strings.TrimSpace(path), ":")
}

// buildAssuan renders the pinentry Assuan command block for a PIN prompt. A
// non-empty keyinfo (the key's SHA256 fingerprint) is sent as SETKEYINFO
// together with allow-external-password-cache — what lets pinentry-mac store
// and reuse the PIN in the macOS keychain. Without it the prompt is bare and
// the keychain is never consulted.
func buildAssuan(prompt, keyinfo string) string {
	if keyinfo != "" {
		return "SETDESC " + prompt + "\n" +
			"OPTION allow-external-password-cache\n" +
			"SETKEYINFO s/" + keyinfo + "\n" +
			"GETPIN\n"
	}
	return "SETDESC " + prompt + "\nGETPIN\n"
}

// promptFingerprint extracts the SHA256 fingerprint an agent-style prompt
// embeds ("… SHA256:<fp>: …"), or "" when none is present.
func promptFingerprint(prompt string) string {
	hashType, rest, _ := strings.Cut(prompt, ":")
	if !strings.HasSuffix(hashType, "SHA256") {
		return ""
	}
	fp, _, _ := strings.Cut(rest, ":")
	return fp
}

// extractPin pulls the PIN out of pinentry's Assuan response: the data lines
// (those containing "D"), joined and stripped of the leading "D " marker.
func extractPin(out string) string {
	var b strings.Builder
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "D") {
			b.WriteString(line)
		}
	}
	joined := b.String()
	if len(joined) < 2 {
		return ""
	}
	return joined[2:]
}

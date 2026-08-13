// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package brewfile

import "strings"

// canonicalName mirrors Homebrew's per-type Brewfile name sanitisation
// (bundle/dsl.rb sanitize_*_name) so verbatim user arguments compare equal to
// the names `brew bundle list` prints: formulae are downcased with the
// `homebrew-` tap-repo prefix stripped and homebrew/homebrew/* collapsed,
// casks reduce to their downcased short token, taps are downcased with the
// `homebrew-` prefix stripped. Extension kinds (vscode, go, ...) are stored
// verbatim by brew and pass through unchanged.
func canonicalName(kind Kind, name string) string {
	switch kind {
	case KindFormula:
		name = strings.ToLower(name)
		parts := strings.Split(name, "/")
		if len(parts) != 3 {
			return name
		}
		if parts[0] == "homebrew" && parts[1] == "homebrew" {
			return parts[2]
		}
		return parts[0] + "/" + strings.Replace(parts[1], "homebrew-", "", 1) + "/" + parts[2]
	case KindCask:
		if parts := strings.SplitN(name, "/", 3); len(parts) == 3 {
			name = parts[2]
		}
		return strings.ToLower(name)
	case KindTap:
		name = strings.ToLower(name)
		if parts := strings.Split(name, "/"); len(parts) == 2 {
			return parts[0] + "/" + strings.TrimPrefix(parts[1], "homebrew-")
		}
		return name
	default:
		return name
	}
}

// trustStoreName normalises name for comparison against `brew trust --json v1`
// output: Homebrew stores downcased names with the tap repo's `homebrew-`
// prefix stripped. Unlike canonicalName, casks keep their full qualification —
// the trust store is keyed by full name, the Brewfile lister by short token.
func trustStoreName(name string) string {
	name = strings.ToLower(name)
	parts := strings.Split(name, "/")
	if len(parts) < 2 {
		return name
	}
	parts[1] = strings.TrimPrefix(parts[1], "homebrew-")
	return strings.Join(parts, "/")
}

// dslWord is the Brewfile DSL keyword for kind: `brew` for formulae (brew
// bundle's --formula flag maps to the :brew entry type), the kind's own name
// for everything else.
func dslWord(kind Kind) string {
	if kind == KindFormula {
		return "brew"
	}
	return string(kind)
}

// wordKinds is the inverse of dslWord: each Brewfile DSL keyword mapped back
// to its entry kind.
var wordKinds = func() map[string]Kind {
	kinds := []Kind{KindFormula, KindCask, KindTap, KindVSCode, KindGo,
		KindCargo, KindUV, KindFlatpak, KindKrew, KindNPM, KindMAS}
	m := make(map[string]Kind, len(kinds))
	for _, k := range kinds {
		m[dslWord(k)] = k
	}
	return m
}()

// LineKey returns a comparison key for a Brewfile entry line: the DSL word
// plus the canonicalised name, so the same package compares equal regardless
// of trailing options (`, trusted: true`), casing, or tap qualification.
// Comments, blanks, and lines that are not entries return "".
func LineKey(line string) string {
	word, rest, cut := strings.Cut(strings.TrimSpace(line), " ")
	kind, known := wordKinds[word]
	if !cut || !known {
		return ""
	}
	rest = strings.TrimSpace(rest)
	if !strings.HasPrefix(rest, `"`) {
		return ""
	}
	name, _, closed := strings.Cut(rest[1:], `"`)
	if !closed || name == "" {
		return ""
	}
	return word + " " + canonicalName(kind, name)
}

// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package env

import (
	"fmt"
	"regexp"
	"strings"
)

// assignRe matches a .env assignment, splitting it into the part that must be
// preserved verbatim (group 1: indentation, an optional "export", the key, and
// the "=" with its surrounding spaces), the key on its own (group 2), and the
// raw value region that follows (group 3). A line that does not match — a blank
// line, a comment, or anything malformed — is left untouched.
var assignRe = regexp.MustCompile(`^(\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*)(.*)$`)

// Capture is the inverse of Inject: it scans a .env document and, for every
// KEY=<literal value> assignment, replaces the value with a
// "{{ dotty://<namespace>/KEY }}" reference and records the value in the
// returned secrets map. Blank lines, comments, empty values, and values that
// are already references are copied through untouched, so the transform is
// idempotent. All formatting outside the rewritten values — indentation, an
// "export" prefix, quoting style, inline comments, and line endings — is
// preserved.
func Capture(src, namespace string) (string, map[string]string, error) {
	if err := ValidateNamespace(namespace); err != nil {
		return "", nil, err
	}
	secrets := map[string]string{}
	lines := strings.Split(src, "\n")
	for i, raw := range lines {
		// Carry a CRLF terminator through unchanged: parse the bare line, then
		// re-append the "\r" the split left on it.
		line, cr := raw, ""
		if strings.HasSuffix(line, "\r") {
			line, cr = line[:len(line)-1], "\r"
		}
		newLine, key, value, captured := captureLine(line, namespace)
		if captured {
			secrets[key] = value
		}
		lines[i] = newLine + cr
	}
	return strings.Join(lines, "\n"), secrets, nil
}

// captureLine rewrites a single .env line. It reports the (possibly rewritten)
// line, and when an assignment was externalized, the key, its value, and
// captured=true. A line that is not an assignment, has an empty value, or whose
// value is already a reference is returned verbatim with captured=false.
func captureLine(line, namespace string) (newLine, key, value string, captured bool) {
	m := assignRe.FindStringSubmatch(line)
	if m == nil {
		return line, "", "", false
	}
	prefix, key, region := m[1], m[2], m[3]
	value, suffix, ok := splitValue(region)
	if !ok || value == "" || looksLikeRef(value) {
		return line, "", "", false
	}
	ref := fmt.Sprintf("{{ %s%s/%s }}", refScheme, namespace, key)
	return prefix + ref + suffix, key, value, true
}

// splitValue parses a .env value region into the decoded value and a verbatim
// suffix (a trailing inline comment and any whitespace around it). A
// double-quoted value is unescaped, a single-quoted value is literal, and an
// unquoted value runs to a whitespace-preceded "#" comment or the line's end.
// ok is false only when a quote is left open, in which case the caller leaves
// the line untouched rather than risk mangling it.
func splitValue(region string) (value, suffix string, ok bool) {
	if region == "" {
		return "", "", true
	}
	switch region[0] {
	case '"', '\'':
		value, suffix, ok := scanQuoted(region, region[0] == '"')
		// A closing quote may be followed only by whitespace and an optional
		// comment; trailing junk (A="x"y) means the line is malformed, so the
		// caller leaves it untouched rather than emit an ambiguous rewrite.
		if !ok || !validSuffix(suffix) {
			return "", "", false
		}
		return value, suffix, true
	default:
		return scanBare(region)
	}
}

// validSuffix reports whether s is what may legitimately follow a quoted value:
// nothing, only whitespace, or a "#" comment introduced by whitespace. The
// leading whitespace before a comment is what keeps the rewrite idempotent —
// re-emitted after the "}}", it is exactly the gap scanBare needs to read the
// value back as a reference. Trailing junk (A="x" y) is rejected so the line is
// left untouched instead of producing an ambiguous rewrite.
func validSuffix(s string) bool {
	t := strings.TrimLeft(s, " \t")
	if t == "" {
		return true // empty or all whitespace
	}
	return t[0] == '#' && len(t) < len(s) // a comment, with whitespace before it
}

// scanBare reads an unquoted value, stopping at an inline comment introduced by
// a "#" that follows whitespace (so "pass#word" keeps the "#"). Trailing
// whitespace before the comment, and the comment itself, become the suffix.
func scanBare(region string) (value, suffix string, ok bool) {
	end := len(region)
	for i := 1; i < len(region); i++ {
		if region[i] == '#' && (region[i-1] == ' ' || region[i-1] == '\t') {
			end = i
			break
		}
	}
	value = strings.TrimRight(region[:end], " \t")
	return value, region[len(value):], true
}

// scanQuoted reads a value opened by region[0] (a single or double quote). With
// double=true the standard backslash escapes are decoded; single quotes are
// literal. Everything after the closing quote is returned as the suffix. ok is
// false when the closing quote is missing.
func scanQuoted(region string, double bool) (value, suffix string, ok bool) {
	quote := region[0]
	var b strings.Builder
	for i := 1; i < len(region); i++ {
		c := region[i]
		if double && c == '\\' && i+1 < len(region) {
			i++
			switch region[i] {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case '\\', '"':
				b.WriteByte(region[i])
			default:
				b.WriteByte('\\')
				b.WriteByte(region[i])
			}
			continue
		}
		if c == quote {
			return b.String(), region[i+1:], true
		}
		b.WriteByte(c)
	}
	return "", "", false
}

// looksLikeRef reports whether v is already a "{{ ... }}" reference, so Capture
// leaves an already-externalized value in place instead of nesting references.
func looksLikeRef(v string) bool {
	v = strings.TrimSpace(v)
	return strings.HasPrefix(v, "{{") && strings.HasSuffix(v, "}}")
}

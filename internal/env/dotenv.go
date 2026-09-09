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
// line, a comment, or anything malformed — carries no assignment.
var assignRe = regexp.MustCompile(`^(\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*)(.*)$`)

// Line is one KEY=value assignment scanned from a .env.dotty template. Value
// is the decoded literal when Ref is nil; otherwise the whole value was a
// single "{{ ... }}" reference and Ref names the credential. Err is set for a
// line that cannot be carried over as either — an unterminated quote, a
// malformed reference, or a reference mixed with literal text — and the
// other fields hold whatever was parsed before the problem.
type Line struct {
	// N is the 1-based line number in the source.
	N     int
	Key   string
	Value string
	Ref   *Reference
	Err   error
}

// Lines scans a .env document and returns its assignments in file order, so a
// later duplicate of a key wins the way a shell sourcing the file would
// behave. Each value is decoded with .env quoting rules — double quotes
// unescape, single quotes are literal, and an unquoted value runs to a
// whitespace-introduced "#" comment. Blank lines, comments, and lines that
// are not assignments are skipped; an empty value is kept, since "KEY="
// deliberately sets KEY to the empty string.
func Lines(src string) []Line {
	var result []Line
	n := 0
	for raw := range strings.Lines(src) {
		n++
		line := strings.TrimRight(raw, "\r\n")
		m := assignRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		l := Line{N: n, Key: m[2]}
		value, _, ok := splitValue(m[3])
		if !ok {
			l.Err = fmt.Errorf("unterminated quote in %q", m[3])
			result = append(result, l)
			continue
		}
		l.Value = value
		refs, whole, err := Refs(value)
		switch {
		case err != nil:
			l.Err = err
		case whole:
			l.Ref = &refs[0]
		case len(refs) > 0:
			l.Err = fmt.Errorf("value mixes a reference with literal text; " +
				"store the secret on its own and use fnox's default = \"...${KEY}...\" interpolation")
		}
		result = append(result, l)
	}
	return result
}

// splitValue parses a .env value region into the decoded value and a verbatim
// suffix (a trailing inline comment and any whitespace around it). A
// double-quoted value is unescaped, a single-quoted value is literal, and an
// unquoted value runs to a whitespace-preceded "#" comment or the line's end.
// ok is false only when a quote is left open or trailing junk follows a
// closing quote, in which case the line cannot be read unambiguously.
func splitValue(region string) (value, suffix string, ok bool) {
	if region == "" {
		return "", "", true
	}
	switch region[0] {
	case '"', '\'':
		value, suffix, ok := scanQuoted(region, region[0] == '"')
		// A closing quote may be followed only by whitespace and an optional
		// comment; trailing junk (A="x"y) means the line is malformed.
		if !ok || !validSuffix(suffix) {
			return "", "", false
		}
		return value, suffix, true
	default:
		return scanBare(region)
	}
}

// validSuffix reports whether s is what may legitimately follow a quoted value:
// nothing, only whitespace, or a "#" comment introduced by whitespace.
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

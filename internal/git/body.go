// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package git

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	bodyOpen  = "<!-- dotty-stack:v1 id=%s -->"
	bodyClose = "<!-- /dotty-stack -->"
)

var reBodyBlock = regexp.MustCompile(`(?s)<!-- dotty-stack:v1 id=([^\s>]+) -->\s*(.*?)\s*<!-- /dotty-stack -->`)

// BuildPRBody constructs a full PR body: stack block + separator + description.
func BuildPRBody(stackID, stackMarkdown, description string) string {
	var b strings.Builder
	fmt.Fprintf(&b, bodyOpen+"\n\n", stackID)
	b.WriteString(strings.TrimSpace(stackMarkdown))
	b.WriteString("\n\n")
	b.WriteString(bodyClose)
	b.WriteString("\n\n---\n\n")
	b.WriteString(strings.TrimSpace(description))
	if description != "" && !strings.HasSuffix(description, "\n") {
		b.WriteByte('\n')
	}
	return b.String()
}

// RewriteStackSection replaces only the marked stack block in an existing body,
// preserving the description after the close marker / --- separator.
func RewriteStackSection(existingBody, stackID, stackMarkdown string) string {
	open := fmt.Sprintf(bodyOpen, stackID)
	block := open + "\n\n" + strings.TrimSpace(stackMarkdown) + "\n\n" + bodyClose

	if reBodyBlock.MatchString(existingBody) {
		return reBodyBlock.ReplaceAllString(existingBody, block)
	}
	// No marker: prepend stack block and keep old body as description.
	desc := strings.TrimSpace(existingBody)
	return BuildPRBody(stackID, stackMarkdown, desc)
}

// ExtractDescription returns the user-owned portion below the stack markers.
func ExtractDescription(body string) string {
	loc := reBodyBlock.FindStringIndex(body)
	if loc == nil {
		return strings.TrimSpace(body)
	}
	rest := strings.TrimSpace(body[loc[1]:])
	rest = strings.TrimPrefix(rest, "---")
	return strings.TrimSpace(rest)
}

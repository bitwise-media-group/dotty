// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package privdot

import (
	"fmt"
	"os"
	"slices"
	"strings"
)

// ReadRecipients returns the age recipients listed in the file at path, in
// order and deduplicated, skipping blank lines and "#" comments — the format
// AppendRecipient writes. A missing file is returned as the os error so
// callers can tell "no private repository" from an unreadable one.
func ReadRecipients(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var recipients []string
	for line := range strings.Lines(string(data)) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !slices.Contains(recipients, line) {
			recipients = append(recipients, line)
		}
	}
	return recipients, nil
}

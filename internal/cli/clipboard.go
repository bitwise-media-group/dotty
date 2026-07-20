// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// clipboardCommand resolves the platform clipboard writer: pbcopy on macOS,
// clip on Windows, and the first of wl-copy, xclip, or xsel found elsewhere.
func clipboardCommand() (name string, args []string, err error) {
	switch runtime.GOOS {
	case "darwin":
		return "pbcopy", nil, nil
	case "windows":
		return "clip", nil, nil
	}
	candidates := []struct {
		name string
		args []string
	}{
		{"wl-copy", nil},
		{"xclip", []string{"-selection", "clipboard"}},
		{"xsel", []string{"--clipboard", "--input"}},
	}
	for _, c := range candidates {
		if _, err := exec.LookPath(c.name); err == nil {
			return c.name, c.args, nil
		}
	}
	return "", nil, errors.New("no clipboard tool found (install wl-copy, xclip, or xsel)")
}

// CopyToClipboard writes text to the system clipboard using the platform's
// native tool.
func CopyToClipboard(ctx context.Context, text string) error {
	name, args, err := clipboardCommand()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("copy to clipboard via %s: %w", name, err)
	}
	return nil
}

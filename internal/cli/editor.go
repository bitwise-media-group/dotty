// MIT License
//
// Copyright (c) 2026 Bitwise Media Group
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// interactiveRunner is the slice of ExecRunner the editor helpers consume;
// tests substitute a fake that writes into the file it is handed.
type interactiveRunner interface {
	RunInteractive(ctx context.Context, name string, args ...string) error
}

// EditorCommand resolves the user's editor: $VISUAL, then $EDITOR, then vi.
// The value is split on whitespace so entries like "code --wait" work.
func EditorCommand() (name string, args []string) {
	for _, env := range []string{"VISUAL", "EDITOR"} {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			fields := strings.Fields(v)
			return fields[0], fields[1:]
		}
	}
	return "vi", nil
}

// EditFile opens path in the user's editor and waits for it to exit.
func EditFile(ctx context.Context, r interactiveRunner, path string) error {
	name, args := EditorCommand()
	if err := r.RunInteractive(ctx, name, append(args, path)...); err != nil {
		return fmt.Errorf("edit %s: %w", path, err)
	}
	return nil
}

// EditTempFile seeds a temp file with initial, opens it in the user's editor,
// and returns the edited content with surrounding whitespace trimmed. The
// temp file is always removed.
func EditTempFile(ctx context.Context, r interactiveRunner, initial string) (string, error) {
	tmp, err := os.CreateTemp("", "dotty-edit-*.md")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	path := tmp.Name()
	defer os.Remove(path)

	if _, err := tmp.WriteString(initial); err != nil {
		tmp.Close()
		return "", fmt.Errorf("seed %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close %s: %w", path, err)
	}

	if err := EditFile(ctx, r, path); err != nil {
		return "", err
	}

	edited, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s back: %w", filepath.Base(path), err)
	}
	return strings.TrimSpace(string(edited)), nil
}

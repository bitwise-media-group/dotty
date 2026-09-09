// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package privdot

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadRecipients(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{name: "empty file", src: "", want: nil},
		{name: "comments and blanks skipped", src: "# yubikey 1 slot 1\nage1aaa\n\n# yubikey 2 slot 1\nage1bbb\n",
			want: []string{"age1aaa", "age1bbb"}},
		{name: "crlf and padding trimmed", src: "  age1aaa \r\nage1bbb\r\n", want: []string{"age1aaa", "age1bbb"}},
		{name: "duplicates collapse in order", src: "age1bbb\nage1aaa\nage1bbb\n", want: []string{"age1bbb", "age1aaa"}},
		{name: "no trailing newline", src: "age1aaa", want: []string{"age1aaa"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), RecipientsFile)
			if err := os.WriteFile(path, []byte(tt.src), 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := ReadRecipients(path)
			if err != nil {
				t.Fatalf("ReadRecipients: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReadRecipients(%q) = %v, want %v", tt.src, got, tt.want)
			}
		})
	}
}

// TestReadRecipientsMissing pins that an absent file surfaces the os error,
// so callers can treat "no private repository" as no recipients.
func TestReadRecipientsMissing(t *testing.T) {
	_, err := ReadRecipients(filepath.Join(t.TempDir(), "nope.txt"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("ReadRecipients(missing) err = %v, want fs.ErrNotExist", err)
	}
}

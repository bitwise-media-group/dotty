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
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func newTestRunner() (*ExecRunner, *bytes.Buffer, *bytes.Buffer) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	r := NewExecRunner(IOStreams{In: strings.NewReader(""), Out: out, ErrOut: errOut}, nil)
	return r, out, errOut
}

func TestExecRunnerOutput(t *testing.T) {
	r, _, _ := newTestRunner()
	ctx := context.Background()

	t.Run("captures stdout", func(t *testing.T) {
		got, err := r.Output(ctx, "sh", "-c", "echo hello")
		if err != nil {
			t.Fatalf("Output() error: %v", err)
		}
		if string(got) != "hello\n" {
			t.Errorf("stdout = %q", got)
		}
	})

	t.Run("folds stderr into the error", func(t *testing.T) {
		_, err := r.Output(ctx, "sh", "-c", "echo boom >&2; exit 3")
		if err == nil {
			t.Fatal("Output() error = nil, want failure")
		}
		if !strings.Contains(err.Error(), "boom") {
			t.Errorf("error %q does not contain stderr text", err)
		}
	})
}

func TestExecRunnerRun(t *testing.T) {
	r, out, errOut := newTestRunner()

	if err := r.Run(context.Background(), "sh", "-c", "echo to-out; echo to-err >&2"); err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if out.String() != "to-out\n" {
		t.Errorf("Out = %q", out.String())
	}
	if errOut.String() != "to-err\n" {
		t.Errorf("ErrOut = %q", errOut.String())
	}
}

func TestExecRunnerRunInteractive(t *testing.T) {
	t.Run("exit code carried in ExitError", func(t *testing.T) {
		r, _, _ := newTestRunner()
		err := r.RunInteractive(context.Background(), "sh", "-c", "exit 42")
		var exitErr *ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("error %v is not an *ExitError", err)
		}
		if exitErr.Code != 42 {
			t.Errorf("Code = %d, want 42", exitErr.Code)
		}
	})

	t.Run("stdin is connected", func(t *testing.T) {
		out := &bytes.Buffer{}
		r := NewExecRunner(IOStreams{In: strings.NewReader("ping\n"), Out: out, ErrOut: &bytes.Buffer{}}, nil)
		if err := r.RunInteractive(context.Background(), "sh", "-c", "read line; echo got-$line"); err != nil {
			t.Fatalf("RunInteractive() error: %v", err)
		}
		if out.String() != "got-ping\n" {
			t.Errorf("Out = %q", out.String())
		}
	})
}

func TestExecRunnerLookPath(t *testing.T) {
	r, _, _ := newTestRunner()
	if _, err := r.LookPath("sh"); err != nil {
		t.Errorf("LookPath(sh) error: %v", err)
	}
	if _, err := r.LookPath("definitely-not-a-real-program-xyz"); err == nil {
		t.Error("LookPath(missing) error = nil, want install hint")
	}
}

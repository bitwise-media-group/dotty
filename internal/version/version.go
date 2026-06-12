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

// Package version exposes build metadata stamped into the binary at link time.
// Keep these as vars (not consts) so -ldflags "-X ..." can rewrite them; an
// unbuilt or `go run` binary reports the defaults.
package version

var (
	// Version is the semver release tag, e.g. v1.2.3.
	Version = "dev"
	// Commit is the short git SHA the binary was built from.
	Commit = "none"
	// BuildDate is the RFC3339 UTC build timestamp.
	BuildDate = "unknown"
)

// String renders the stamped metadata as a single human-readable line.
func String() string {
	return Version + " (commit " + Commit + ", built " + BuildDate + ")"
}

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

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// DocsFlags holds the flags for the hidden docs command.
type DocsFlags struct {
	Out    string
	Format string
}

var docsFlags = DocsFlags{}

// docsCmd regenerates the committed CLI reference. It lives as a hidden
// command (rather than the usual standalone docgen helper) because the flat
// cmd/ layout makes this package main, which nothing can import.
var docsCmd = &cobra.Command{
	Use:    "docs",
	Short:  "Generate the CLI reference from the command tree.",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		rootCmd.DisableAutoGenTag = true // keep the output reproducible
		if err := os.MkdirAll(docsFlags.Out, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", docsFlags.Out, err)
		}
		switch docsFlags.Format {
		case "markdown":
			return doc.GenMarkdownTree(rootCmd, docsFlags.Out)
		case "man":
			return doc.GenManTree(rootCmd, &doc.GenManHeader{Title: "DOTTY", Section: "1"}, docsFlags.Out)
		case "rest":
			return doc.GenReSTTree(rootCmd, docsFlags.Out)
		default:
			return fmt.Errorf("unknown format %q (expected markdown, man, or rest)", docsFlags.Format)
		}
	},
}

func init() {
	docsCmd.Flags().StringVar(&docsFlags.Out, "out", "docs/cli", "directory to write the reference into")
	docsCmd.Flags().StringVar(&docsFlags.Format, "format", "markdown", "output format: markdown, man, or rest")
	rootCmd.AddCommand(docsCmd)
}

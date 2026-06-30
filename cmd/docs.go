// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// DocsFlags holds the flags for the hidden docs command.
type DocsFlags struct {
	Out    string
	Format string
}

var docsFlags = DocsFlags{}

// manDate pins the man-page .TH date so generation is byte-for-byte
// reproducible. Cobra otherwise stamps time.Now(), churning every page's date
// header on each regeneration. Bump this on a meaningful documentation revision.
var manDate = time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)

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
			return doc.GenManTree(rootCmd, &doc.GenManHeader{Title: "DOTTY", Section: "1", Date: &manDate}, docsFlags.Out)
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

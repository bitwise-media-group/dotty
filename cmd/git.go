// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"github.com/spf13/cobra"
)

var gitCmd = &cobra.Command{
	Use:   "git <verb>",
	Short: "Git helpers built on dotty's commit signing.",
	Long: `Helpers that drive git through dotty's hardware-backed signing. Set signing
up first with ` + "`dotty signing-key sign --print-git-config`" + `.`,
	Example: `  dotty git resign HEAD~3
  dotty git resign --root --reset-author`,
}

func init() {
	rootCmd.AddCommand(gitCmd)
}

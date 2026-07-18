// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"strconv"

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

// stackHops parses the optional [num] argument shared by up/down (default 1).
func stackHops(args []string) (int, error) {
	if len(args) == 0 {
		return 1, nil
	}
	n, err := strconv.Atoi(args[0])
	if err != nil || n < 1 {
		return 0, errors.New("num must be a positive integer")
	}
	return n, nil
}

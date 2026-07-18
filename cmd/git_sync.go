// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/git"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

var gitSyncFlags struct {
	Continue bool
	Abort    bool
	Yes      bool
}

var gitSyncCmd = &cobra.Command{
	Use:   "sync [--continue | --abort]",
	Short: "Fetch trunk, clean merged layers, refresh PR maps, rebase+resign if diverged.",
	Long: `Synchronise the current stack with trunk:

  1. Fetch upstream/origin
  2. Drop layers already on trunk (default: delete local + origin branches)
  3. If any open layer diverged from trunk — or no longer contains the layer
     below it, because new commits landed mid-stack — prompt to rebase the
     open stack and re-sign each rewritten layer (use --continue / --abort
     around conflicts)
  4. Force-with-lease push the open branches
  5. Refresh the stack visualisation on every open PR

Config: git config dotty.stack.cleanup false  # keep merged branches`,
	Example: `  dotty git sync
  dotty git sync --continue
  dotty git sync --yes`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if gitSyncFlags.Continue && gitSyncFlags.Abort {
			return errors.New("--continue and --abort are mutually exclusive")
		}
		ios := cli.System()
		r := newRunner(ios)
		ctx := cmd.Context()

		if gitSyncFlags.Abort {
			if err := git.AbortSyncRebase(ctx, r); err != nil {
				return err
			}
			tui.Successf(ios, "Aborted stack rebase")
			return nil
		}

		if gitSyncFlags.Continue {
			return continueStackSync(cmd, ios, r)
		}

		s, err := git.LoadStackForHEAD(ctx, r)
		if err != nil {
			return err
		}
		trunk, err := git.ResolveTrunk(ctx, r)
		if err != nil {
			return err
		}
		if err := git.FetchTrunk(ctx, r, trunk); err != nil {
			tui.Warnf(ios, "Fetch trunk: %v", err)
		}
		_ = git.FetchPushRemote(ctx, r)

		cur, err := git.CurrentBranch(ctx, r)
		if err != nil {
			return err
		}
		cfg := git.DefaultCleanup(ctx, r)

		// Cleanup merged layers until none remain merged.
		for {
			rows := git.Status(ctx, r, s, trunk, cur)
			// RelIdentical means empty layer still at trunk tip — not merged work.
			i := slices.IndexFunc(rows, func(row git.LayerStatus) bool {
				return row.Relation == git.RelMerged
			})
			if i < 0 {
				break
			}
			mergedBranch := rows[i].Branch
			tui.Infof(ios, "Merged: %s", mergedBranch)
			s, err = git.CleanupMergedLayer(ctx, r, s, mergedBranch, cfg)
			if err != nil {
				return err
			}
			if len(s.Layers) == 0 {
				tui.Successf(ios, "Stack fully merged; nothing left")
				return nil
			}
			cur, _ = git.CurrentBranch(ctx, r)
		}

		rows := git.Status(ctx, r, s, trunk, cur)
		diverged := git.AnyDiverged(rows)
		if diverged || git.AnyStale(ctx, r, rows) {
			reason := "stack diverged from " + trunk.Ref()
			if !diverged {
				reason = "upper layers are missing new commits from a layer below"
			}
			ok := gitSyncFlags.Yes
			if !ok {
				var cerr error
				ok, cerr = tui.ConfirmDefault(ios,
					strings.ToUpper(reason[:1])+reason[1:]+". Rebase + resign the open stack?",
					"Rewrites SHAs; each rewritten layer will be re-signed with your hardware key.",
					true)
				if cerr != nil {
					if errors.Is(cerr, tui.ErrNotInteractive) {
						return fmt.Errorf("%w (pass --yes to rebase non-interactively)", cerr)
					}
					return cerr
				}
			}
			if !ok {
				return fmt.Errorf("%s; re-run with confirmation to rebase+resign", reason)
			}
			if err := rebaseResignStack(ctx, ios, r, s, trunk); err != nil {
				return err
			}
			s, err = git.LoadStackForHEAD(ctx, r)
			if err != nil {
				return err
			}
		}

		return refreshOpenPRBodies(ctx, ios, r, s, trunk)
	},
}

func continueStackSync(cmd *cobra.Command, ios cli.IOStreams, r *cli.ExecRunner) error {
	ctx := cmd.Context()
	st, err := git.ReadRebaseState(ctx, r)
	if err != nil {
		return err
	}
	if st == nil {
		return errors.New("no stack rebase in progress (nothing to --continue)")
	}
	if err := r.RunInteractive(ctx, "git", "rebase", "--continue"); err != nil {
		return fmt.Errorf("rebase --continue: %w (fix conflicts, then re-run sync --continue)", err)
	}
	trunk := git.Trunk{Remote: st.TrunkRemote, Branch: st.TrunkBranch}
	if st.Index < len(st.OpenBranches) {
		branch := st.OpenBranches[st.Index]
		if err := git.Checkout(ctx, r, branch); err != nil {
			return err
		}
		parent := trunk.Ref()
		if st.Index > 0 {
			parent = st.OpenBranches[st.Index-1]
		}
		if err := resignRange(ctx, r, parent); err != nil {
			return err
		}
		st.Index++
	}
	for st.Index < len(st.OpenBranches) {
		branch := st.OpenBranches[st.Index]
		base := trunk.Ref()
		if st.Index > 0 {
			base = st.OpenBranches[st.Index-1]
		}
		st.Phase = "rebase"
		if err := git.WriteRebaseState(ctx, r, *st); err != nil {
			return err
		}
		before, err := git.RevParse(ctx, r, branch)
		if err != nil {
			return err
		}
		if err := git.RebaseOnto(ctx, r, branch, base); err != nil {
			return fmt.Errorf("%w\nfix conflicts, then: dotty git sync --continue", err)
		}
		after, err := git.RevParse(ctx, r, branch)
		if err != nil {
			return err
		}
		// No-op rebase: nothing rewritten, signatures intact — skip the resign.
		if after != before {
			if err := resignRange(ctx, r, base); err != nil {
				return err
			}
		}
		st.Index++
	}
	for _, b := range st.OpenBranches {
		if err := git.ForcePushBranch(ctx, r, b); err != nil {
			return err
		}
	}
	_ = git.ClearRebaseState(ctx, r)
	s, err := git.LoadStackForHEAD(ctx, r)
	if err != nil {
		return err
	}
	return refreshOpenPRBodies(ctx, ios, r, s, trunk)
}

func rebaseResignStack(ctx context.Context, ios cli.IOStreams, r *cli.ExecRunner,
	s git.Stack, trunk git.Trunk,
) error {
	cur, _ := git.CurrentBranch(ctx, r)
	rows := git.Status(ctx, r, s, trunk, cur)
	var open []string
	for _, row := range rows {
		if row.Relation != git.RelMerged {
			open = append(open, row.Branch)
		}
	}
	if len(open) == 0 {
		return nil
	}
	st := git.RebaseState{
		StackID:      s.ID,
		TrunkRemote:  trunk.Remote,
		TrunkBranch:  trunk.Branch,
		OpenBranches: open,
		Index:        0,
		Phase:        "rebase",
	}
	if err := git.WriteRebaseState(ctx, r, st); err != nil {
		return err
	}
	for i, branch := range open {
		base := trunk.Ref()
		if i > 0 {
			base = open[i-1]
		}
		st.Index = i
		st.Phase = "rebase"
		_ = git.WriteRebaseState(ctx, r, st)
		tui.Infof(ios, "Rebasing %s onto %s", branch, base)
		before, err := git.RevParse(ctx, r, branch)
		if err != nil {
			return err
		}
		if err := git.RebaseOnto(ctx, r, branch, base); err != nil {
			return fmt.Errorf("%w\nfix conflicts, then: dotty git sync --continue\nor:  dotty git sync --abort", err)
		}
		after, err := git.RevParse(ctx, r, branch)
		if err != nil {
			return err
		}
		// A no-op rebase rewrote nothing: signatures are intact, and forcing a
		// resign would rewrite (and re-sign, one key touch per commit) layers
		// that were never touched.
		if after == before {
			continue
		}
		tui.Infof(ios, "Re-signing %s", branch)
		if err := resignRange(ctx, r, base); err != nil {
			return err
		}
	}
	for _, b := range open {
		tui.Infof(ios, "Pushing %s", b)
		if err := git.ForcePushBranch(ctx, r, b); err != nil {
			return err
		}
	}
	return git.ClearRebaseState(ctx, r)
}

func resignRange(ctx context.Context, r *cli.ExecRunner, base string) error {
	opts := git.Options{Base: base}
	n, err := git.CommitCount(ctx, r, opts)
	if err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	return git.Resign(ctx, r, opts)
}

func refreshOpenPRBodies(ctx context.Context, ios cli.IOStreams, r *cli.ExecRunner,
	s git.Stack, trunk git.Trunk,
) error {
	cur, err := git.CurrentBranch(ctx, r)
	if err != nil {
		return err
	}
	rows := git.Status(ctx, r, s, trunk, cur)
	merged := git.MergeMap(rows)
	baseRemote := trunk.Remote
	prURL := git.PRURLBuilder(ctx, r, baseRemote)

	for i, layer := range s.Layers {
		if layer.PR == 0 || merged[layer.Branch] {
			continue
		}
		stackMD := git.FormatStackMap(s, layer.Branch, prURL, merged)
		desc := ""
		// Prefer preserving existing description from GitHub if we can — for now rebuild from title commit.
		if layer.TitleSHA != "" {
			parent := git.ParentRevForLayer(s, i, trunk)
			if commits, err := git.LayerCommits(ctx, r, parent, layer.Branch); err == nil {
				// Tolerate an abbreviated stored TitleSHA.
				if j := slices.IndexFunc(commits, func(c git.Commit) bool {
					return strings.HasPrefix(c.SHA, layer.TitleSHA)
				}); j >= 0 {
					desc = commits[j].Body
				}
			}
		}
		body := git.BuildPRBody(s.ID, stackMD, desc)
		if err := git.UpdatePRBody(ctx, r, layer.PR, body, baseRemote); err != nil {
			tui.Warnf(ios, "Refresh PR#%d: %v", layer.PR, err)
			continue
		}
		tui.Successf(ios, "Updated stack map on PR#%d", layer.PR)
	}

	// Refreshing bodies never changes layer relations, so the rows computed
	// above are still current for the closing status print.
	git.FormatStatus(ios.Out, s, trunk, rows)
	return nil
}

func init() {
	gitSyncCmd.Flags().BoolVar(&gitSyncFlags.Continue, "continue", false,
		"resume after resolving rebase conflicts")
	gitSyncCmd.Flags().BoolVar(&gitSyncFlags.Abort, "abort", false,
		"abort an in-progress stack rebase")
	gitSyncCmd.Flags().BoolVarP(&gitSyncFlags.Yes, "yes", "y", false,
		"rebase+resign without prompting when diverged")
	gitCmd.AddCommand(gitSyncCmd)
}

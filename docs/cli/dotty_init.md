## dotty init

Scaffold a new dotfiles repository and set up this machine.

### Synopsis

Create a dotfiles repository from the template embedded in dotty, driven by
a short wizard: where repositories live, which optional tools and coding
agents to include, and how to seed the Brewfile. ghostty, oh-my-posh, vivid,
zsh, and git config are always included.

Nothing is written until a summary is confirmed. init then renders the
repository — including the profile (answers, Brewfile, and the per-profile
renders of anything machine-class-specific) under profiles/<name>, so
profiles travel with the repo and are shared across machines of the same
class — stages it with git (the first commit is left for you to sign), links
the home/ tree into your home directory, activates the profile (the
active-profile symlink is the only machine-local state), and installs the
lobe-icons glyph font. Files already in the way of a link are resolved per
--on-conflict, and legacy files that shadow the rendered configuration from
outside any link site (~/.gitconfig, ~/.zshrc and the other bare zsh startup
files) are retired; backups land under $XDG_DATA_HOME/dotty/backups and are
restorable with dotty dotfiles restore.

Re-running init against an existing repository and profile walks the same
interview again with the stored answers as the defaults, so a machine class
can be extended (or trimmed) later; keeping every answer re-renders and
re-links idempotently, and a repository in the legacy layout is migrated to
the current one. Without a terminal the stored answers are taken as-is, so
scripted re-runs never prompt. With --yes a re-run reuses every stored
answer and skips the confirmation summary, asking only which profile to use
plus any question the stored profile predates. A new machine adopts a fresh
clone the same way — run init from inside it, or point --repo at it.

```
dotty init [flags]
```

### Examples

```
  dotty init
  dotty init --repo ~/Repos/dotfiles --addons=tmux,lsd --agents=claude-code --yes
```

### Options

```
      --addons strings            optional add-ons: nvim,btop,k9s,lazygit,lsd,tmux,yazi
      --agents strings            coding agents: claude-code,codex,opencode,antigravity,grok
      --allowed-serials strings   restrict the profile to these security-key serials
      --description string        short description of the profile
      --dump-brews                seed the Brewfile from the installed packages
      --git-email string          git identity email for the private git config
      --git-name string           git identity name for the private git config
      --harden                    confine the coding agents: sandbox, credential-read denies, ask-first permissions
  -h, --help                      help for init
      --macos-defaults strings    macOS defaults groups to apply (see the wizard picklist; empty for none)
      --marketplace               add the bitwise skills marketplace to the selected agents
      --on-conflict string        existing-file resolution: backup, adopt, skip, or fail (default "backup")
      --piv                       require smart-card (PIV) login system-wide
      --profile-name string       dotty profile to create (default machine name)
      --repo string               dotfiles repository path (default <repos-dir>/dotfiles)
      --repos-dir string          directory your repositories live in (default ~/Repos)
      --security-keys             this machine class signs with hardware security keys
      --wallpaper string          wallpaper image from ~/.local/share/wallpapers
      --worktrees string          agent worktree location: a directory name inside each repo (default .worktrees) or an absolute path
      --yes                       skip the confirmation summary and reuse stored answers; only unanswered questions are asked
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty](dotty.md)	 - Utilities for a terminal-driven workflow and dotfiles.


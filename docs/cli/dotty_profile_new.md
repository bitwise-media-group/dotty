## dotty profile new

Create a profile and set this machine up as one of its class.

### Synopsis

Create a profile by running the init interview for it: a profile directory
with nothing in it is a machine class that configures nothing, so new is
dotty init with the profile name settled in advance. Without --name dotty
prompts for one, then asks init's questions — where the dotfiles repository
lives, which add-ons and coding agents to include, security keys, macOS
defaults — renders profiles/<name> into the repository, links the home/ tree,
and activates the profile. Nothing is written until the summary is confirmed.

Every init flag works here too, so a profile can be created unattended. The
name has to be free: an existing profile is re-rendered with dotty init and
switched to with dotty profile activate.

```
dotty profile new [flags]
```

### Examples

```
  dotty profile new
  dotty profile new --name=work --description="work laptop"
  dotty profile new --name=work --addons=tmux --agents=claude-code --yes
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
  -h, --help                      help for new
      --macos-defaults strings    macOS defaults groups to apply (see the wizard picklist; empty for none)
      --marketplace               add the bitwise skills marketplace to the selected agents
      --name string               name for the new profile
      --on-conflict string        existing-file resolution: backup, adopt, skip, or fail (default "backup")
      --piv                       require smart-card (PIV) login system-wide
      --private-repo string       encrypted private dotfiles repository path (empty for none)
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

* [dotty profile](dotty_profile.md)	 - Manage system profiles that travel across machines.


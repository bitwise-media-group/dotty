## dotty

Utilities for a terminal-driven workflow and dotfiles.

### Synopsis

dotty manages the moving parts of a terminal-centric machine setup:
system profiles that travel across machines, the mise-managed packages that
keep installs reproducible, named aliases for hardware security keys, and SSH
signing keys that live on those keys (including git commit signing).

### Examples

```
  dotty profile new --name=work
  dotty packages add brew-cask:ghostty
  dotty security-key add --name=primary
  dotty signing-key new
```

### Options

```
  -h, --help             help for dotty
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty completion](dotty_completion.md)	 - Generate the autocompletion script for the specified shell
* [dotty dotfiles](dotty_dotfiles.md)	 - Operate on the dotfiles repository dotty init generated.
* [dotty env](dotty_env.md)	 - Migrate legacy keychain credentials to fnox.
* [dotty git](dotty_git.md)	 - Git helpers built on dotty's commit signing.
* [dotty init](dotty_init.md)	 - Scaffold a new dotfiles repository and set up this machine.
* [dotty packages](dotty_packages.md)	 - Manage the profile's packages (mise tools and bootstrap packages).
* [dotty private](dotty_private.md)	 - Manage the encrypted private dotfiles repository.
* [dotty profile](dotty_profile.md)	 - Manage system profiles that travel across machines.
* [dotty security-key](dotty_security-key.md)	 - Manage hardware security keys.
* [dotty signing-key](dotty_signing-key.md)	 - Create and use SSH signing keys on hardware security keys.
* [dotty tmux](dotty_tmux.md)	 - Tmux sessions for agent-driven development.
* [dotty worktree](dotty_worktree.md)	 - Manage agent git worktrees.


## dotty

Utilities for a terminal-driven workflow and dotfiles.

### Synopsis

dotty manages the moving parts of a terminal-centric machine setup:
system profiles that travel across machines, the Homebrew Brewfile that keeps
installs reproducible, named aliases for hardware security keys, and SSH
signing keys that live on those keys (including git commit signing).

### Examples

```
  dotty profile new --name=work
  dotty brewfile add --cask ghostty
  dotty security-key add --name=primary
  dotty signing-key new
```

### Options

```
  -h, --help             help for dotty
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty brewfile](dotty_brewfile.md)	 - Manage the profile's Brewfile for reproducible brews.
* [dotty completion](dotty_completion.md)	 - Generate the autocompletion script for the specified shell
* [dotty env](dotty_env.md)	 - Store and inject credentials from the macOS Keychain.
* [dotty git](dotty_git.md)	 - Git helpers built on dotty's commit signing.
* [dotty profile](dotty_profile.md)	 - Manage system profiles that travel across machines.
* [dotty security-key](dotty_security-key.md)	 - Manage hardware security keys.
* [dotty signing-key](dotty_signing-key.md)	 - Create and use SSH signing keys on hardware security keys.


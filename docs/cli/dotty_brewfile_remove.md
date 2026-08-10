## dotty brewfile remove

Remove entries from the Brewfile.

### Synopsis

Remove one or more entries from the Brewfile, or pick several interactively
(a filterable checklist) when no name is given. Entries default to formulae;
pass a type flag for anything else. Trust grants recorded for removed
tap-qualified names (and taps) are revoked best-effort. Nothing is
uninstalled: removed entries stay on the machine until
`dotty brewfile sync` removes what the Brewfile no longer lists —
pass --sync to run it immediately.

```
dotty brewfile remove [--tap | --cask | --formula | ...] [<name> ...] [flags]
```

### Examples

```
  dotty brewfile remove ripgrep jq
  dotty brewfile remove --cask ghostty
  dotty brewfile rm --tap
  dotty brewfile remove --sync acme/tap/widget
```

### Options

```
      --cargo                      remove Cargo packages
      --cask                       remove casks
      --flatpak                    remove Flatpak packages
      --formula                    remove formulae (the default)
      --go                         remove Go packages
  -h, --help                       help for remove
      --krew                       remove Krew plugins
      --mas                        remove Mac App Store entries
      --npm                        remove npm packages
      --sync dotty brewfile sync   run dotty brewfile sync after removing
      --tap                        remove taps
      --uv                         remove uv tools
      --vscode                     remove VSCode extensions
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty brewfile](dotty_brewfile.md)	 - Manage the profile's Brewfile for reproducible brews.


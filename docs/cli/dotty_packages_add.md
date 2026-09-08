## dotty packages add

Add packages to the profile and install them.

### Synopsis

Record one or more packages in the profile's config.toml and install them.
Tools (aqua:owner/repo, github:owner/repo, npm:name, pipx:name, go:module,
cargo:crate, or a mise core tool like go or node) go through mise use and
are locked for every platform in mise.lock; bootstrap packages (brew:name,
brew-cask:name, mas:id, flatpak:id, apt:name, …) go through mise bootstrap
packages use and track their latest version. Ids the profile already
declares — in config.toml or a component fragment — are skipped rather than
duplicated; the profile is still installed. Bare names are refused: a
registry shorthand like git or flux resolves to whichever entry claims it
(git-chglog, flux-operator), so spell the backend.

```
dotty packages add <id> [...] [flags]
```

### Examples

```
  dotty packages add aqua:sharkdp/bat aqua:jqlang/jq
  dotty packages add brew-cask:raycast
  dotty packages add github:bitwise-media-group/evolve
  dotty packages add npm:prettier
```

### Options

```
  -h, --help   help for add
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty packages](dotty_packages.md)	 - Manage the profile's packages (mise tools and bootstrap packages).


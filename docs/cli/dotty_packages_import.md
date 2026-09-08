## dotty packages import

Import installed Homebrew formulae, or a Brewfile, into the profile.

### Synopsis

Record what is already on the machine in the profile's config.toml. By
default the Homebrew formulae installed on request become brew: bootstrap
packages (with the taps they need); --all includes every linked formula,
dependencies too. With --brewfile the given Brewfile is converted instead:
formulae the components already provide as tools are dropped, other formulae
become brew: packages, casks become brew-cask: packages restricted to
macOS, and entry types mise has no equivalent for are reported. Either way
entries the profile already declares are skipped, and new ones land under a
header comment so the import is idempotent.

```
dotty packages import [--all] [--brewfile <path>] [flags]
```

### Examples

```
  dotty packages import
  dotty packages import --all
  dotty packages import --brewfile ~/Brewfile
```

### Options

```
      --all               import every linked formula, dependencies included
      --brewfile string   convert this Brewfile instead of the installed formulae
  -h, --help              help for import
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty packages](dotty_packages.md)	 - Manage the profile's packages (mise tools and bootstrap packages).


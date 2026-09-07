## dotty private init

Scaffold or adopt the private repository.

### Synopsis

Create the private repository skeleton at path — the private marker, the
git attributes and ignore guards, a pre-commit hook running dotty private
verify, and a profile matching the active one — and record the path in the
active profile's answers so every machine of the class finds it. The profile's
home tree is seeded with the drop-in directories the public templates already
include and dotty private link deploys: .ssh/config.d (ssh host blocks) and
.config/private/git (the private git identity). An existing repository is
adopted: nothing already there is touched, so re-running init is always safe.
Without a path, a git repository at the working directory is preferred when
init may safely target it — it already carries the private marker, or it is a
fresh clone holding nothing a scaffold could disturb; otherwise the stored
answer is reused, falling back to dotfiles.private beside the public
repository.

```
dotty private init [path] [flags]
```

### Examples

```
  dotty private init ~/Repos/dotfiles.private
  cd ~/Repos/dotfiles.private && dotty private init
  dotty private init
```

### Options

```
  -h, --help   help for init
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
      --repo string      private repository (default: the active profile's stored answer)
```

### SEE ALSO

* [dotty private](dotty_private.md)	 - Manage the encrypted private dotfiles repository.


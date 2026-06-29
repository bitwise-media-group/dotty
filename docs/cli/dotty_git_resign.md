## dotty git resign

Rebase and re-sign commits up to a commitish.

### Synopsis

Rebase the commits up to a target and re-sign each one with your hardware
signing key. Pass --root to resign every commit from the start of history, or a
<commitish> to resign the commits after it (<commitish>..HEAD).

With --reset-author, each commit's author is also reset to your current
user.name/user.email, and any trailer naming the original author (for example
Co-authored-by: or Signed-off-by:) is rewritten to the new identity.

Resigning rewrites history: commits get new SHAs. It prompts for confirmation
unless --yes is given. Signing must already be configured — see
`dotty signing-key sign --print-git-config`.

```
dotty git resign [--root | <commitish>] [--reset-author] [flags]
```

### Examples

```
  dotty git resign HEAD~3
  dotty git resign --root --reset-author
  dotty git resign HEAD~5 --yes
```

### Options

```
  -h, --help           help for resign
      --reset-author   reset author to user.name/user.email and rewrite matching trailers
      --root           resign every commit from the start of history
  -y, --yes            skip the confirmation prompt
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty git](dotty_git.md)	 - Git helpers built on dotty's commit signing.


## dotty private status

Report each private entry's freshness, plus repository hygiene.

### Synopsis

Classify every entry the profile carries without touching hardware:
ok (everything agrees), stale (the repository changed; run dotty private
link), drifted (the decrypted copy was edited; run dotty private encrypt),
conflict (both changed; reconcile by hand), or missing (never decrypted
here). Repository hygiene findings — plaintext siblings, sensitive-looking
unencrypted files, half-finished enrollments — are reported for the whole
repository.

```
dotty private status [flags]
```

### Examples

```
  dotty private status
  dotty private status --profile work
```

### Options

```
  -h, --help   help for status
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
      --repo string      private repository (default: the active profile's stored answer)
```

### SEE ALSO

* [dotty private](dotty_private.md)	 - Manage the encrypted private dotfiles repository.


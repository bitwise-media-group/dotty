## dotty private verify

Check the repository for plaintext accidents.

### Synopsis

Scan for hygiene problems that must never be committed: a plaintext file
sitting beside its ciphertext, a sensitive-looking file that was never
encrypted, or a profile whose enrollment is half-finished. With --staged only
the files staged in git are checked — the mode the scaffolded pre-commit hook
runs — and content is never read, so the check is safe anywhere. Any finding
exits non-zero.

```
dotty private verify [flags]
```

### Examples

```
  dotty private verify
  dotty private verify --staged
```

### Options

```
  -h, --help     help for verify
      --staged   check only the files staged in git (pre-commit mode)
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
      --repo string      private repository (default: the active profile's stored answer)
```

### SEE ALSO

* [dotty private](dotty_private.md)	 - Manage the encrypted private dotfiles repository.


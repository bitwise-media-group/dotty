## dotty env migrate

Move dotty env credentials and templates into fnox.

### Synopsis

Set up fnox for dotty's use, then carry the legacy store into it. Setup is
idempotent: a software age identity is generated and parked in the macOS
Keychain, and the global config (~/.config/fnox/config.toml) gets an age
provider encrypting to it plus every security key enrolled for the private
dotfiles, so a backup key can decrypt on a machine without the identity.
Secrets only enter fnox exec subprocesses (env = "exec"), never the shell.

Every keychain namespace becomes a fnox profile in the global config (the
"default" namespace lands in the top-level secrets); --namespace limits the
set. Each PATH is a .env.dotty template (default: the one in the working
directory, when present) that becomes a fnox.toml beside it — literals as
defaults, {{ dotty://ns/KEY }} references as encrypted values. Keys fnox
already has are skipped unless --force, so re-running is safe. Failures are
reported per entry and never abort the run. The keychain items and templates
are left in place; --purge deletes a namespace's keychain item once every
credential in it migrated, after confirmation. Delete a template yourself
once fnox exec works in its directory.

```
dotty env migrate [PATH...] [flags]
```

### Examples

```
  dotty env migrate
  dotty env migrate --namespace aws --namespace ci --purge
  dotty env migrate --skip-keychain ./svc/.env.dotty
  dotty env migrate --dry-run
```

### Options

```
      --dry-run                 report what would be migrated without writing anything
      --force                   overwrite keys fnox already has
  -h, --help                    help for migrate
      --namespace stringArray   keychain namespace to migrate (repeatable; default: all)
      --purge                   delete each keychain namespace once every credential in it migrated
      --skip-keychain           migrate only the given templates, not the keychain
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty env](dotty_env.md)	 - Migrate legacy keychain credentials to fnox.


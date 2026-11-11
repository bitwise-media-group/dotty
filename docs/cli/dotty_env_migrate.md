## dotty env migrate

Move dotty env credentials into fnox, and template variables into mise.

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
directory, when present), split by what its lines hold. Plain KEY=value
lines are configuration, not secrets: they become [env] variables in the
mise.toml beside the template (trusted when dotty creates it), which mise
loads into the shell and its tasks — mise expands $VARS and {{ }} there, so
values carrying either are warned about. {{ dotty://ns/KEY }} references
become encrypted values in a fnox.toml beside it, created only when the
template has any. Keys already present are skipped unless --force, so
re-running is safe. Failures are reported per entry and never abort the run.
The keychain items and templates are left in place; --purge deletes a
namespace's keychain item once every credential in it migrated, after
confirmation. Delete a template yourself once mise (and fnox exec, for its
secrets) works in its directory.

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
      --dry-run[=true|false]         report what would be migrated without writing anything
      --force[=true|false]           overwrite keys fnox already has
  -h, --help[=true|false]            help for migrate
      --namespace stringArray        keychain namespace to migrate (repeatable; default: all)
      --purge[=true|false]           delete each keychain namespace once every credential in it migrated
      --skip-keychain[=true|false]   migrate only the given templates, not the keychain
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty env](dotty_env.md)	 - Migrate legacy dotty env credentials to fnox and mise.


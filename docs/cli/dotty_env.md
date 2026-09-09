## dotty env

Migrate legacy keychain credentials to fnox.

### Synopsis

Secrets are managed by fnox (https://fnox.jdx.dev): fnox set / get / list /
remove replace the old dotty env verbs, fnox exec runs a command with them in
its environment, and fnox export writes a .env for tools that insist on one.
dotty installs fnox with its packages, activates its shell hook, and sets up
an age provider whose identity lives in the macOS Keychain.

The one verb left here carries credentials out of the old dotty env store —
keychain namespaces and .env.dotty templates — into fnox.

```
dotty env <verb> [flags]
```

### Examples

```
  dotty env migrate
  dotty env migrate --namespace aws --purge
  fnox -P aws exec -- aws s3 ls
```

### Options

```
  -h, --help   help for env
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty](dotty.md)	 - Utilities for a terminal-driven workflow and dotfiles.
* [dotty env migrate](dotty_env_migrate.md)	 - Move dotty env credentials and templates into fnox.


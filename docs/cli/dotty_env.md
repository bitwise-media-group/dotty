## dotty env

Store and inject credentials from the macOS Keychain.

### Synopsis

Manage generic credentials in the macOS login keychain and inject them into
templates and processes — the way the 1Password CLI does, but with no external
service. Secrets are grouped into namespaces; each namespace is a single
keychain item under the service name "dotty:<namespace>". get reads one value
(like op read), use fills a template (like op inject), and run launches a
process with the namespace's secrets in its environment (like op run).

### Examples

```
  dotty env add --namespace=aws AWS_ACCESS_KEY_ID
  dotty env list --namespace=aws
  dotty env get --namespace=aws AWS_ACCESS_KEY_ID
  dotty env run --namespace=aws -- aws s3 ls
```

### Options

```
  -h, --help               help for env
      --namespace string   credential namespace to operate on (default "default")
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty](dotty.md)	 - Utilities for a terminal-driven workflow and dotfiles.
* [dotty env add](dotty_env_add.md)	 - Store a credential in a namespace.
* [dotty env get](dotty_env_get.md)	 - Print a credential value.
* [dotty env list](dotty_env_list.md)	 - List the credential names in a namespace.
* [dotty env remove](dotty_env_remove.md)	 - Remove credentials from a namespace.
* [dotty env run](dotty_env_run.md)	 - Run a command with a namespace's credentials in its environment.
* [dotty env use](dotty_env_use.md)	 - Fill a template with credential references.


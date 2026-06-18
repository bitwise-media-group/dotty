## dotty env remove

Remove credentials from a namespace.

### Synopsis

Remove one credential by KEY, the whole namespace with --all, or pick several
interactively (a filterable checklist) when no KEY is given. Removing the last
credential also removes the namespace's keychain item.

```
dotty env remove [<KEY>] [flags]
```

### Examples

```
  dotty env remove --namespace=aws AWS_ACCESS_KEY_ID
  dotty env remove --namespace=aws --all
  dotty env rm --namespace=aws
```

### Options

```
      --all    remove the entire namespace
  -h, --help   help for remove
```

### Options inherited from parent commands

```
      --namespace string   credential namespace to operate on (default "default")
      --profile string     profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty env](dotty_env.md)	 - Store and inject credentials from the macOS Keychain.


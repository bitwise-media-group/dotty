## dotty env list

List the credential names in a namespace.

### Synopsis

Print the key names stored in the namespace, one per line, sorted. Values are
never printed — use get, use, or run to read them.

```
dotty env list [flags]
```

### Examples

```
  dotty env list --namespace=aws
```

### Options

```
  -h, --help   help for list
```

### Options inherited from parent commands

```
      --namespace string   credential namespace to operate on (default "default")
      --profile string     profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty env](dotty_env.md)	 - Store and inject credentials from the macOS Keychain.


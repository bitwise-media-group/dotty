## dotty env get

Print a credential value.

### Synopsis

Print the value of a single credential to stdout. The argument is either a
KEY in the --namespace, or a full dotty://<namespace>/<key> reference (which
names its own namespace). A trailing newline is printed unless --no-newline.

```
dotty env get <KEY | dotty://namespace/key> [flags]
```

### Examples

```
  dotty env get --namespace=aws AWS_ACCESS_KEY_ID
  dotty env get dotty://aws/AWS_ACCESS_KEY_ID | pbcopy
```

### Options

```
  -h, --help         help for get
      --no-newline   do not print a trailing newline
```

### Options inherited from parent commands

```
      --namespace string   credential namespace to operate on (default "default")
      --profile string     profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty env](dotty_env.md)	 - Store and inject credentials from the macOS Keychain.


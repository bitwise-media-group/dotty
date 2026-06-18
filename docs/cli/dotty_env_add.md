## dotty env add

Store a credential, or capture a whole .env file.

### Synopsis

Store a credential under KEY in the namespace. With a terminal attached the
value is read from a hidden prompt; when input is piped, the value is read from
stdin (a single trailing newline is stripped). The value is never taken from a
flag, so it stays out of shell history and the process list.

With --in-file, KEY is omitted and a .env file is captured instead: every
KEY=value assignment is stored in the namespace and its value is replaced with a
{{ dotty://<namespace>/KEY }} reference (the inverse of env use). The result is
written to --out-file, which defaults to --in-file; replacing an existing file
is confirmed first. Blank lines, comments, empty values, and values that are
already references are left untouched.

```
dotty env add [<KEY>] [flags]
```

### Examples

```
  dotty env add --namespace=aws AWS_ACCESS_KEY_ID
  printf '%s' "$TOKEN" | dotty env add --namespace=ci GITHUB_TOKEN
  dotty env add --namespace=aws --in-file=.env
```

### Options

```
  -h, --help              help for add
      --in-file string    capture secrets from this .env file instead of a single KEY
      --out-file string   file to write captured references to (default: --in-file)
```

### Options inherited from parent commands

```
      --namespace string   credential namespace to operate on (default "default")
      --profile string     profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty env](dotty_env.md)	 - Store and inject credentials from the macOS Keychain.


## dotty env add

Store a credential in a namespace.

### Synopsis

Store a credential under KEY in the namespace. With a terminal attached the
value is read from a hidden prompt; when input is piped, the value is read from
stdin (a single trailing newline is stripped). The value is never taken from a
flag, so it stays out of shell history and the process list.

```
dotty env add <KEY> [flags]
```

### Examples

```
  dotty env add --namespace=aws AWS_ACCESS_KEY_ID
  printf '%s' "$TOKEN" | dotty env add --namespace=ci GITHUB_TOKEN
```

### Options

```
  -h, --help   help for add
```

### Options inherited from parent commands

```
      --namespace string   credential namespace to operate on (default "default")
      --profile string     profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty env](dotty_env.md)	 - Store and inject credentials from the macOS Keychain.


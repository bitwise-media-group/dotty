## dotty env run

Run a command with a namespace's credentials in its environment.

### Synopsis

Launch a command with every credential in the namespace exported as an
environment variable, the way op run does. dotty parses its own --namespace
(and --help); everything after -- is the command and its arguments, passed
through untouched. Put dotty's flags before -- (use -- when the command takes a
--namespace of its own). The command inherits the terminal, and dotty exits
with its exit code.

```
dotty env run -- <command> [args...] [flags]
```

### Examples

```
  dotty env run --namespace=aws -- aws s3 ls
  dotty env run --namespace=ci -- ./deploy.sh
```

### Options

```
  -h, --help   help for run
```

### Options inherited from parent commands

```
      --namespace string   credential namespace to operate on (default "default")
      --profile string     profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty env](dotty_env.md)	 - Store and inject credentials from the macOS Keychain.


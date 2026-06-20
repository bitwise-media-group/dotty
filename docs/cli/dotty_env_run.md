## dotty env run

Run a command with a namespace's credentials in its environment.

### Synopsis

Launch a command with every credential in the namespace exported as an
environment variable, the way op run does. dotty parses its own --namespace and
--in-file (and --help); everything after -- is the command and its arguments,
passed through untouched. Put dotty's flags before -- (use -- when the command
takes a --namespace of its own). The command inherits the terminal, and dotty
exits with its exit code.

With --in-file, the environment is built from a .env template instead of the
whole namespace: every {{ dotty://<namespace>/KEY }} reference is resolved from
the keychain and every plain KEY=value assignment is passed through, the way env
use fills a template — but the secrets are handed straight to the process and
never written to disk.

With neither --namespace nor --in-file, the template defaults to a .env.dotty in
the working directory (the same form as --in-file); if there is none, run
reports an error with usage.

```
dotty env run [--in-file=<file>] -- <command> [args...] [flags]
```

### Examples

```
  dotty env run --namespace=aws -- aws s3 ls
  dotty env run --namespace=ci -- ./deploy.sh
  dotty env run --in-file=.env.tmpl -- ./serve
  dotty env run -- ./serve   # builds the environment from ./.env.dotty
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


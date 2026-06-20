## dotty env use

Fill a template with credential references.

### Synopsis

Replace every {{ dotty://<namespace>/<key> }} reference (and bare {{ KEY }}
resolved against --namespace) in a template with its value, the way op inject
does. The template is read from --in-file or stdin; with neither --namespace nor
--in-file and nothing piped in, it falls back to a .env.dotty in the working
directory, and reports an error with usage when there is none. Output is written
to --out-file (created with 0600) or stdout. An unknown or malformed reference
is an error, and an --out-file is written atomically so a failed run leaves no
partial file.

```
dotty env use [--in-file=<file>] [--out-file=<file>] [flags]
```

### Examples

```
  dotty env use --in-file=.env.tmpl --out-file=.env
  echo 'token={{ dotty://ci/GITHUB_TOKEN }}' | dotty env use
  dotty env use --out-file=.env   # fills ./.env.dotty
```

### Options

```
  -h, --help              help for use
      --in-file string    template file to read (default: stdin)
      --out-file string   file to write, created with 0600 (default: stdout)
```

### Options inherited from parent commands

```
      --namespace string   credential namespace to operate on (default "default")
      --profile string     profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty env](dotty_env.md)	 - Store and inject credentials from the macOS Keychain.


## dotty completion fish

Generate the autocompletion script for fish

### Synopsis

Generate the autocompletion script for the fish shell.

To load completions in your current shell session:

	dotty completion fish | source

To load completions for every new session, execute once:

	dotty completion fish > ~/.config/fish/completions/dotty.fish

You will need to start a new shell for this setup to take effect.


```
dotty completion fish [flags]
```

### Options

```
  -h, --help[=true|false]              help for fish
      --no-descriptions[=true|false]   disable completion descriptions
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty completion](dotty_completion.md)	 - Generate the autocompletion script for the specified shell


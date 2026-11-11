## dotty completion zsh

Generate the autocompletion script for zsh

### Synopsis

Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

	echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions in your current shell session:

	source <(dotty completion zsh)

To load completions for every new session, execute once:

#### Linux:

	dotty completion zsh > "${fpath[1]}/_dotty"

#### macOS:

	dotty completion zsh > $(brew --prefix)/share/zsh/site-functions/_dotty

You will need to start a new shell for this setup to take effect.


```
dotty completion zsh [flags]
```

### Options

```
  -h, --help[=true|false]              help for zsh
      --no-descriptions[=true|false]   disable completion descriptions
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty completion](dotty_completion.md)	 - Generate the autocompletion script for the specified shell


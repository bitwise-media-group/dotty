## dotty completion bash

Generate the autocompletion script for bash

### Synopsis

Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source <(dotty completion bash)

To load completions for every new session, execute once:

#### Linux:

	dotty completion bash > /etc/bash_completion.d/dotty

#### macOS:

	dotty completion bash > $(brew --prefix)/etc/bash_completion.d/dotty

You will need to start a new shell for this setup to take effect.


```
dotty completion bash
```

### Options

```
  -h, --help[=true|false]              help for bash
      --no-descriptions[=true|false]   disable completion descriptions
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty completion](dotty_completion.md)	 - Generate the autocompletion script for the specified shell


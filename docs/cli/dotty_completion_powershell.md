## dotty completion powershell

Generate the autocompletion script for powershell

### Synopsis

Generate the autocompletion script for powershell.

To load completions in your current shell session:

	dotty completion powershell | Out-String | Invoke-Expression

To load completions for every new session, add the output of the above command
to your powershell profile.


```
dotty completion powershell [flags]
```

### Options

```
  -h, --help[=true|false]              help for powershell
      --no-descriptions[=true|false]   disable completion descriptions
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty completion](dotty_completion.md)	 - Generate the autocompletion script for the specified shell


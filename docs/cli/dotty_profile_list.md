## dotty profile list

List the profiles available on this machine.

### Synopsis

Print every profile under $XDG_CONFIG_HOME/dotty as a table — name,
description, and creation date — with * marking the active one. Unlike the
other profile verbs this one never prompts, so the output is the same piped as
it is on a terminal; ask for one profile's detail with get.

```
dotty profile list [flags]
```

### Examples

```
  dotty profile list
  dotty profile ls
```

### Options

```
  -h, --help   help for list
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty profile](dotty_profile.md)	 - Manage system profiles that travel across machines.


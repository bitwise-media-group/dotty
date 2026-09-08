## dotty profile activate

Activate an existing profile.

### Synopsis

Point the active-profile symlink at a profile. Without --name dotty
presents a fuzzy-finding picklist of existing profiles. If the named profile
does not exist, dotty offers to create it first, which runs the init
interview for it the way dotty profile new does — and ends with the new
profile active. Everything reached through the active-profile link swaps
with it, the profile's packages included: ~/.config/mise now names the new
profile's mise directory, and dotty packages sync converges the machine on
it.

```
dotty profile activate [flags]
```

### Examples

```
  dotty profile activate
  dotty profile activate --name=work
```

### Options

```
  -h, --help          help for activate
      --name string   profile to activate
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty profile](dotty_profile.md)	 - Manage system profiles that travel across machines.


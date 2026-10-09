<!--
  Copyright 2026 Bitwise Media Group Ltd
  SPDX-License-Identifier: MIT
-->

<!--
  Source of truth: internal/scaffold/template/mise.d/macos-*.toml (labels in
  internal/macos/defaults.go) — update this page when they change.
-->

# macOS defaults

dotty ships a curated set of macOS defaults groups, and
[mise](https://mise.jdx.dev/bootstrap/macos-defaults.html) applies them. Pick
them from the [`dotty init`](../cli/dotty_init.md) wizard's checklist or name
them up front with `--macos-defaults=<group,group>`. Each group is individually
selectable, so you only take the opinions you want. Your selections are stored
in the profile's answers and offered as defaults the next time you run `init`.

Each selected group renders as its own fragment in the profile,
`profiles/<name>/mise/conf.d/macos-<group>.toml`, so what a machine class
applies is plain, reviewable TOML in the repository. Deselecting a group on a
re-run removes its fragment. The fragments render on every OS; mise ignores
their macOS sections elsewhere.

## Applying, re-applying, and checking for drift

On macOS, `dotty init` installs the profile's tools and packages and then
applies the defaults:

```sh
mise bootstrap --only macos-defaults --yes
```

To re-apply them later, run the same command. Your shell's mise config is the
active profile's, so no extra setup is needed. It writes the defaults and then
runs each fragment's `post-defaults` hook, which restarts the affected app.
`mise bootstrap macos defaults apply` writes the same defaults but skips the
hooks.

To see whether the machine still matches the profile, run:

```sh
mise bootstrap macos defaults status
```

Each entry reports `set`, `differs` (with the current value), or `unset`.

Hooks run on every apply, even when nothing changed, so each `init` restarts the
Dock, Finder, and the menu bar. The keyboard, input-sources, trackpad, and mouse
groups take effect after you log out and back in.

## keyboard

Key repeat instead of the accent popup, auto-capitalisation, and a double space
inserting a period.

| Domain           | Key                                    | Value   |
| ---------------- | -------------------------------------- | ------- |
| `NSGlobalDomain` | `ApplePressAndHoldEnabled`             | `false` |
| `NSGlobalDomain` | `NSAutomaticCapitalizationEnabled`     | `true`  |
| `NSGlobalDomain` | `NSAutomaticPeriodSubstitutionEnabled` | `true`  |

Holding a key repeats it (the terminal-friendly behaviour) instead of opening
the accented-character popup.

## input-sources

The ABC keyboard layout plus the emoji & symbols viewer, with the input menu
hidden from the menu bar.

| Domain                    | Key                        | Value                                                 |
| ------------------------- | -------------------------- | ----------------------------------------------------- |
| `com.apple.HIToolbox`     | `AppleEnabledInputSources` | ABC (layout id `252`), `com.apple.CharacterPaletteIM` |
| `com.apple.TextInputMenu` | `visible`                  | `false`                                               |

## menu-bar

Always visible, except in full screen. Restarts `SystemUIServer`.

| Domain                    | Key                               | Value   |
| ------------------------- | --------------------------------- | ------- |
| `NSGlobalDomain`          | `_HIHideMenuBar`                  | `false` |
| `NSGlobalDomain`          | `AppleMenuBarVisibleInFullscreen` | `false` |
| `com.apple.controlcenter` | `AutoHideMenuBarOption`           | `2`     |

## trackpad

Tap to click, light click, three-finger drag, four-finger swipes between spaces
and full-screen apps, and maximum tracking speed.

| Domain                              | Key                                                      | Value  |
| ----------------------------------- | -------------------------------------------------------- | ------ |
| built-in and Bluetooth trackpads    | `Clicking`, `TrackpadThreeFingerDrag`                    | `true` |
| built-in and Bluetooth trackpads    | `TrackpadThreeFinger{Horiz,Vert}SwipeGesture`            | `0`    |
| built-in and Bluetooth trackpads    | `TrackpadThreeFingerTapGesture`                          | `0`    |
| built-in and Bluetooth trackpads    | `TrackpadFourFinger{Horiz,Vert}SwipeGesture`             | `2`    |
| `com.apple.AppleMultitouchTrackpad` | `FirstClickThreshold`, `SecondClickThreshold`            | `0`    |
| `NSGlobalDomain`                    | `com.apple.trackpad.scaling`                             | `3.0`  |
| `NSGlobalDomain`                    | `com.apple.trackpad.forceClick`                          | `true` |
| `NSGlobalDomain`                    | `NSWindowShouldDragOnGesture`                            | `true` |
| `NSGlobalDomain` (current host)     | `com.apple.trackpad.threeFingerDragGesture`              | `true` |
| `NSGlobalDomain` (current host)     | `com.apple.trackpad.threeFinger{Horiz,Vert}SwipeGesture` | `0`    |
| `NSGlobalDomain` (current host)     | `com.apple.trackpad.fourFinger{Horiz,Vert}SwipeGesture`  | `2`    |
| `NSGlobalDomain` (current host)     | `com.apple.mouse.tapBehavior`                            | `1`    |

The built-in trackpad is `com.apple.AppleMultitouchTrackpad`; Bluetooth
trackpads are `com.apple.driver.AppleBluetoothMultitouch.trackpad`. System
Settings reads the gestures from the current-host global domain, so the fragment
sets both.

## mouse

Magic Mouse: primary click only, two-finger swipe between full-screen apps,
two-finger double-tap for Mission Control, and no smart zoom. Set in both
`com.apple.AppleMultitouchMouse` and
`com.apple.driver.AppleBluetoothMultitouch.mouse`.

| Key                                                                   | Value         |
| --------------------------------------------------------------------- | ------------- |
| `MouseButtonMode`                                                     | `"OneButton"` |
| `MouseButtonDivision`                                                 | `55`          |
| `MouseHorizontalScroll`, `MouseVerticalScroll`, `MouseMomentumScroll` | `true`        |
| `MouseOneFingerDoubleTapGesture`                                      | `0`           |
| `MouseTwoFingerDoubleTapGesture`                                      | `3`           |
| `MouseTwoFingerHorizSwipeGesture`                                     | `2`           |

## finder

Clean desktop, path/status bars, no extension-change warnings. Restarts
`Finder`.

| Domain                      | Key                               | Value   |
| --------------------------- | --------------------------------- | ------- |
| `com.apple.TimeMachine`     | `DoNotOfferNewDisksForBackup`     | `true`  |
| `com.apple.finder`          | `ShowExternalHardDrivesOnDesktop` | `false` |
| `com.apple.finder`          | `ShowHardDrivesOnDesktop`         | `false` |
| `com.apple.finder`          | `ShowRemovableMediaOnDesktop`     | `false` |
| `com.apple.finder`          | `ShowMountedServersOnDesktop`     | `false` |
| `com.apple.finder`          | `CreateDesktop`                   | `false` |
| `com.apple.desktopservices` | `DSDontWriteNetworkStores`        | `true`  |
| `com.apple.finder`          | `ShowPathbar`                     | `true`  |
| `com.apple.finder`          | `ShowStatusBar`                   | `true`  |
| `com.apple.finder`          | `FXEnableExtensionChangeWarning`  | `false` |

`CreateDesktop false` hides desktop icons entirely; files in `~/Desktop` are
untouched. `DSDontWriteNetworkStores` stops Finder littering network shares with
`.DS_Store` files.

## screenshots

| Domain                    | Key    | Value |
| ------------------------- | ------ | ----- |
| `com.apple.screencapture` | `type` | `png` |

## software-update

Check for updates daily.

| Domain                     | Key                 | Value |
| -------------------------- | ------------------- | ----- |
| `com.apple.SoftwareUpdate` | `ScheduleFrequency` | `1`   |

## spaces

Per-display Spaces that never rearrange themselves. Restarts `Dock`, since
`mru-spaces` lives in the Dock's domain.

| Domain             | Key              | Value   |
| ------------------ | ---------------- | ------- |
| `com.apple.spaces` | `spans-displays` | `false` |
| `com.apple.dock`   | `mru-spaces`     | `false` |

## dock

Sizing, magnification, minimize-to-application, and the pinned apps. Restarts
`Dock`.

| Domain           | Key                       | Value   |
| ---------------- | ------------------------- | ------- |
| `com.apple.dock` | `autohide`                | `false` |
| `com.apple.dock` | `magnification`           | `true`  |
| `com.apple.dock` | `tilesize`                | `43.0`  |
| `com.apple.dock` | `largesize`               | `77.0`  |
| `com.apple.dock` | `minimize-to-application` | `true`  |

The Dock pins, in order: Apps, Siri, Safari, Messages, Mail, Photos, FaceTime,
Phone, Calendar, Reminders, Notes, Music, then the optional apps, and System
Settings last. The optional apps come from the wizard's "Which apps should the
Dock pin?" question, or `--dock-apps`:

| `--dock-apps` value | App                         | Offered when                        |
| ------------------- | --------------------------- | ----------------------------------- |
| `ghostty`           | `/Applications/Ghostty.app` | always                              |
| `claude-code`       | `/Applications/Claude.app`  | the `claude-code` agent is selected |
| `codex`             | `/Applications/ChatGPT.app` | the `codex` agent is selected       |

Without an answer, every offered app is pinned. mise refuses to pin an app that
isn't installed, which is why init installs packages before applying the
defaults; the agents' fragments install the Claude and ChatGPT casks.

The Downloads stack is left alone. It's the macOS default, and managing it would
commit a per-user path that the Dock then decorates with its own IDs.

## animations

| Domain           | Key                                  | Value   |
| ---------------- | ------------------------------------ | ------- |
| `NSGlobalDomain` | `NSAutomaticWindowAnimationsEnabled` | `false` |

## gpg-keychain

Let GPG tools cache PINs in the macOS keychain (pairs with the pinentry-mac
bridge described in
[Signing keys & first commit](../getting-started/signing.md)).

| Domain                | Key               | Value   |
| --------------------- | ----------------- | ------- |
| `org.gpgtools.common` | `UseKeychain`     | `true`  |
| `org.gpgtools.common` | `DisableKeychain` | `false` |

## default-terminal

Ghostty opens shell scripts and executables. macOS keeps these handlers in
Launch Services, not in a defaults key mise could set entry by entry, so the
fragment installs [duti](https://github.com/moretension/duti) as a bootstrap
package and its `post-defaults` hook runs:

```sh
duti -s com.mitchellh.ghostty public.unix-executable shell
duti -s com.mitchellh.ghostty com.apple.terminal.shell-script all   # .command, .tool
duti -s com.mitchellh.ghostty public.shell-script all               # .sh
duti -s com.mitchellh.ghostty public.zsh-script all                 # .zsh
```

Ghostty declares no URL schemes, so `ssh://` and `x-man-page://` links stay with
Terminal. Check the result with `duti -x sh`.

## Smartcard enforcement (PIV)

Separate from the groups above, `dotty init --piv` (or the matching wizard
question) enforces smart-card login **system-wide**. dotty applies this one
itself, last, because it writes `/Library/Preferences` through `sudo`, which
mise's user defaults can't. It prompts for your password on the terminal:

```sh
sudo defaults write /Library/Preferences/com.apple.security.smartcard enforceSmartCard -bool true
sudo defaults write /Library/Preferences/com.apple.security.smartcard allowUnmappedUsers -int 1
```

!!! warning "Lockout risk"

    Once `enforceSmartCard` is set, macOS refuses password-only logins. Make
    sure a smart card (e.g. your YubiKey's PIV applet) is enrolled and working
    **before** enabling this, and keep a recovery path — a second enrolled
    key, or a way to boot into recovery and delete the preference:

    ```sh
    sudo defaults delete /Library/Preferences/com.apple.security.smartcard enforceSmartCard
    ```

## Undoing a group

Deselect the group on a re-run of `dotty init` so the profile stops applying it.
That removes the fragment but doesn't change the machine: every setting is a
plain user default (except PIV), so delete the keys you want back and restart
the affected app:

```sh
defaults delete com.apple.dock tilesize
killall Dock
```

Deleting a key restores the macOS built-in default, not any value you had set
before dotty ran. To hand shell scripts back to Terminal, run
`duti -s com.apple.Terminal public.shell-script all` (and likewise for the other
UTIs above).

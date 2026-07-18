<!--
  Copyright 2026 Bitwise Media Group Ltd
  SPDX-License-Identifier: MIT
-->

<!-- Source of truth: internal/macos/defaults.go — update this page when it changes. -->

# macOS defaults

dotty ships a curated set of macOS `defaults` groups. They are applied as the
**last** step of [`dotty init`](../cli/dotty_init.md) — either picked
interactively from the wizard's checklist or named up front with
`--macos-defaults=<group,group>` — and each group is individually selectable, so
you only take the opinions you want. Your selections are stored in the profile's
answers and offered as defaults the next time you run `init`.

There is currently no standalone re-apply command: to apply a group after the
fact, re-run `dotty init` (stored answers make this quick) or run the
`defaults write` commands below by hand.

Groups that change Finder, the Dock, or the menu bar restart the affected
service (`killall Finder`, and so on) once after all writes; everything else
takes effect immediately or at next login.

## keyboard

Key repeat instead of the accent popup.

| Domain           | Key                        | Value   |
| ---------------- | -------------------------- | ------- |
| `NSGlobalDomain` | `ApplePressAndHoldEnabled` | `false` |

Holding a key repeats it (the terminal-friendly behaviour) instead of opening
the accented-character popup.

## menu-bar

Always visible, except in full screen. Restarts `SystemUIServer`.

| Domain                    | Key                               | Value   |
| ------------------------- | --------------------------------- | ------- |
| `NSGlobalDomain`          | `_HIHideMenuBar`                  | `false` |
| `NSGlobalDomain`          | `AppleMenuBarVisibleInFullscreen` | `false` |
| `com.apple.controlcenter` | `AutoHideMenuBarOption`           | `2`     |

## trackpad

Three-finger drag, and drag windows from anywhere with a gesture.

| Domain                              | Key                           | Value  |
| ----------------------------------- | ----------------------------- | ------ |
| `com.apple.AppleMultitouchTrackpad` | `TrackpadThreeFingerDrag`     | `true` |
| `NSGlobalDomain`                    | `NSWindowShouldDragOnGesture` | `true` |

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

Per-display Spaces that never rearrange themselves. Restarts `Dock`.

| Domain             | Key              | Value   |
| ------------------ | ---------------- | ------- |
| `com.apple.spaces` | `spans-displays` | `false` |
| `com.apple.dock`   | `mru-spaces`     | `false` |

## dock

Sizing and minimize-to-application. Restarts `Dock`.

| Domain           | Key                       | Value   |
| ---------------- | ------------------------- | ------- |
| `com.apple.dock` | `autohide`                | `false` |
| `com.apple.dock` | `largesize`               | `96`    |
| `com.apple.dock` | `minimize-to-application` | `true`  |
| `com.apple.dock` | `tilesize`                | `48`    |

## animations

| Domain           | Key                                  | Value   |
| ---------------- | ------------------------------------ | ------- |
| `NSGlobalDomain` | `NSAutomaticWindowAnimationsEnabled` | `false` |

## gpg-keychain

Let GPG tools cache PINs in the macOS keychain (pairs with the pinentry-mac
bridge described in
[Signing keys & first commit](../getting-started/signing.md)).

| Domain                | Key               | Value |
| --------------------- | ----------------- | ----- |
| `org.gpgtools.common` | `UseKeychain`     | `yes` |
| `org.gpgtools.common` | `DisableKeychain` | `no`  |

## Smartcard enforcement (PIV)

Separate from the groups above, `dotty init --piv` (or the matching wizard
question) enforces smart-card login **system-wide**. This one runs through
`sudo` and prompts for your password on the terminal:

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

## Wallpaper

When the wizard offers a wallpaper, it lists images from
`~/.local/share/wallpapers` (`.png`, `.jpg`, `.jpeg`, `.heic`, `.tiff`). dotty
distributes no wallpapers — populate that directory from your own private repo.
The selected image is applied to **every** desktop via `osascript` (System
Events).

## Undoing a group

Every write above is a plain user default (except PIV). To revert one, delete
the key and restart the affected service:

```sh
defaults delete com.apple.dock tilesize
killall Dock
```

Deleting a key restores the macOS built-in default, not any value you had set
before dotty ran.

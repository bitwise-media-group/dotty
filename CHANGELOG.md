# Changelog

## [0.3.3](https://github.com/bitwise-media-group/dotty/compare/v0.3.2...v0.3.3) (2026-08-22)


### Bug Fixes

* **deps:** update bitwise-media-group/github-workflows action to v6.1.1 ([#122](https://github.com/bitwise-media-group/dotty/issues/122)) ([a7c5123](https://github.com/bitwise-media-group/dotty/commit/a7c5123e7caceeb4562ea562442a5bf7051ebc11))
* **deps:** update bitwise-media-group/github-workflows action to v6.2.0 ([#128](https://github.com/bitwise-media-group/dotty/issues/128)) ([880ad91](https://github.com/bitwise-media-group/dotty/commit/880ad91eaee881e772a4e5c0978265249d4de27f))
* **deps:** update module charm.land/bubbletea/v2 to v2.0.9 ([#134](https://github.com/bitwise-media-group/dotty/issues/134)) ([dc04a38](https://github.com/bitwise-media-group/dotty/commit/dc04a38362edf02eac8e96cece88c3a672a7f8b4))
* **deps:** update module charm.land/lipgloss/v2 to v2.0.6 ([#126](https://github.com/bitwise-media-group/dotty/issues/126)) ([ab9983c](https://github.com/bitwise-media-group/dotty/commit/ab9983cd2cbfcaf229e9f59f013962acb65a569f))

## [0.3.2](https://github.com/bitwise-media-group/dotty/compare/v0.3.1...v0.3.2) (2026-08-14)


### Bug Fixes

* **init:** merge into an existing profile Brewfile instead of overwriting ([ce0a229](https://github.com/bitwise-media-group/dotty/commit/ce0a229f0c0b0ae447d3c4f56d748a66e1fa9c9e)), closes [#109](https://github.com/bitwise-media-group/dotty/issues/109)
* **linker:** remove entry symlinks before mirroring copy-deployed files ([a4a4f01](https://github.com/bitwise-media-group/dotty/commit/a4a4f015be348cebeae8bd6d02329c2129695c7f))

## [0.3.1](https://github.com/bitwise-media-group/dotty/compare/v0.3.0...v0.3.1) (2026-08-12)


### Features

* **brewfile:** add remove command with trust revocation ([7fcd9b3](https://github.com/bitwise-media-group/dotty/commit/7fcd9b37d7a75e4d8a7ab80a11158b16fa46e8d5))
* **codex:** render config as a profile layer so codex owns config.toml ([b717722](https://github.com/bitwise-media-group/dotty/commit/b7177229726fa1e2fef99b99d817af6d7e9cd182))
* **linker:** deploy grok hooks as real file copies, not symlinks ([0998766](https://github.com/bitwise-media-group/dotty/commit/099876625999ff0dc32da315eda8cedcebc2e8f0))
* **private:** encrypted per-profile private dotfiles (dotty private) ([056797d](https://github.com/bitwise-media-group/dotty/commit/056797dad8396009c48a5aed60cb75e658582866))
* **scaffold:** allow ~/.local/state in agent sandbox configs ([a59b2dc](https://github.com/bitwise-media-group/dotty/commit/a59b2dc0ab1278bdc1909e7e3d22d768c5713538))


### Bug Fixes

* **brewfile:** run brew bundle install with the terminal attached ([443cf40](https://github.com/bitwise-media-group/dotty/commit/443cf40df8021c67fec84c76f84b908315d0adab))

## [0.3.0](https://github.com/bitwise-media-group/dotty/compare/v0.2.6...v0.3.0) (2026-07-28)


### ⚠ BREAKING CHANGES

* **profile:** `dotty profile new` no longer creates a bare profile directory; it runs the full init flow and always activates the result, so --activate is deprecated and does nothing.

### Features

* **git:** drop landed layers before proposing, add propose --draft ([3341663](https://github.com/bitwise-media-group/dotty/commit/3341663df8f2ffaecd549d3920aa28f7be850de3))
* **git:** own the pull request of every branch in a stack ([fa9db18](https://github.com/bitwise-media-group/dotty/commit/fa9db18403fcfc450aed862bf8ad949d05f91862))
* **profile:** add list, get, and delete verbs ([b28ce06](https://github.com/bitwise-media-group/dotty/commit/b28ce0696b4d40e398188ea51d67876948fe8a61)), closes [#58](https://github.com/bitwise-media-group/dotty/issues/58)
* **profile:** make `profile new` a proxy for `dotty init` ([940359f](https://github.com/bitwise-media-group/dotty/commit/940359fc2436c047eaba16d8a2e5e5c14b1f09c6))
* **scaffold:** add keybinds to launch coding agents unsandboxed ([5cf83c4](https://github.com/bitwise-media-group/dotty/commit/5cf83c4e1a4596725e0b3e6745ab229cc4d9a54f))
* **scaffold:** add Swift language support to the Neovim template ([68a2fd5](https://github.com/bitwise-media-group/dotty/commit/68a2fd5e1dd4215d5d5665148b0d56249d03bd50))
* **scaffold:** allow Swift and mise tooling in agent sandboxes ([d9e2099](https://github.com/bitwise-media-group/dotty/commit/d9e20994ccbd4bd0051a7114c8f47236db3f8c74))

## [0.2.6](https://github.com/bitwise-media-group/dotty/compare/v0.2.5...v0.2.6) (2026-07-22)


### Bug Fixes

* **brewfile:** tap missing third-party taps before brew bundle add ([ca843e3](https://github.com/bitwise-media-group/dotty/commit/ca843e3d7fa69932dc363edf12ad1970233e12cc))

## [0.2.5](https://github.com/bitwise-media-group/dotty/compare/v0.2.4...v0.2.5) (2026-07-21)


### Features

* **git:** push new stack branches to origin on start/append ([f2b841b](https://github.com/bitwise-media-group/dotty/commit/f2b841bd61d22d4fe910aba8ad7ce3da2479d6cd))


### Bug Fixes

* **scaffold:** disable EQUALS expansion in the zsh template ([e924bbd](https://github.com/bitwise-media-group/dotty/commit/e924bbd00bb558f908a7efbc250f39949e5d15c7))

## [0.2.4](https://github.com/bitwise-media-group/dotty/compare/v0.2.3...v0.2.4) (2026-07-20)


### Features

* **git:** add --auto-merge flag to propose ([b906b14](https://github.com/bitwise-media-group/dotty/commit/b906b1412949fb0ece3a196228b7c929fbef84a4))
* **git:** add --browse and --copy flags to propose ([4172f60](https://github.com/bitwise-media-group/dotty/commit/4172f6071195acdedcc47348147853c0324a84c9))
* **git:** read flag defaults from git configuration ([c9a64e5](https://github.com/bitwise-media-group/dotty/commit/c9a64e5cf531dd09ede3ccc3c39299a47657ed77))
* **linker:** retire legacy files that shadow the rendered config ([4bf4b6a](https://github.com/bitwise-media-group/dotty/commit/4bf4b6afbae4c75f9a39b978ab35fbe651e89db0))
* **scaffold:** trust the bitwise-media-group tap in core.Brewfile ([669c341](https://github.com/bitwise-media-group/dotty/commit/669c341eec458e58c99f6f2fe9ed254c26b851ed))


### Bug Fixes

* **signing-key:** answer ssh yes/no prompts with CONFIRM, not GETPIN ([4a86b41](https://github.com/bitwise-media-group/dotty/commit/4a86b41634201b14c24b4e13a8e71033d810caef))

## [0.2.3](https://github.com/bitwise-media-group/dotty/compare/v0.2.2...v0.2.3) (2026-07-19)


### Features

* **brewfile:** dedupe adds and record trusted: true on new entries ([a31c87a](https://github.com/bitwise-media-group/dotty/commit/a31c87a05405950ee9e0b61931c4a9dfb2c7e501))
* **git:** push trunk from done and track origin from start ([c4d8f16](https://github.com/bitwise-media-group/dotty/commit/c4d8f16d4601ac285f34b057a67a8e4a62284339))
* **init:** finish the interview before acting, and brew dotty itself ([48ccadd](https://github.com/bitwise-media-group/dotty/commit/48ccadd87b05018f84effaedb130d2060d43869b))

## [0.2.2](https://github.com/bitwise-media-group/dotty/compare/v0.2.1...v0.2.2) (2026-07-18)


### Bug Fixes

* **release:** ship static shell completions in the Homebrew cask ([16cb6e9](https://github.com/bitwise-media-group/dotty/commit/16cb6e9a73d90a96555cccac9e61f3a89b5cc1fd))

## [0.2.1](https://github.com/bitwise-media-group/dotty/compare/v0.2.0...v0.2.1) (2026-07-18)


### Bug Fixes

* **build:** ignore bin so goreleaer does not detect a dirty branch ([2a2e57e](https://github.com/bitwise-media-group/dotty/commit/2a2e57e9ce2ae010a4a132314918cf5022c43391))

## [0.2.0](https://github.com/bitwise-media-group/dotty/compare/v0.1.1...v0.2.0) (2026-07-18)


### ⚠ BREAKING CHANGES

* **git:** dotty git stack now opens the switch picker instead of printing stack status; use dotty git status for the status view.

### Features

* **dotfiles:** add the stow-style linker and dotfiles commands ([073d514](https://github.com/bitwise-media-group/dotty/commit/073d5141a522326b022ed41d330425f99d433c2d))
* **git:** add the signature-preserving stacked branch workflow ([d9447a6](https://github.com/bitwise-media-group/dotty/commit/d9447a63a6ecec85dcfbedbd40fcb5a233032771))
* **git:** rename dotty git stack to dotty git status ([b1b8cc0](https://github.com/bitwise-media-group/dotty/commit/b1b8cc02486d40b6d78504427a7f07f85d54012b))
* **init:** add the dotty init wizard ([377f2c0](https://github.com/bitwise-media-group/dotty/commit/377f2c05b4fd25444e0690e846d1e0580f96cf1f))
* **scaffold:** embed the dotfiles template with repo-shared profiles ([27f6a01](https://github.com/bitwise-media-group/dotty/commit/27f6a0138d4673ff21a68fc14f676ab6c67126e9))
* **security-key:** add a per-profile serial allowlist ([44ca895](https://github.com/bitwise-media-group/dotty/commit/44ca8955ed85a08117378f21b83f76e95847c4b4))
* **signing-key:** route the dotty-ssh-askpass shim to ask-pass ([f9cb2af](https://github.com/bitwise-media-group/dotty/commit/f9cb2afa8e710429e8871f00fcb185111abbeb81))
* **signing-key:** trust enrolled keys in git allowed_signers ([a8eb4b0](https://github.com/bitwise-media-group/dotty/commit/a8eb4b0e4c0471e8698e20f0b0b1765e29e0a508))
* **tmux:** add hidden set-status command for agent lifecycle hooks ([139e70b](https://github.com/bitwise-media-group/dotty/commit/139e70ba38ca7ec16761a6596b448a894537f1ac))
* **tmux:** add tmux new session launcher ([6041bb0](https://github.com/bitwise-media-group/dotty/commit/6041bb04ad13260f071cbdd8d906e135d79dd8a0))
* **tmux:** launch grok in its own agent window ([25ca105](https://github.com/bitwise-media-group/dotty/commit/25ca10561a98ccf807287e4cf967b9e67b6615c1))
* **tui:** add fzf-style fuzzy picker and use it for tmux new ([471f9f1](https://github.com/bitwise-media-group/dotty/commit/471f9f1e13816d8824fd451581fc8baf608fe7d1))
* **tui:** grow the prompt helpers for interview flows ([054eb0c](https://github.com/bitwise-media-group/dotty/commit/054eb0cf14ddb58aa977f7b6c0fe446a0d4a8944))
* **tui:** theme huh/bubbletea prompts with design-system dotty colours ([f6fa25a](https://github.com/bitwise-media-group/dotty/commit/f6fa25a798f2cf30d9f4f6d36687af0841cb9c56)), closes [#13](https://github.com/bitwise-media-group/dotty/issues/13)
* **worktree:** add agent worktree lifecycle commands ([24157bd](https://github.com/bitwise-media-group/dotty/commit/24157bd489aa6aacdb8cce522401a65632ef4cfe))


### Bug Fixes

* **brewfile:** drop obsolete --no-cleanup from brew bundle upgrade ([8376362](https://github.com/bitwise-media-group/dotty/commit/8376362816f8e55ca116a177097d63ebb6c7dbe9)), closes [#27](https://github.com/bitwise-media-group/dotty/issues/27)
* **git:** drop redundant branch names from the PR stack map ([d1dab30](https://github.com/bitwise-media-group/dotty/commit/d1dab30afe40ae55c955affdc8936d27e8ce20f1))
* **git:** populate empty sign-off emails during resign --reset-author ([6004b21](https://github.com/bitwise-media-group/dotty/commit/6004b213314dcdfce0d63b57837ce1f530821485))
* **git:** push only rewritten branches, skip current PR bodies, restore HEAD ([b775164](https://github.com/bitwise-media-group/dotty/commit/b775164b8b9f2a8a509d29124c007d148704f608))
* **release:** finish the make-library migration for SBOM generation ([8ff60c9](https://github.com/bitwise-media-group/dotty/commit/8ff60c94bd2c135bd7a09cefdf772e5c693f6ea0))
* **signing-key:** cache ssh client-auth PINs in the macOS keychain ([c9cf9f5](https://github.com/bitwise-media-group/dotty/commit/c9cf9f5e86c385964e79c8da10e1d7f0dca03c8a))


### Performance Improvements

* **tui:** cap picklist height so long lists render a viewport ([f886dd3](https://github.com/bitwise-media-group/dotty/commit/f886dd3db72192980badc0b3fcdacf53fd55b3cb))

## [0.1.1](https://github.com/bitwise-media-group/dotty/compare/v0.1.0...v0.1.1) (2026-07-01)


### Features

* **signing-key:** add import command for existing key stubs ([2f2ed30](https://github.com/bitwise-media-group/dotty/commit/2f2ed30cca45a00790340164629887e849e4228f))
* **signing-key:** add link verb to select the plugged-in key for ssh ([246421c](https://github.com/bitwise-media-group/dotty/commit/246421cc0ca637e52ae61a1a0454ea63149dff2c))
* **signing-key:** authorize a key on a remote host's authorized_keys ([d917296](https://github.com/bitwise-media-group/dotty/commit/d917296dc2dac0720e7933ec1e833fca560af475))
* **signing-key:** cache the YubiKey PIN in the macOS keychain ([248d1f6](https://github.com/bitwise-media-group/dotty/commit/248d1f6f820f613806c3fdce1b59a9ab7e6e6ce0))


### Bug Fixes

* **docs:** pin man-page .TH date for reproducible generation ([56586a8](https://github.com/bitwise-media-group/dotty/commit/56586a83387f51274a6d8d47ed51fcb6dec22c2c))

## 0.1.0 (2026-06-29)


### ⚠ BREAKING CHANGES

* **env:** a bare `dotty env run -- <command>` no longer exports the whole default namespace; it now requires a .env.dotty (or an explicit --namespace). Pass --namespace=default to restore the previous behavior.

### Features

* **brewfile:** add brew bundle engine with tap-trust flow ([2f5b59b](https://github.com/bitwise-media-group/dotty/commit/2f5b59b37ab300325354322a04eff71832745034))
* **env:** add --in-file to env run for in-memory secret injection ([9bdd53e](https://github.com/bitwise-media-group/dotty/commit/9bdd53e0def08e69c8782dbc5a97f133488c8b35))
* **env:** add keychain-backed credential command ([84159ab](https://github.com/bitwise-media-group/dotty/commit/84159abcc8d876836d7715fec047c556beda9bc8))
* **env:** capture .env files into keychain references ([9dc5e6e](https://github.com/bitwise-media-group/dotty/commit/9dc5e6e82ba42a19cec2e27dd39fb6849fdda8ff))
* **env:** default run and use to .env.dotty in the working directory ([ba9ce96](https://github.com/bitwise-media-group/dotty/commit/ba9ce9687f6cb5c4cfc8603bb663c1c9cd52781d))
* **git:** add `dotty git resign` to rebase and re-sign commits ([0e74e0c](https://github.com/bitwise-media-group/dotty/commit/0e74e0c19d3c32b1a6c0fc46877bd3b65b5f72a7))
* **profile:** add profiles, activation symlink, and brewfile commands ([7348cb6](https://github.com/bitwise-media-group/dotty/commit/7348cb6c0c112aa346feff1b43dbf255522c825d))
* **security-key:** add YubiKey serial aliases with tree multi-select removal ([6e17f27](https://github.com/bitwise-media-group/dotty/commit/6e17f27491424402897d83312b1ea8a39aabd437))
* **signing-key:** add resident SSH signing keys and git commit signing ([28635df](https://github.com/bitwise-media-group/dotty/commit/28635df493509d58c52db6d8f468a0426d9758a0))


### Bug Fixes

* **env:** move serviceName into the darwin keychain backend ([e640fa9](https://github.com/bitwise-media-group/dotty/commit/e640fa9e52ab78153aa21094e417d61f0e121e6a))

# Changelog

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

## [1.1.0](https://github.com/xraph/dtl/compare/v1.0.1...v1.1.0) (2026-08-02)

### Features

* **cmd/dtl-lsp:** a Language Server Protocol server ([c7901f9](https://github.com/xraph/dtl/commit/c7901f93088282bf1933a3b457c81e427aa9783a))
* **lang:** editor intelligence as pure functions ([0cf6695](https://github.com/xraph/dtl/commit/0cf6695c0a46955922e2c7257fe61c703d008527))
* **lang:** let the host supply a dataset's reference convention ([3dad8d9](https://github.com/xraph/dtl/commit/3dad8d91474c1d731d92ae8fa5c3db2c46a22bb8))

### Bug Fixes

* **ci:** depend on the published adapter, annotate a scanner false positive ([0411342](https://github.com/xraph/dtl/commit/04113423327c3b36c7383b4686d54937115dd1c4))
* tighten workflow_call secret grants ([71613b3](https://github.com/xraph/dtl/commit/71613b31f88a321fd2b611a3fab65cf7d0d1e7af))

### Documentation

* add reachable-baseline-tag step before migrating vessel and go-utils ([ac04d43](https://github.com/xraph/dtl/commit/ac04d43e759eb2938146a4983819d4a667143fb2))
* design for generic xraph/workflows repo and Go track completion ([d24ea4e](https://github.com/xraph/dtl/commit/d24ea4ecf200c8e8c3bd90ffd5bfd83364a7cf85))
* design for the Rust CI track ([329036c](https://github.com/xraph/dtl/commit/329036c093e76e7647845f312f100564e811a18f))
* implementation plan for generic xraph/workflows phase 0+1 ([aa9a3d2](https://github.com/xraph/dtl/commit/aa9a3d2cd9b10882f0f0cd1f89df4b02578c7e9e))
* implementation plan for the Rust CI track ([05b94a3](https://github.com/xraph/dtl/commit/05b94a340dd7b0e522f987bc15edfa13d5171f79))
* re-sync plan with shipped go-ci.yml and record as-built deviations ([2f3c942](https://github.com/xraph/dtl/commit/2f3c942b9e74eee1dd9f60a7c9c773c78c3562dc))
* record as-built deviations for the generic workflows phase ([2256a70](https://github.com/xraph/dtl/commit/2256a703d8784786b43dc0bfcab9746f0c0f8f19))
* record as-built deviations for the Rust track ([4ad2a31](https://github.com/xraph/dtl/commit/4ad2a31dac66203e18176fd8fd29c8163a7f34fa))
* record deferred items closed after the Rust track shipped ([9adfecc](https://github.com/xraph/dtl/commit/9adfecc174ca54f298ea820fb2ee29d60173d1bc))
* require security-events write on go-ci callers for SARIF upload ([eae80c4](https://github.com/xraph/dtl/commit/eae80c47652142504c6ec56027faf1ae47722cd1))

# Changelog

All notable changes to this project are documented in this file.

## [v1.0.1] - 2026-07-31

### Changes since v1.0.0

- 0355926 (HEAD -> main, origin/main) ci: add release and CodeQL workflows (#2)
- 0132044 ci: adopt shared go-workflows (#1)

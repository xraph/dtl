## [1.5.0](https://github.com/xraph/dtl/compare/v1.4.0...v1.5.0) (2026-08-03)

### ⚠ BREAKING CHANGES

* **stdlib:** len() on a string now returns the number of characters
rather than the number of bytes. Results differ for any string
containing non-ASCII characters: len("café") is now 4, previously 5.
Callers that genuinely wanted a byte count — sizing a buffer or a
network payload — should compute it host-side rather than through len.

### Migration: `len` on strings

**Who is affected.** Only code that calls `len()` on a string that can contain
non-ASCII characters. `len` on arrays and objects is unchanged, and any string
that is pure ASCII returns exactly what it did before — one byte per character.

```
len("hello")   -- 5 before, 5 now      (unchanged)
len("café")    -- 5 before, 4 now      (é is two bytes, one character)
len("日本語")   -- 9 before, 3 now
len([1, 2, 3]) -- 3 before, 3 now      (unchanged)
```

**Why it changed.** `len` was the only string function in the library counting
bytes. `substr`, `left`, `right`, `pad_left`, `pad_right`, `truncate` and
`reverse_text` have always counted characters, so `len` disagreed with every
function it would naturally be paired with:

```
substr("café", 0, len("café"))   -- asked for 5 characters of a 4-character string
pad_left("café", len("café"))    -- padded to 5, producing a leading space
```

Those expressions were silently wrong on non-ASCII input and correct on ASCII,
which is why the disagreement survived so long.

**What to do.**

*If you slice, pad, or bound strings with `len`* — no action. Those call sites
were latently wrong on non-ASCII input and are now correct. This release fixes
them rather than breaking them.

*If you genuinely needed a byte count* — sizing a buffer, enforcing a network
payload limit, or checking a database column width measured in bytes — `len` no
longer answers that question. Compute it host-side and register it under your
own namespace:

```go
reg.RegisterBuiltin("bytes::len", &executor.BuiltinFunc{
	Name: "bytes::len", MinArgs: 1, MaxArgs: 1,
	Doc: "bytes::len(s) -> int -- Length of s in bytes",
	Fn: func(args []any) (any, error) {
		return int64(len(executor.ToString(args[0]))), nil
	},
})
```

**Finding affected call sites.** Search your DTL sources for `len(` applied to a
string. The ones that matter are those whose result is compared against a limit
expressed in bytes, or passed to something outside DTL that counts bytes —
a column width, a protocol field, an allocation size. Uses that stay inside DTL
(slicing, padding, validation against a human-facing character count) need no
change.

### Bug Fixes

* **stdlib:** len counts characters, not bytes ([bacf375](https://github.com/xraph/dtl/commit/bacf375c6cfd2e421e8d5995fd9d39014377c97e))

## [1.4.0](https://github.com/xraph/dtl/compare/v1.3.0...v1.4.0) (2026-08-03)

### Features

* **parser:** bare lambdas and the comparison shorthand ([cd82d41](https://github.com/xraph/dtl/commit/cd82d41c249bd18829ac474e878a23c7f04828aa))
* **stdlib:** add case conversion, trimming, indexing, and masking ([d4f396c](https://github.com/xraph/dtl/commit/d4f396c179a6968fc4d327918778b6688d5998f2))
* **stdlib:** add json::, encoding::, and hash:: ([b723321](https://github.com/xraph/dtl/commit/b723321e80a8c9517f0e33b9944314b9aed3bfbf))
* **stdlib:** add null handling, dotted-path access, and object reshaping ([d550707](https://github.com/xraph/dtl/commit/d550707d680e445c4d439d83d41c9952c8209775))
* **stdlib:** add regex:: over a bounded compiled-pattern cache ([a299aaf](https://github.com/xraph/dtl/commit/a299aafa904967d2f3ca4df5d916cf0741dfbf79))
* **stdlib:** document every builtin at its registration site ([dc4380d](https://github.com/xraph/dtl/commit/dc4380d3deddeaec0a2ea90a60d7e496c0038515))
* **stdlib:** fill out collections, math, datetime, and stats ([f92e230](https://github.com/xraph/dtl/commit/f92e2306893614014425e1f8704d3eb8959adc0d))

### Bug Fixes

* **stdlib:** reject unknown datetime units instead of guessing ([93aa244](https://github.com/xraph/dtl/commit/93aa2444221cf4f832acf0e5f4c59ed6e0d84eeb))

### Documentation

* describe the standard library as it actually ships ([98c3473](https://github.com/xraph/dtl/commit/98c3473043a75e7680a9aba3b6bf7792ba0dd287))
* design for standard library expansion ([837aba0](https://github.com/xraph/dtl/commit/837aba067f216c633338d44648b7ae360b7a5aa7))

## [1.3.0](https://github.com/xraph/dtl/compare/v1.2.1...v1.3.0) (2026-08-02)

### Features

* **syntaxes:** register cleanly with Shiki ([b4067db](https://github.com/xraph/dtl/commit/b4067db13f4dccffbb719b736b9ca3689a02a6ac))

## [1.2.1](https://github.com/xraph/dtl/compare/v1.2.0...v1.2.1) (2026-08-02)

### Documentation

* DTL is Data Transformation Language ([666e5c2](https://github.com/xraph/dtl/commit/666e5c242f1876e0c61ef5ab36688b4a83c13d83))

## [1.2.0](https://github.com/xraph/dtl/compare/v1.1.1...v1.2.0) (2026-08-02)

### Features

* **syntaxes:** ship the TextMate grammar with the language ([c07b8a1](https://github.com/xraph/dtl/commit/c07b8a1493cf0afe91989b0e64f9392b7a807f2d))

## [1.1.1](https://github.com/xraph/dtl/compare/v1.1.0...v1.1.1) (2026-08-02)

### Documentation

* say the name is not an acronym ([5b6f03c](https://github.com/xraph/dtl/commit/5b6f03cfef7d13c51eb80b515524c06d88e4074d))

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

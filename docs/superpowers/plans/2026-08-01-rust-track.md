# Rust CI Track Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `rust-ci.yml` to `xraph/workflows`, harvested from `octopus`, and wire `octopus` up as its full consumer and `farp`'s Rust job as a partial one.

**Architecture:** Five jobs preserving `octopus`'s fast/slow split — `lint`, `audit` and `test` gate merges; `test-extended` and `docs` are opt-in and non-gating. Per-target Makefile detection is reused verbatim from `go-ci.yml`, so `octopus` keeps its Makefile policy while a bare crate gets cargo defaults. No Rust release workflow is built: cargo-dist owns `octopus`'s releases and semantic-release owns `farp-rust`'s.

**Tech Stack:** GitHub Actions (`workflow_call`), Rust stable/nightly, `cargo-audit`, `Swatinem/rust-cache`, `taiki-e/install-action`.

**Spec:** [`docs/superpowers/specs/2026-08-01-rust-track-design.md`](../specs/2026-08-01-rust-track-design.md)

## Global Constraints

**Three working trees.** Every `git` command is prefixed with its directory. Never cross-commit.

| repo | path | state at plan time |
|---|---|---|
| workflows | `/Users/rexraphael/Work/xraph/go-workflows` | local dir name deliberately unchanged; GitHub repo is `xraph/workflows`. `main` at `bd4b93d`, tag `v1.5.0`, moving `v1` |
| octopus | `/Users/rexraphael/Work/xraph/octopus` | **checked out on `feat/virtualgateway-proxy-mode`, which is 3 behind `origin/main` and 0 ahead** — a stale checkout with no unique work. Tree clean. `origin/main` at `83a3cbb` |
| farp | `/Users/rexraphael/Work/xraph/farp` | `main` at `187745f`, clean |

**Pinned action majors:**

| action | pin |
|---|---|
| `actions/checkout` | `v7` |
| `actions/upload-artifact` | `v7` |
| `Swatinem/rust-cache` | `v2` |
| `taiki-e/install-action` | `v2` |
| `dtolnay/rust-toolchain` | `@master` with an explicit `toolchain:` input |

`dtolnay/rust-toolchain` publishes no releases — `@stable` and `@master` are moving branches. Using `@master` with an explicit `toolchain:` gives one code path for stable, nightly and pinned versions, and is what `octopus`'s own matrix already does.

**No separate cache step.** `Swatinem/rust-cache@v2` caches the registry index, downloaded crates, git dependencies and the target directory. Adding `actions/cache` alongside it is redundant, mirroring the `setup-go` decision in the Go track.

**Absolute rules — each has already caused a real defect in this project:**

1. **No `${{ }}` expression inside any `run:` block.** GitHub substitutes it into the script *source* before bash parses it, so the value becomes code. Route through the step's `env:` and reference as a quoted bash variable. `${{ }}` in `if:`, `with:`, `env:`, `working-directory:` is correct.
2. **A step's own `env:` is not in scope in that step's `if:`.**
3. **A `workflow_call` callee's `permissions:` can only narrow the caller's token**, and GitHub validates every nested job's permissions at parse time, including `if:`-skipped jobs.
4. **Conventional commits. No `Co-Authored-By` trailers of any kind.**

**Permissions differ from the Go track.** Nothing in `rust-ci.yml` uploads SARIF, so every job declares `permissions: contents: read` and **callers need no `permissions:` block at all** — unlike `go-ci.yml`'s callers. State this in the README; the two callers otherwise look interchangeable and are not.

**Environment facts:**
- `cargo` and `rustc` are available locally at `~/.cargo/bin`, so the fixture is testable without CI.
- `gh run list` immediately after a push often returns a **stale** run, and `gh run watch` on a finished run exits green instantly. Match `headSha` against the commit you pushed.
- `actionlint` runs `shellcheck`. For `echo` strings containing literal markdown backticks it reports SC2016; follow the existing precedent in `go-ci.yml` — a narrowly-scoped `# shellcheck disable=SC2016` — after confirming the string really is literal.

**Outward-facing gates.** Tasks 1, 2 and 3 push to public repos and cut a tag. Each is marked **REQUIRES CONFIRMATION**.

---

## File Structure

**`xraph/workflows`:**

| file | responsibility |
|---|---|
| `.github/workflows/rust-ci.yml` | **new** — the reusable Rust CI workflow |
| `testdata/fixture-rust/` | **new** — minimal crate, no Makefile, exercises the cargo fallback path |
| `.github/workflows/self-test.yml` | modified — one smoke job |
| `examples/rust-library/ci.yml` | **new** — copy-paste starter |
| `README.md` | modified |

**`xraph/octopus`:** `.github/workflows/ci.yml` — `lint`, `audit`, `test`, `test-extended`, `docs` replaced by a call; `coverage`, `bench`, `build-release`, `docker` and a new `octopus-checks` job stay local.

**`xraph/farp`:** `.github/workflows/ci.yml` — the `rust-test` job becomes a call.

---

## Task 1: `rust-ci.yml`, fixture, smoke job

**REQUIRES CONFIRMATION** — pushes to `xraph/workflows` and tags `v1.6.0`.

**Files:**
- Create: `.github/workflows/rust-ci.yml`
- Create: `testdata/fixture-rust/{Cargo.toml,Cargo.lock,src/lib.rs}`
- Create: `examples/rust-library/ci.yml`
- Modify: `.github/workflows/self-test.yml`, `README.md`

**Interfaces:**
- Consumes: nothing.
- Produces: `rust-ci.yml` with inputs `working-directory`, `toolchain`, `run-extended`, `extended-matrix`, `build-docs`, `skip-audit`, `system-deps-ubuntu`, `system-deps-macos`. Tasks 2 and 3 call it.

- [ ] **Step 1: Create the fixture crate**

`testdata/fixture-rust/Cargo.toml`:

```toml
[package]
name = "fixture-rust"
version = "0.1.0"
edition = "2021"
publish = false

[dependencies]
```

`testdata/fixture-rust/src/lib.rs`:

```rust
//! Minimal crate used to smoke-test the reusable Rust CI workflow.
//! It deliberately ships no Makefile, so the workflow exercises its
//! cargo fallback commands rather than make targets.

/// Returns the sum of `a` and `b`.
pub fn add(a: i64, b: i64) -> i64 {
    a + b
}

#[cfg(test)]
mod tests {
    use super::add;

    #[test]
    fn adds_two_numbers() {
        assert_eq!(add(2, 3), 5);
    }
}
```

- [ ] **Step 2: Generate and commit `Cargo.lock`**

`cargo audit` reads `Cargo.lock` and fails without one. A library crate would normally gitignore it — this is a fixture, so it is committed deliberately.

```bash
cd /Users/rexraphael/Work/xraph/go-workflows/testdata/fixture-rust
cargo generate-lockfile
ls Cargo.lock
```

Expected: `Cargo.lock` exists. If `.gitignore` in the repo root excludes it, add a negation so it is tracked, and say so in your report.

- [ ] **Step 3: Ignore Rust build output**

Building the fixture locally creates `testdata/fixture-rust/target/`, which is large and must never be committed. The repo's `.gitignore` has no Rust entries yet. Append:

```
target/
```

`Cargo.lock` must stay **tracked** — do not add it to `.gitignore`. Confirm both:

```bash
cd /Users/rexraphael/Work/xraph/go-workflows
git check-ignore -v testdata/fixture-rust/target 2>/dev/null && echo "target ignored (correct)"
git check-ignore -v testdata/fixture-rust/Cargo.lock >/dev/null 2>&1 && echo "PROBLEM: Cargo.lock is ignored" || echo "Cargo.lock tracked (correct)"
```

- [ ] **Step 4: Verify the fixture locally**

```bash
cd /Users/rexraphael/Work/xraph/go-workflows/testdata/fixture-rust
cargo fmt --all -- --check; echo "fmt exit=$?"
cargo clippy --all-targets --all-features -- -D warnings; echo "clippy exit=$?"
cargo test --all-features; echo "test exit=$?"
test -f Makefile && echo "UNEXPECTED Makefile" || echo "no Makefile (correct)"
```

Expected: all three exit 0, and no Makefile. If clippy fails under `-D warnings`, fix the fixture source — do not relax the workflow's fallback.

- [ ] **Step 5: Write `rust-ci.yml`**

```yaml
name: Rust CI

on:
  workflow_call:
    inputs:
      working-directory:
        description: 'Directory containing Cargo.toml'
        type: string
        default: '.'
      toolchain:
        description: 'Rust toolchain for the gating jobs'
        type: string
        default: 'stable'
      run-extended:
        description: 'Run the OS x toolchain matrix. The caller supplies its own policy expression.'
        type: boolean
        default: false
      extended-matrix:
        description: 'JSON array of {os, toolchain} objects'
        type: string
        default: '[{"os":"macos-latest","toolchain":"stable"},{"os":"ubuntu-latest","toolchain":"nightly"}]'
      build-docs:
        description: 'Build rustdoc and upload it as an artifact'
        type: boolean
        default: false
      skip-audit:
        type: boolean
        default: false
      system-deps-ubuntu:
        description: 'Space-separated apt packages, e.g. "pkg-config libssl-dev protobuf-compiler"'
        type: string
        default: ''
      system-deps-macos:
        description: 'Space-separated brew packages, e.g. "protobuf"'
        type: string
        default: ''

jobs:
  lint:
    name: Lint (rustfmt + clippy)
    runs-on: ubuntu-latest
    permissions:
      contents: read
    defaults:
      run:
        shell: bash
        working-directory: ${{ inputs.working-directory }}
    steps:
      - name: Checkout
        uses: actions/checkout@v7

      - name: Install Rust toolchain
        uses: dtolnay/rust-toolchain@master
        with:
          toolchain: ${{ inputs.toolchain }}
          components: rustfmt, clippy

      - name: Cache build artifacts
        uses: Swatinem/rust-cache@v2
        with:
          workspaces: ${{ inputs.working-directory }}

      - name: Install system dependencies
        env:
          DEPS: ${{ inputs.system-deps-ubuntu }}
        run: |
          set -euo pipefail
          if [ -n "$DEPS" ]; then
            sudo apt-get update
            # Package names never contain spaces; word splitting is intended.
            # shellcheck disable=SC2086
            sudo apt-get install -y $DEPS
          fi

      - name: Resolve commands
        id: cmd
        run: |
          set -euo pipefail
          if [ -f Makefile ] && make -n fmt >/dev/null 2>&1; then
            echo 'fmt=make fmt' >> "$GITHUB_OUTPUT"
            echo 'fmtsrc=Makefile target fmt' >> "$GITHUB_OUTPUT"
          else
            echo 'fmt=cargo fmt --all -- --check' >> "$GITHUB_OUTPUT"
            echo 'fmtsrc=fallback cargo fmt' >> "$GITHUB_OUTPUT"
          fi
          if [ -f Makefile ] && make -n clippy >/dev/null 2>&1; then
            echo 'clippy=make clippy' >> "$GITHUB_OUTPUT"
            echo 'clippysrc=Makefile target clippy' >> "$GITHUB_OUTPUT"
          else
            echo 'clippy=cargo clippy --all-targets --all-features -- -D warnings' >> "$GITHUB_OUTPUT"
            echo 'clippysrc=fallback cargo clippy' >> "$GITHUB_OUTPUT"
          fi

      - name: Check formatting
        env:
          CMD: ${{ steps.cmd.outputs.fmt }}
          SRC: ${{ steps.cmd.outputs.fmtsrc }}
        run: |
          set -euo pipefail
          printf -- '- fmt: %s\n' "$SRC" >> "$GITHUB_STEP_SUMMARY"
          eval "$CMD"

      - name: Run clippy
        env:
          CMD: ${{ steps.cmd.outputs.clippy }}
          SRC: ${{ steps.cmd.outputs.clippysrc }}
        run: |
          set -euo pipefail
          printf -- '- clippy: %s\n' "$SRC" >> "$GITHUB_STEP_SUMMARY"
          eval "$CMD"

  audit:
    name: Security Audit
    if: ${{ !inputs.skip-audit }}
    runs-on: ubuntu-latest
    permissions:
      contents: read
    defaults:
      run:
        shell: bash
        working-directory: ${{ inputs.working-directory }}
    steps:
      - name: Checkout
        uses: actions/checkout@v7

      - name: Install Rust toolchain
        uses: dtolnay/rust-toolchain@master
        with:
          toolchain: ${{ inputs.toolchain }}

      # Prebuilt binary (seconds) rather than `cargo install` from source (minutes).
      - name: Install cargo-audit
        uses: taiki-e/install-action@v2
        with:
          tool: cargo-audit

      - name: Resolve audit command
        id: cmd
        run: |
          set -euo pipefail
          if [ -f Makefile ] && make -n audit >/dev/null 2>&1; then
            echo 'run=make audit' >> "$GITHUB_OUTPUT"
            echo 'source=Makefile target audit' >> "$GITHUB_OUTPUT"
          else
            echo 'run=cargo audit' >> "$GITHUB_OUTPUT"
            echo 'source=fallback cargo audit' >> "$GITHUB_OUTPUT"
          fi

      - name: Run audit
        env:
          CMD: ${{ steps.cmd.outputs.run }}
          SRC: ${{ steps.cmd.outputs.source }}
        run: |
          set -euo pipefail
          printf -- '- audit: %s\n' "$SRC" >> "$GITHUB_STEP_SUMMARY"
          eval "$CMD"

  test:
    name: Test (ubuntu / ${{ inputs.toolchain }})
    runs-on: ubuntu-latest
    permissions:
      contents: read
    defaults:
      run:
        shell: bash
        working-directory: ${{ inputs.working-directory }}
    steps:
      - name: Checkout
        uses: actions/checkout@v7

      - name: Install Rust toolchain
        uses: dtolnay/rust-toolchain@master
        with:
          toolchain: ${{ inputs.toolchain }}

      - name: Cache build artifacts
        uses: Swatinem/rust-cache@v2
        with:
          workspaces: ${{ inputs.working-directory }}
          key: ubuntu-${{ inputs.toolchain }}

      - name: Install system dependencies
        env:
          DEPS: ${{ inputs.system-deps-ubuntu }}
        run: |
          set -euo pipefail
          if [ -n "$DEPS" ]; then
            sudo apt-get update
            # shellcheck disable=SC2086
            sudo apt-get install -y $DEPS
          fi

      - name: Resolve commands
        id: cmd
        run: |
          set -euo pipefail
          if [ -f Makefile ] && make -n build >/dev/null 2>&1; then
            echo 'build=make build' >> "$GITHUB_OUTPUT"
            echo 'buildsrc=Makefile target build' >> "$GITHUB_OUTPUT"
          else
            echo 'build=cargo build --all-features' >> "$GITHUB_OUTPUT"
            echo 'buildsrc=fallback cargo build' >> "$GITHUB_OUTPUT"
          fi
          if [ -f Makefile ] && make -n test >/dev/null 2>&1; then
            echo 'test=make test' >> "$GITHUB_OUTPUT"
            echo 'testsrc=Makefile target test' >> "$GITHUB_OUTPUT"
          else
            echo 'test=cargo test --all-features' >> "$GITHUB_OUTPUT"
            echo 'testsrc=fallback cargo test' >> "$GITHUB_OUTPUT"
          fi

      - name: Build
        env:
          CMD: ${{ steps.cmd.outputs.build }}
          SRC: ${{ steps.cmd.outputs.buildsrc }}
        run: |
          set -euo pipefail
          printf -- '- build: %s\n' "$SRC" >> "$GITHUB_STEP_SUMMARY"
          eval "$CMD"

      - name: Run tests
        env:
          CMD: ${{ steps.cmd.outputs.test }}
          SRC: ${{ steps.cmd.outputs.testsrc }}
        run: |
          set -euo pipefail
          printf -- '- test: %s\n' "$SRC" >> "$GITHUB_STEP_SUMMARY"
          eval "$CMD"

  test-extended:
    name: Test Extended (${{ matrix.os }} / ${{ matrix.toolchain }})
    if: inputs.run-extended
    runs-on: ${{ matrix.os }}
    permissions:
      contents: read
    strategy:
      fail-fast: false
      matrix:
        include: ${{ fromJSON(inputs.extended-matrix) }}
    defaults:
      run:
        shell: bash
        working-directory: ${{ inputs.working-directory }}
    steps:
      - name: Checkout
        uses: actions/checkout@v7

      - name: Install Rust toolchain
        uses: dtolnay/rust-toolchain@master
        with:
          toolchain: ${{ matrix.toolchain }}

      - name: Cache build artifacts
        uses: Swatinem/rust-cache@v2
        with:
          workspaces: ${{ inputs.working-directory }}
          key: ${{ matrix.os }}-${{ matrix.toolchain }}

      - name: Install system dependencies (Ubuntu)
        if: startsWith(matrix.os, 'ubuntu')
        env:
          DEPS: ${{ inputs.system-deps-ubuntu }}
        run: |
          set -euo pipefail
          if [ -n "$DEPS" ]; then
            sudo apt-get update
            # shellcheck disable=SC2086
            sudo apt-get install -y $DEPS
          fi

      - name: Install system dependencies (macOS)
        if: startsWith(matrix.os, 'macos')
        env:
          DEPS: ${{ inputs.system-deps-macos }}
        run: |
          set -euo pipefail
          if [ -n "$DEPS" ]; then
            # shellcheck disable=SC2086
            brew install $DEPS
          fi

      - name: Resolve commands
        id: cmd
        run: |
          set -euo pipefail
          if [ -f Makefile ] && make -n build >/dev/null 2>&1; then
            echo 'build=make build' >> "$GITHUB_OUTPUT"
          else
            echo 'build=cargo build --all-features' >> "$GITHUB_OUTPUT"
          fi
          if [ -f Makefile ] && make -n test >/dev/null 2>&1; then
            echo 'test=make test' >> "$GITHUB_OUTPUT"
          else
            echo 'test=cargo test --all-features' >> "$GITHUB_OUTPUT"
          fi

      - name: Build
        env:
          CMD: ${{ steps.cmd.outputs.build }}
        run: |
          set -euo pipefail
          eval "$CMD"

      - name: Run tests
        env:
          CMD: ${{ steps.cmd.outputs.test }}
        run: |
          set -euo pipefail
          eval "$CMD"

  docs:
    name: Documentation
    if: inputs.build-docs
    runs-on: ubuntu-latest
    permissions:
      contents: read
    defaults:
      run:
        shell: bash
        working-directory: ${{ inputs.working-directory }}
    steps:
      - name: Checkout
        uses: actions/checkout@v7

      - name: Install Rust toolchain
        uses: dtolnay/rust-toolchain@master
        with:
          toolchain: ${{ inputs.toolchain }}

      - name: Cache build artifacts
        uses: Swatinem/rust-cache@v2
        with:
          workspaces: ${{ inputs.working-directory }}

      - name: Install system dependencies
        env:
          DEPS: ${{ inputs.system-deps-ubuntu }}
        run: |
          set -euo pipefail
          if [ -n "$DEPS" ]; then
            sudo apt-get update
            # shellcheck disable=SC2086
            sudo apt-get install -y $DEPS
          fi

      - name: Resolve docs command
        id: cmd
        run: |
          set -euo pipefail
          if [ -f Makefile ] && make -n docs >/dev/null 2>&1; then
            echo 'run=make docs' >> "$GITHUB_OUTPUT"
          else
            echo 'run=cargo doc --no-deps --all-features' >> "$GITHUB_OUTPUT"
          fi

      - name: Build documentation
        env:
          CMD: ${{ steps.cmd.outputs.run }}
        run: |
          set -euo pipefail
          eval "$CMD"

      - name: Upload documentation
        uses: actions/upload-artifact@v7
        with:
          name: documentation
          path: ${{ inputs.working-directory }}/target/doc
```

- [ ] **Step 6: Run actionlint, expect it to pass**

```bash
cd /Users/rexraphael/Work/xraph/go-workflows && "$(go env GOPATH)/bin/actionlint" -color; echo "exit=$?"
```

Expected: exit 0. If SC2016 fires on a `printf`/`echo` string, confirm the string is literal before adding a narrowly-scoped `# shellcheck disable=SC2016`.

- [ ] **Step 7: Add the smoke job to `self-test.yml`**

Append to the `jobs:` block. Note the job-level `permissions:` — GitHub validates every nested job's permissions at parse time, so this is required even though `rust-ci.yml` only needs read access.

```yaml
  smoke-rust:
    name: Smoke — Rust fallback path
    permissions:
      contents: read
    uses: ./.github/workflows/rust-ci.yml
    with:
      working-directory: testdata/fixture-rust
      skip-audit: false
```

`run-extended` and `build-docs` stay at their defaults so the smoke run is fast; `skip-audit: false` is explicit because the audit path is the one most likely to break on a fixture without dependencies.

- [ ] **Step 8: Write `examples/rust-library/ci.yml`**

```yaml
# Copy to .github/workflows/ci.yml in a Rust repository.
#
# Unlike the Go CI caller, this needs no `permissions:` block — nothing in
# rust-ci.yml uploads SARIF, so the default read token is sufficient.
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
  schedule:
    # Exercise the expensive legs off the PR path so per-PR feedback stays fast.
    - cron: '0 6 * * *'

concurrency:
  group: ci-${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: ${{ github.ref != 'refs/heads/main' }}

jobs:
  ci:
    uses: xraph/workflows/.github/workflows/rust-ci.yml@v1
    with:
      # The caller owns the policy for when the expensive matrix runs.
      run-extended: ${{ github.event_name == 'schedule' || github.ref == 'refs/heads/main' }}
      build-docs: ${{ github.event_name == 'push' && github.ref == 'refs/heads/main' }}
      # Rust crates often need native toolchains. Leave empty if yours does not.
      system-deps-ubuntu: 'pkg-config libssl-dev protobuf-compiler'
      system-deps-macos: 'protobuf'
```

- [ ] **Step 9: Document the Rust track in `README.md`**

Add `rust-ci.yml` to the `## Workflows` table, and append:

```markdown
## Rust track

`rust-ci.yml` runs lint (rustfmt + clippy), a security audit (`cargo-audit`),
and build + test. Two further jobs are opt-in and non-gating: `test-extended`
(an OS × toolchain matrix) and `docs`.

That split is deliberate — only the fast jobs gate merges, so the expensive
legs can run on `main` and a nightly cron without slowing per-PR feedback.
The caller decides when, by passing its own expression to `run-extended`.

Like `go-ci.yml`, commands are resolved **per Makefile target**: if the repo has
a Makefile defining `fmt`, `clippy`, `audit`, `build`, `test` or `docs`, that
target is used; otherwise the equivalent `cargo` command runs. Each job records
which it chose in the run summary.

**Callers need no `permissions:` block.** Nothing here uploads SARIF, so the
default read token suffices — unlike `go-ci.yml`, whose callers must grant
`security-events: write`.

Rust crates frequently need native build dependencies. Pass them with
`system-deps-ubuntu` (apt) and `system-deps-macos` (brew); both default to empty.

### There is deliberately no Rust release workflow

`octopus` generates its release pipeline with cargo-dist from
`dist-workspace.toml`, and `farp-rust` is released by `farp`'s semantic-release
because its version is `farp`'s version. Both work; neither should be replaced
by a shared workflow.
```

- [ ] **Step 10: Run actionlint again, expect it to pass**

```bash
cd /Users/rexraphael/Work/xraph/go-workflows && "$(go env GOPATH)/bin/actionlint" -color; echo "exit=$?"
```

Expected: exit 0. This run also validates the local `uses: ./.github/workflows/rust-ci.yml` call, so an input-name typo fails here rather than in CI.

- [ ] **Step 11: Commit, push, and confirm the smoke job passes** — CONFIRM FIRST

```bash
cd /Users/rexraphael/Work/xraph/go-workflows
git add .github/workflows/rust-ci.yml .github/workflows/self-test.yml examples/rust-library/ci.yml README.md testdata/fixture-rust
git status --short
git commit -m "feat: add reusable rust-ci workflow"
git push
sleep 15
gh run list --workflow=self-test.yml --limit 3 --json databaseId,headSha,status -q '.[] | "\(.status) \(.headSha[0:7]) \(.databaseId)"'
```

Identify the run whose `headSha` matches your commit, then:

```bash
gh run watch <that run id> --exit-status
```

- [ ] **Step 12: Confirm the fallback paths were actually taken**

The fixture has no Makefile, so every resolved command must be a `cargo` fallback. Read the run summary rather than trusting the green tick:

```bash
cd /Users/rexraphael/Work/xraph/go-workflows
gh run view <run id> --json jobs -q '.jobs[] | select(.name|test("Rust")) | "\(.conclusion)  \(.name)"'
gh run view --web
```

Expected in the `smoke-rust` jobs' summaries: `fmt: fallback cargo fmt`, `clippy: fallback cargo clippy`, `audit: fallback cargo audit`, `build: fallback cargo build`, `test: fallback cargo test`. Also confirm `test-extended` and `docs` were **skipped**, since `run-extended` and `build-docs` default to false.

If `audit` fails because `Cargo.lock` is absent from the fixture, that is Step 2 not having landed — fix it there rather than passing `skip-audit: true`.

- [ ] **Step 13: Tag `v1.6.0`** — CONFIRM FIRST

```bash
cd /Users/rexraphael/Work/xraph/go-workflows
git tag -a v1.6.0 -m "Release v1.6.0 - add the Rust CI track"
git push origin v1.6.0
sleep 15
gh run list --workflow=release.yml --limit 2 --json databaseId,headSha,status -q '.[] | "\(.status) \(.headSha[0:7]) \(.databaseId)"'
```

Watch the matching run, then prove the moving tag advanced:

```bash
git fetch --tags --force
git rev-parse v1^{commit} v1.6.0^{commit}
```

Expected: both print the same SHA.

---

## Task 2: Migrate `octopus`

**REQUIRES CONFIRMATION** — pushes to `xraph/octopus` and opens a PR.

This is not a mechanical substitution. `octopus`'s `test` job mixes shared concerns with local ones, and those must be split out.

**Files:**
- Modify: `/Users/rexraphael/Work/xraph/octopus/.github/workflows/ci.yml`

**Interfaces:**
- Consumes: `rust-ci.yml@v1` from Task 1.
- Produces: the migration pattern Task 3 adapts for `farp`.

- [ ] **Step 1: Get onto `main` safely**

The working tree is on `feat/virtualgateway-proxy-mode`, which is 3 commits behind `origin/main` and **0 ahead** — a stale checkout carrying no unique work. Verify that is still true before switching, because switching would otherwise risk losing commits:

```bash
cd /Users/rexraphael/Work/xraph/octopus
git fetch
git rev-list --left-right --count origin/main...HEAD
git status --porcelain | wc -l
```

Expected: `3	0` and `0`. **If the second number is not 0, stop and report BLOCKED** — the branch has unique work and this plan's assumption no longer holds.

```bash
git checkout main && git pull
git checkout -b ci/shared-rust-workflow
```

- [ ] **Step 2: Record the current job list, to verify nothing is lost**

```bash
cd /Users/rexraphael/Work/xraph/octopus
grep -nE "^  [a-z-]+:$" .github/workflows/ci.yml
```

Expected: `lint`, `audit`, `test`, `test-extended`, `coverage`, `bench`, `build-release`, `docker`, `docs`, `ci-success`. Keep this list; Step 6 checks it against the result.

- [ ] **Step 3: Replace the five shared jobs with one call**

Delete the `lint`, `audit`, `test`, `test-extended` and `docs` job definitions. In their place add:

```yaml
  ci:
    uses: xraph/workflows/.github/workflows/rust-ci.yml@v1
    with:
      run-extended: ${{ github.event_name == 'schedule' || (github.event_name == 'push' && github.ref == 'refs/heads/main') }}
      build-docs: ${{ github.event_name == 'push' && github.ref == 'refs/heads/main' }}
      system-deps-ubuntu: 'pkg-config libssl-dev protobuf-compiler'
      system-deps-macos: 'protobuf'
```

The `run-extended` and `build-docs` expressions reproduce exactly the `if:` conditions the deleted `test-extended` and `docs` jobs carried, so scheduling behavior is unchanged.

- [ ] **Step 4: Move `octopus`-specific steps into their own job**

The deleted `test` job contained two steps that are not shared concerns: the vendored Helm CRD staleness check and the examples build. They must survive.

```yaml
  octopus-checks:
    name: Octopus-specific checks
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      - name: Checkout code
        uses: actions/checkout@v7

      - name: Install Rust toolchain
        uses: dtolnay/rust-toolchain@master
        with:
          toolchain: stable

      - name: Cache build artifacts
        uses: Swatinem/rust-cache@v2
        with:
          key: octopus-checks

      - name: Install system dependencies
        run: |
          sudo apt-get update
          sudo apt-get install -y pkg-config libssl-dev protobuf-compiler

      # The protocol's CRDs are vendored into the Helm chart. A drift here ships
      # a chart that does not match the binary, so this is a build gate.
      - name: Check vendored Helm CRDs are up to date
        run: |
          cargo run -q --all-features --bin octopus -- crd > /tmp/octopus-crds.yaml
          if ! diff -u deploy/helm/octopus/crds/octopus-crds.yaml /tmp/octopus-crds.yaml; then
            echo "::error::Vendored Helm CRDs are stale. Regenerate with: octopus crd > deploy/helm/octopus/crds/octopus-crds.yaml"
            exit 1
          fi

      - name: Build examples
        run: cargo build --examples --all-features
```

- [ ] **Step 5: Update the `ci-success` gate**

It currently reads `needs: [lint, audit, test]` and checks each result. Those jobs no longer exist under those names. Replace the job with:

```yaml
  ci-success:
    name: CI Success
    needs: [ci, octopus-checks]
    runs-on: ubuntu-latest
    if: always()
    steps:
      - name: Verify required jobs succeeded
        env:
          CI_RESULT: ${{ needs.ci.result }}
          CHECKS_RESULT: ${{ needs.octopus-checks.result }}
        run: |
          set -euo pipefail
          failed=0
          for entry in "ci:${CI_RESULT}" "octopus-checks:${CHECKS_RESULT}"; do
            name="${entry%%:*}"; status="${entry##*:}"
            if [ "$status" != "success" ]; then
              echo "::error::Required CI job '$name' did not succeed (result: $status)"
              failed=1
            fi
          done
          [ "$failed" -eq 0 ] || exit 1
          echo "All required CI jobs passed."
```

Note the values now come through `env:` rather than being interpolated into the script — the repository-wide rule. The loop also reports *every* failure before exiting, rather than stopping at the first.

Any other job that listed `lint`, `audit` or `test` in its `needs:` must be repointed to `ci`. Search for them:

```bash
cd /Users/rexraphael/Work/xraph/octopus
grep -n "needs:" .github/workflows/ci.yml
```

- [ ] **Step 6: Verify no job was lost**

```bash
cd /Users/rexraphael/Work/xraph/octopus
grep -nE "^  [a-z-]+:$" .github/workflows/ci.yml
```

Expected now: `coverage`, `bench`, `build-release`, `docker`, `ci`, `octopus-checks`, `ci-success`. Compare against Step 2's list — `lint`, `audit`, `test`, `test-extended` and `docs` are gone because `ci` subsumes them; everything else must still be present.

- [ ] **Step 7: Lint locally**

```bash
cd /Users/rexraphael/Work/xraph/octopus && "$(go env GOPATH)/bin/actionlint" -color; echo "exit=$?"
```

Expected: exit 0. `actionlint` cannot resolve the remote `@v1` reference, so this checks local syntax and `needs:` wiring only.

- [ ] **Step 8: Push and open the PR** — CONFIRM FIRST

```bash
cd /Users/rexraphael/Work/xraph/octopus
git add .github/workflows/ci.yml
git status --short
git commit -m "ci: adopt shared xraph/workflows rust-ci"
git push -u origin ci/shared-rust-workflow
gh pr create --fill --title "ci: adopt shared rust-ci workflow"
gh pr checks --watch
```

Expected: `ci` (lint, audit, test), `octopus-checks`, `coverage`, `bench`, `build-release`, `docker` and `ci-success` all pass. `test-extended` and `docs` should be **skipped** on a PR, since both expressions are false there.

If a job fails, diagnose from the real logs. Do **not** weaken `rust-ci.yml` — it is published and `farp` adopts it next. If clippy fails because the shared fallback denies warnings while `octopus`'s Makefile did not, note that `octopus` has a Makefile, so the probe should prefer `make clippy` — if it did not, that is a genuine finding in the probe and worth reporting rather than working around.

- [ ] **Step 9: Merge**

```bash
cd /Users/rexraphael/Work/xraph/octopus
gh pr merge --squash --delete-branch
git checkout main && git pull
```

---

## Task 3: Migrate `farp`'s Rust job

**REQUIRES CONFIRMATION** — pushes to `xraph/farp` and opens a PR.

**Files:**
- Modify: `/Users/rexraphael/Work/xraph/farp/.github/workflows/ci.yml`

**Interfaces:**
- Consumes: `rust-ci.yml@v1`, and the migration pattern from Task 2.
- Produces: nothing later depends on.

- [ ] **Step 1: Branch**

```bash
cd /Users/rexraphael/Work/xraph/farp
git checkout main && git pull
git checkout -b ci/shared-rust-workflow
```

- [ ] **Step 2: Replace the `rust-test` job**

The existing `rust-test` job guards every step on `[ -f farp-rust/Cargo.toml ]`. That was defensive; the crate exists and is `farp` v1.3.0's published Rust binding, so the caller is unconditional.

Delete the whole `rust-test` job and add:

```yaml
  rust-test:
    name: Rust Tests
    uses: xraph/workflows/.github/workflows/rust-ci.yml@v1
    with:
      working-directory: farp-rust
      run-extended: ${{ github.event_name == 'push' && github.ref == 'refs/heads/main' }}
```

`farp-rust` has no native dependencies, so `system-deps-ubuntu` and `system-deps-macos` stay empty. `build-docs` stays false — `farp` publishes to docs.rs, which builds its own documentation.

**One capability is dropped deliberately:** the old job ran `cargo tarpaulin` for coverage under `continue-on-error: true`. `rust-ci.yml` has no coverage job, and a step that could never fail was not gating anything. Report this in your report so it is a recorded decision rather than a silent loss.

- [ ] **Step 3: Check nothing else referenced the old job**

```bash
cd /Users/rexraphael/Work/xraph/farp
grep -n "rust-test\|needs:" .github/workflows/ci.yml
```

Any `needs: [... rust-test ...]` still resolves, since the job keeps its name. Confirm no step referenced the deleted `check_cargo` step output.

- [ ] **Step 4: Lint locally**

```bash
cd /Users/rexraphael/Work/xraph/farp && "$(go env GOPATH)/bin/actionlint" -color; echo "exit=$?"
```

Expected: exit 0.

- [ ] **Step 5: Push and open the PR** — CONFIRM FIRST

```bash
cd /Users/rexraphael/Work/xraph/farp
git add .github/workflows/ci.yml
git status --short
git commit -m "ci: adopt shared xraph/workflows rust-ci for farp-rust"
git push -u origin ci/shared-rust-workflow
gh pr create --fill --title "ci: adopt shared rust-ci workflow for farp-rust"
gh pr checks --watch
```

Expected: `rust-test` passes with lint, audit and test. `farp`'s Go jobs, its CodeQL job and its semantic-release pipeline are untouched and should be unaffected.

`farp-rust` has no committed `Cargo.lock` if it is a library crate. If `audit` fails for that reason, the correct fix is to pass `skip-audit: true` for `farp` **and say so**, or to commit a lockfile — decide based on whether `farp-rust` is a library or a binary, and report which you chose and why.

- [ ] **Step 6: Merge and confirm the end state**

```bash
cd /Users/rexraphael/Work/xraph/farp
gh pr merge --squash --delete-branch
git checkout main && git pull

for r in octopus farp; do
  echo "=== $r ==="
  gh run list --repo "xraph/$r" --limit 3 --json name,conclusion -q '[.[]|"\(.name):\(.conclusion)"]|join("  ")'
done
```

Expected: both repos green on their most recent runs.

---

## As-built: what changed after this plan was written

Executed. `rust-ci.yml` shipped in `xraph/workflows` `v1.6.0`, hardened in `v1.7.0`.
`farp` migrated and merged; `octopus` migrated but **left unmerged** — see below.

| # | Found by | Change |
|---|---|---|
| 1 | Task 1 implementer | This plan's Step 11 `git add` list omitted `.gitignore`, which Step 3 required editing. Included anyway, or the `target/` ignore rule would never have reached the repo. |
| 2 | Task 2 implementer | `octopus`'s `main` has failed CI on seven consecutive nightlies since 2026-07-26 — a real `clippy::question_mark` violation plus RUSTSEC advisories, all pre-dating this work. PR #5 left open rather than merging past a red required gate. |
| 3 | Final review | **This plan's Task 2 Step 5 introduced a gating regression.** Bundling all five shared jobs behind one `ci` caller and gating `ci-success` on it meant a nightly-toolchain or rustdoc failure would redden the required check — `main`'s pre-migration policy deliberately excluded those. Fixed by adding an `only-extended` input, letting `octopus` split into a gating `ci` and a non-gating `ci-extended`. Shipped `v1.7.0`. |
| 4 | Final review | `test-extended` and `docs` resolved commands but wrote nothing to the step summary, contradicting both the README and this plan. Now they report, like every other job. |
| 5 | Final review | `cargo-audit` needs a `Cargo.lock`, which library crates conventionally gitignore — so `farp` had to pass `skip-audit: true` and ship unaudited. The fallback now runs `[ -f Cargo.lock ] \|\| cargo generate-lockfile` first, so library crates are audited by default. |
| 6 | Final review | `upload-artifact` had no `if-no-files-found: error` (a docs command writing outside `target/doc` gave a green run and an empty artifact) and used a constant artifact name (a monorepo calling twice would collide). Added the guard and a `docs-artifact-name` input. |

### Deferred items, subsequently closed

| item | resolution |
|---|---|
| `octopus` `main` red, PR #5 unmerged | Fixed both causes and merged. The clippy failure was `question_mark` firing only under **clippy 1.97** (CI's stable) while local was 1.96 — a new-lint-on-toolchain-bump, not stale code. Replaced the explicit `return None` with `?`; 84 router tests pass. `crossbeam-epoch 0.9.18 → 0.9.20` cleared RUSTSEC-2026-0204. Merged as `xraph/octopus` PR #6, then PR #5 rebased onto the green base and merged — removing 149 lines from octopus's CI. |
| No Rust partial-Makefile fixture | Added `testdata/fixture-rust-make` (defines `fmt` and `test`, deliberately not `clippy`/`audit`/`build`/`docs`) plus a smoke job. The run log now shows the mixed resolution directly: `Makefile target fmt`, `Makefile target test`, `fallback cargo clippy`, `fallback cargo audit`, `fallback cargo build`. Shipped `v1.8.0`. |
| No `cargo-flags` escape hatch | Added; threaded through every fallback so a crate with mutually exclusive features can pass `--features x` instead of being forced to add a Makefile. Shipped `v1.8.0`. |
| `farp` lost per-PR `cargo doc` | Restored via `build-docs: true` with `docs-artifact-name: farp-rust-docs`. |
| `farp` shipped unaudited (`skip-audit: true`) | Removed. The `v1.7.0` lockfile-generation fix means `cargo audit` now runs on `farp-rust` despite its gitignored `Cargo.lock` — verified green in production. |

**Still deferred**, recorded rather than silently dropped:

- `farp` lost its per-PR `cargo doc` run; `build-docs` defaults to false and `farp` publishes to docs.rs. Broken intra-doc links will first surface at `cargo publish`.
- `farp` lost a `cargo tarpaulin` coverage step that ran under `continue-on-error: true` and gated nothing.
- No Rust fixture covers the *partial-Makefile* case, unlike the Go track's `fixture-partial-make`. The make branch is proven by `octopus` PR #5's logs (`CMD: make clippy`, `CMD: make audit`) rather than by `self-test.yml`.
- No `cargo-flags` escape hatch, so a crate with mutually exclusive features cannot use `--all-features` without adding a Makefile.
- `dtolnay/rust-toolchain@master` is a moving branch on a third-party action, now at five sites plus `octopus-checks`.

## Follow-up, explicitly not in this plan

- **Phase 3 — Node/TypeScript**, six repos (`forge-js`, `stockgist`, `tkm`, `tkm-website`, `website`, `xraph`), predominantly private Next.js applications needing CI and deploy rather than npm publishing.
- **Phase 4 — Dart/Flutter**, `game-cli` and `gameframework`, including `game-cli`'s cross-platform binary matrix and Homebrew formula.
- A Rust coverage job, if `farp` or `octopus` later wants the `tarpaulin` capability dropped in Task 3 back as a shared concern.
- Carried over and still open: `confy`'s CodeQL default-setup conflict, branch protection on `xraph/workflows`, SHA-pinning `ncipollo/release-action`, and `go-utils`'s 13 Dependabot alerts.

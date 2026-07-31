# Shared Go Release Workflows Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up `xraph/go-workflows` — reusable GitHub Actions workflows for the org's Go library and Go CLI tracks — and wire `xraph/dtl` up as its first consumer.

**Architecture:** Four `on: workflow_call` reusable workflows (`go-ci`, `go-release`, `go-binary-release`, `codeql`) live in one repository, versioned with semver plus a moving `v1` tag that consumers pin to. Consumer repositories are reduced to a trigger block and a `uses:` line. Repository identity is derived from `github.repository` rather than typed per repo, which removes the class of bug currently live in `confy`'s release notes.

**Tech Stack:** GitHub Actions (`workflow_call`), Go 1.24–1.26, `golangci-lint`, `gosec`, `govulncheck`, `ncipollo/release-action`, GoReleaser, `actionlint`.

**Spec:** [`docs/superpowers/specs/2026-07-30-shared-go-release-workflows-design.md`](../specs/2026-07-30-shared-go-release-workflows-design.md)

## Global Constraints

These apply to every task. Values were resolved on 2026-07-30 and are the pinned defaults referred to throughout.

**Two working trees.** Tasks touch two repositories:
- `/Users/rexraphael/Work/xraph/go-workflows` — new, created in Task 1
- `/Users/rexraphael/Work/xraph/dtl` — existing, on `main`, remote `git@github.com:xraph/dtl.git`

Every `git` command below is prefixed with the directory it runs in. Never commit `go-workflows` content into `dtl` or vice versa.

**Pinned third-party action majors** (latest as of 2026-07-30):

| action | pin |
|---|---|
| `actions/checkout` | `v7` |
| `actions/setup-go` | `v7` |
| `codecov/codecov-action` | `v7` |
| `golangci/golangci-lint-action` | `v9` |
| `ncipollo/release-action` | `v1` |
| `goreleaser/goreleaser-action` | `v7` |
| `github/codeql-action` | `v4` |

**Pinned tool version defaults** (latest stable as of 2026-07-30):

| tool | pin |
|---|---|
| `golangci-lint` | `v2.12.2` |
| `gosec` | `v2.28.0` |
| `govulncheck` (`golang.org/x/vuln`) | `v1.6.0` |
| `goreleaser` | `v2.17.1` |
| `actionlint` | `v1.7.12` |

Note these are newer than the existing repos' `golangci-lint v2.6.1`. That is intentional — the shared repo starts current.

**No `actions/cache` step.** `actions/setup-go@v7` caches modules and build output itself via `cache: true` (the default) plus `cache-dependency-path`. The manual `actions/cache` block in today's workflows is redundant and is not carried over.

**Cache key is `go.mod`, not `go.sum`.** `actions/setup-go` fails the job outright when `cache-dependency-path` matches no file, and a module with no dependencies has no `go.sum`. `go.mod` always exists, so it can never produce that failure. The trade-off is a slightly staler cache when `go.sum` changes without `go.mod` changing — a minor performance cost, versus a hard failure for any dependency-free consumer.

**Never put a `${{ }}` expression inside a `run:` block.** GitHub substitutes `${{ }}` into the script *source* before bash parses it, so the value becomes code rather than data. A commit message containing an apostrophe then breaks the quoting, and a crafted one executes arbitrary shell in a job holding `contents: write`. Pass every value through the step's `env:` block and reference it as a quoted bash variable:

```yaml
env:
  BODY: ${{ steps.changelog.outputs.body }}
run: printf '%s\n' "$BODY"
```

Use `printf '%s\n'` rather than `echo` so leading dashes and backslashes are not interpreted. `${{ }}` in `if:`, `with:`, `env:`, and `working-directory:` is correct and stays. This rule is absolute and applies to every workflow in the repository — an earlier draft of `go-release.yml` violated it and shipped a real shell-injection vulnerability, fixed in `v1.1.1`.

**Multiline `$GITHUB_OUTPUT` needs a randomized heredoc delimiter.** A fixed delimiter can be closed early by content that happens to match it. Generate one per run: `DELIM="GHEOF_$(openssl rand -hex 8)"`.

**`GITHUB_TOKEN` scope note.** The local `gh` token lacks the `workflow` scope, so workflow files cannot be pushed over HTTPS. Git is configured for SSH (`git@github.com:...`), which uses the SSH key and is unaffected. All pushes in this plan use SSH remotes.

**Outward-facing gates.** Tasks 3, 4, 6 and 7 create a public repository, open a pull request, or publish a release. Each is marked **REQUIRES CONFIRMATION** and must not proceed until the user explicitly approves that specific action.

**Commit style.** Conventional commits. No `Co-Authored-By` trailers.

---

## File Structure

**`xraph/go-workflows`** (new repository):

| file | responsibility |
|---|---|
| `.github/workflows/go-ci.yml` | reusable: test matrix, lint, verify, security |
| `.github/workflows/go-release.yml` | reusable: tag-push + dispatch release |
| `.github/workflows/go-binary-release.yml` | reusable: GoReleaser + Homebrew/Scoop |
| `.github/workflows/codeql.yml` | reusable: Go CodeQL |
| `.github/workflows/self-test.yml` | this repo's CI: actionlint + two smoke calls |
| `.github/workflows/release.yml` | tags this repo, retargets the moving `v1` |
| `testdata/fixture-lib/` | Go module with **no** Makefile — exercises the raw-`go` fallback |
| `testdata/fixture-partial-make/` | Go module with a Makefile defining `test` but not `test-coverage` |
| `examples/library-caller/` | copy-paste starters: `ci.yml`, `release.yml`, `codeql.yml` |
| `examples/cli-caller/` | copy-paste starter for the binary track |
| `README.md` | usage, inputs, version table, recovery procedures |

**`xraph/dtl`** (existing repository):

| file | responsibility |
|---|---|
| `.github/workflows/ci.yml` | calls `go-ci.yml@v1` |
| `.github/workflows/release.yml` | calls `go-release.yml@v1` |
| `.github/workflows/codeql.yml` | calls `codeql.yml@v1` |

---

## Task 1: Bootstrap `go-workflows` with fixtures and actionlint

**Files:**
- Create: `/Users/rexraphael/Work/xraph/go-workflows/README.md`
- Create: `/Users/rexraphael/Work/xraph/go-workflows/.gitignore`
- Create: `/Users/rexraphael/Work/xraph/go-workflows/testdata/fixture-lib/{go.mod,fixture.go,fixture_test.go}`
- Create: `/Users/rexraphael/Work/xraph/go-workflows/testdata/fixture-partial-make/{go.mod,Makefile,fixture.go,fixture_test.go}`
- Create: `/Users/rexraphael/Work/xraph/go-workflows/.github/workflows/self-test.yml`

**Interfaces:**
- Consumes: nothing (first task)
- Produces: two fixture module paths used as `working-directory` values by Task 2's smoke jobs — `testdata/fixture-lib` and `testdata/fixture-partial-make`. Both modules expose `func Add(a, b int) int`.

- [ ] **Step 1: Create the repository skeleton**

```bash
mkdir -p /Users/rexraphael/Work/xraph/go-workflows/.github/workflows
mkdir -p /Users/rexraphael/Work/xraph/go-workflows/testdata/fixture-lib
mkdir -p /Users/rexraphael/Work/xraph/go-workflows/testdata/fixture-partial-make
mkdir -p /Users/rexraphael/Work/xraph/go-workflows/examples/library-caller
mkdir -p /Users/rexraphael/Work/xraph/go-workflows/examples/cli-caller
cd /Users/rexraphael/Work/xraph/go-workflows && git init -b main
```

- [ ] **Step 2: Write the no-Makefile fixture**

The fixture takes a real dependency on purpose. A module with zero dependencies has no `go.sum`, and `actions/setup-go` fails hard when its `cache-dependency-path` matches no file — so a dependency-free fixture would fail for a reason that has nothing to do with what is being tested. It also makes `go mod download` do actual work.

`testdata/fixture-lib/go.mod`:

```
module github.com/xraph/go-workflows/testdata/fixture-lib

go 1.24

require github.com/google/uuid v1.6.0
```

`testdata/fixture-lib/fixture.go`:

```go
// Package fixture is a minimal module used to smoke-test the reusable workflows.
package fixture

import "github.com/google/uuid"

// Add returns the sum of a and b.
func Add(a, b int) int {
	return a + b
}

// ID returns a new random identifier. It exists so the module has a real
// dependency, which gives it a go.sum.
func ID() string {
	return uuid.NewString()
}
```

`testdata/fixture-lib/fixture_test.go`:

```go
package fixture

import "testing"

func TestAdd(t *testing.T) {
	if got := Add(2, 3); got != 5 {
		t.Fatalf("Add(2, 3) = %d, want 5", got)
	}
}

func TestID(t *testing.T) {
	if ID() == "" {
		t.Fatal("ID() returned an empty string")
	}
}
```

Generate `go.sum` — do not hand-write it:

```bash
cd /Users/rexraphael/Work/xraph/go-workflows/testdata/fixture-lib && go mod tidy
```

- [ ] **Step 3: Write the partial-Makefile fixture**

`testdata/fixture-partial-make/go.mod`:

```
module github.com/xraph/go-workflows/testdata/fixture-partial-make

go 1.24

require github.com/google/uuid v1.6.0
```

`testdata/fixture-partial-make/fixture.go`:

```go
// Package fixture is a minimal module used to smoke-test the reusable workflows.
package fixture

import "github.com/google/uuid"

// Add returns the sum of a and b.
func Add(a, b int) int {
	return a + b
}

// ID returns a new random identifier. It exists so the module has a real
// dependency, which gives it a go.sum.
func ID() string {
	return uuid.NewString()
}
```

`testdata/fixture-partial-make/fixture_test.go`:

```go
package fixture

import "testing"

func TestAdd(t *testing.T) {
	if got := Add(2, 3); got != 5 {
		t.Fatalf("Add(2, 3) = %d, want 5", got)
	}
}

func TestID(t *testing.T) {
	if ID() == "" {
		t.Fatal("ID() returned an empty string")
	}
}
```

Then generate its `go.sum`:

```bash
cd /Users/rexraphael/Work/xraph/go-workflows/testdata/fixture-partial-make && go mod tidy
```

`testdata/fixture-partial-make/Makefile` — defines `test` and `vet` but deliberately **not** `test-coverage`, which is the case the per-target probe exists to survive:

```makefile
.PHONY: test vet

test:
	@echo "MAKE-TARGET-TEST"
	go test ./...

vet:
	@echo "MAKE-TARGET-VET"
	go vet ./...
```

- [ ] **Step 4: Verify both fixtures build and test green locally**

```bash
cd /Users/rexraphael/Work/xraph/go-workflows/testdata/fixture-lib && go test ./...
cd /Users/rexraphael/Work/xraph/go-workflows/testdata/fixture-partial-make && go test ./... && make test && ! make -n test-coverage
```

Expected: both `go test` runs print `ok`. `make test` prints `MAKE-TARGET-TEST`. The final `! make -n test-coverage` succeeds *because* `make` fails — that is the missing-target condition Task 2's probe must handle.

- [ ] **Step 5: Write `.gitignore`**

```
coverage.out
coverage/
dist/
RELEASE_NOTES.md
```

- [ ] **Step 6: Write `self-test.yml` with the actionlint job only**

Smoke jobs are added in Task 2, once there is a workflow to smoke.

```yaml
name: Self Test

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  actionlint:
    name: Lint workflows
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v7

      - name: Set up Go
        uses: actions/setup-go@v7
        with:
          go-version: '1.26'
          cache: false

      - name: Install actionlint
        run: go install github.com/rhysd/actionlint/cmd/actionlint@v1.7.12

      - name: Run actionlint
        run: actionlint -color

  fixtures:
    name: Fixtures build
    runs-on: ubuntu-latest
    strategy:
      matrix:
        fixture: [fixture-lib, fixture-partial-make]
    steps:
      - name: Checkout
        uses: actions/checkout@v7

      - name: Set up Go
        uses: actions/setup-go@v7
        with:
          go-version: '1.26'
          cache-dependency-path: testdata/${{ matrix.fixture }}/go.mod

      - name: Test fixture
        working-directory: testdata/${{ matrix.fixture }}
        run: go test ./...
```

- [ ] **Step 7: Write the README skeleton**

```markdown
# xraph/go-workflows

Reusable GitHub Actions workflows for xraph Go repositories.

Consumers pin to the moving major tag:

```yaml
jobs:
  ci:
    uses: xraph/go-workflows/.github/workflows/go-ci.yml@v1
    secrets: inherit
```

## Workflows

| workflow | purpose |
|---|---|
| `go-ci.yml` | test matrix, lint, verify, security |
| `go-release.yml` | tag-push and manual-dispatch releases |
| `go-binary-release.yml` | GoReleaser cross-platform binaries |
| `codeql.yml` | CodeQL analysis for Go |

## Pinned tool versions

| tool | version |
|---|---|
| golangci-lint | v2.12.2 |
| gosec | v2.28.0 |
| govulncheck | v1.6.0 |
| goreleaser | v2.17.1 |
| actionlint | v1.7.12 |

Bumps are deliberate commits to this repository. See `CHANGELOG.md`.
```

- [ ] **Step 8: Install actionlint locally and run it**

```bash
go install github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
cd /Users/rexraphael/Work/xraph/go-workflows && "$(go env GOPATH)/bin/actionlint" -color
```

Expected: no output, exit 0. If `actionlint` reports `self-test.yml` errors, fix them before committing.

- [ ] **Step 9: Commit**

```bash
cd /Users/rexraphael/Work/xraph/go-workflows
git add .
git commit -m "chore: bootstrap go-workflows with smoke fixtures and actionlint"
```

---

## Task 2: `go-ci.yml`

**Files:**
- Create: `/Users/rexraphael/Work/xraph/go-workflows/.github/workflows/go-ci.yml`
- Modify: `/Users/rexraphael/Work/xraph/go-workflows/.github/workflows/self-test.yml` (add two smoke jobs)
- Create: `/Users/rexraphael/Work/xraph/go-workflows/examples/library-caller/ci.yml`

**Interfaces:**
- Consumes: `testdata/fixture-lib` and `testdata/fixture-partial-make` from Task 1.
- Produces: `go-ci.yml` with inputs `go-versions`, `primary-go-version`, `os`, `working-directory`, `golangci-lint-version`, `gosec-version`, `govulncheck-version`, `coverage`, `codecov-flag-name`, `skip-security`; optional secret `CODECOV_TOKEN`. Task 4 calls it from `dtl`.

- [ ] **Step 1: Write `go-ci.yml`**

```yaml
name: Go CI

on:
  workflow_call:
    inputs:
      go-versions:
        description: 'JSON array of Go versions for the test matrix'
        type: string
        default: '["1.24","1.25"]'
      primary-go-version:
        description: 'Go version used for lint, coverage and security. Empty means the last entry of go-versions.'
        type: string
        default: ''
      os:
        description: 'JSON array of runner labels for the test matrix'
        type: string
        default: '["ubuntu-latest","macos-latest","windows-latest"]'
      working-directory:
        description: 'Directory containing go.mod'
        type: string
        default: '.'
      golangci-lint-version:
        type: string
        default: 'v2.12.2'
      gosec-version:
        type: string
        default: 'v2.28.0'
      govulncheck-version:
        type: string
        default: 'v1.6.0'
      coverage:
        description: 'Produce and upload a coverage report from the primary combination'
        type: boolean
        default: true
      codecov-flag-name:
        description: 'Codecov flag name. Empty means <repo-name>-coverage.'
        type: string
        default: ''
      skip-security:
        type: boolean
        default: false
    secrets:
      CODECOV_TOKEN:
        required: false

jobs:
  setup:
    name: Resolve config
    runs-on: ubuntu-latest
    outputs:
      primary-go-version: ${{ steps.resolve.outputs.primary }}
      codecov-flag: ${{ steps.resolve.outputs.flag }}
    steps:
      - name: Resolve primary Go version and Codecov flag
        id: resolve
        shell: bash
        env:
          PRIMARY_IN: ${{ inputs.primary-go-version }}
          VERSIONS_IN: ${{ inputs.go-versions }}
          FLAG_IN: ${{ inputs.codecov-flag-name }}
          REPO_NAME: ${{ github.event.repository.name }}
        run: |
          set -euo pipefail
          if [ -n "$PRIMARY_IN" ]; then
            PRIMARY="$PRIMARY_IN"
          else
            PRIMARY=$(printf '%s' "$VERSIONS_IN" | jq -r '.[-1]')
          fi
          FLAG="$FLAG_IN"
          if [ -z "$FLAG" ]; then
            FLAG="${REPO_NAME}-coverage"
          fi
          echo "primary=$PRIMARY" >> "$GITHUB_OUTPUT"
          echo "flag=$FLAG" >> "$GITHUB_OUTPUT"
          {
            echo "### CI configuration"
            echo "- primary Go version: \`$PRIMARY\`"
            echo "- Codecov flag: \`$FLAG\`"
            echo "- working directory: \`${{ inputs.working-directory }}\`"
          } >> "$GITHUB_STEP_SUMMARY"

  test:
    name: Test (${{ matrix.os }}, go${{ matrix.go-version }})
    needs: setup
    runs-on: ${{ matrix.os }}
    defaults:
      run:
        shell: bash
        working-directory: ${{ inputs.working-directory }}
    strategy:
      fail-fast: false
      matrix:
        os: ${{ fromJSON(inputs.os) }}
        go-version: ${{ fromJSON(inputs.go-versions) }}
    steps:
      - name: Checkout
        uses: actions/checkout@v7

      - name: Set up Go
        uses: actions/setup-go@v7
        with:
          go-version: ${{ matrix.go-version }}
          cache-dependency-path: ${{ inputs.working-directory }}/go.mod

      - name: Download dependencies
        run: go mod download

      - name: Resolve test command
        id: cmd
        run: |
          set -euo pipefail
          if [ -f Makefile ] && make -n test >/dev/null 2>&1; then
            echo 'run=make test' >> "$GITHUB_OUTPUT"
            echo 'source=Makefile target `test`' >> "$GITHUB_OUTPUT"
          else
            echo 'run=go test -race ./...' >> "$GITHUB_OUTPUT"
            echo 'source=fallback `go test -race ./...`' >> "$GITHUB_OUTPUT"
          fi

      - name: Report resolved test command
        run: |
          echo "- \`${{ matrix.os }}\` go${{ matrix.go-version }} tests: ${{ steps.cmd.outputs.source }}" >> "$GITHUB_STEP_SUMMARY"

      - name: Run tests
        run: ${{ steps.cmd.outputs.run }}

      - name: Resolve coverage command
        id: covcmd
        if: inputs.coverage && matrix.os == 'ubuntu-latest' && matrix.go-version == needs.setup.outputs.primary-go-version
        run: |
          set -euo pipefail
          if [ -f Makefile ] && make -n test-coverage >/dev/null 2>&1; then
            echo 'run=make test-coverage' >> "$GITHUB_OUTPUT"
            echo 'source=Makefile target `test-coverage`' >> "$GITHUB_OUTPUT"
          else
            echo 'run=go test -race -covermode=atomic -coverprofile=coverage.out ./...' >> "$GITHUB_OUTPUT"
            echo 'source=fallback `go test -coverprofile`' >> "$GITHUB_OUTPUT"
          fi

      - name: Run coverage
        if: steps.covcmd.outcome == 'success'
        run: |
          echo "- coverage: ${{ steps.covcmd.outputs.source }}" >> "$GITHUB_STEP_SUMMARY"
          ${{ steps.covcmd.outputs.run }}

      - name: Locate coverage profile
        id: covfile
        if: steps.covcmd.outcome == 'success'
        run: |
          set -euo pipefail
          if [ -f coverage.out ]; then
            echo "path=${{ inputs.working-directory }}/coverage.out" >> "$GITHUB_OUTPUT"
          elif [ -f coverage/coverage.out ]; then
            echo "path=${{ inputs.working-directory }}/coverage/coverage.out" >> "$GITHUB_OUTPUT"
          else
            echo "path=" >> "$GITHUB_OUTPUT"
            echo "No coverage profile produced; skipping upload." >> "$GITHUB_STEP_SUMMARY"
          fi

      - name: Check for Codecov token
        id: codecov
        if: steps.covfile.outputs.path != ''
        env:
          CODECOV_TOKEN: ${{ secrets.CODECOV_TOKEN }}
        run: |
          set -euo pipefail
          if [ -n "${CODECOV_TOKEN:-}" ]; then
            echo 'present=true' >> "$GITHUB_OUTPUT"
          else
            echo 'present=false' >> "$GITHUB_OUTPUT"
            echo "CODECOV_TOKEN not set; skipping coverage upload." >> "$GITHUB_STEP_SUMMARY"
          fi

      - name: Upload coverage to Codecov
        if: steps.codecov.outputs.present == 'true'
        uses: codecov/codecov-action@v7
        with:
          token: ${{ secrets.CODECOV_TOKEN }}
          files: ${{ steps.covfile.outputs.path }}
          flags: unittests
          name: ${{ needs.setup.outputs.codecov-flag }}
          fail_ci_if_error: false

  lint:
    name: Lint
    needs: setup
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v7

      - name: Set up Go
        uses: actions/setup-go@v7
        with:
          go-version: ${{ needs.setup.outputs.primary-go-version }}
          cache-dependency-path: ${{ inputs.working-directory }}/go.mod

      - name: Run golangci-lint
        uses: golangci/golangci-lint-action@v9
        with:
          version: ${{ inputs.golangci-lint-version }}
          working-directory: ${{ inputs.working-directory }}
          args: --timeout=5m

  verify:
    name: Verify
    needs: setup
    runs-on: ubuntu-latest
    defaults:
      run:
        shell: bash
        working-directory: ${{ inputs.working-directory }}
    steps:
      - name: Checkout
        uses: actions/checkout@v7

      - name: Set up Go
        uses: actions/setup-go@v7
        with:
          go-version: ${{ needs.setup.outputs.primary-go-version }}
          cache-dependency-path: ${{ inputs.working-directory }}/go.mod

      - name: Check formatting
        run: |
          set -euo pipefail
          UNFORMATTED=$(gofmt -l .)
          if [ -n "$UNFORMATTED" ]; then
            echo "These files are not gofmt-clean:"
            echo "$UNFORMATTED"
            exit 1
          fi

      - name: Resolve vet command
        id: vetcmd
        run: |
          set -euo pipefail
          if [ -f Makefile ] && make -n vet >/dev/null 2>&1; then
            echo 'run=make vet' >> "$GITHUB_OUTPUT"
            echo 'source=Makefile target `vet`' >> "$GITHUB_OUTPUT"
          else
            echo 'run=go vet ./...' >> "$GITHUB_OUTPUT"
            echo 'source=fallback `go vet ./...`' >> "$GITHUB_OUTPUT"
          fi

      - name: Run vet
        run: |
          echo "- vet: ${{ steps.vetcmd.outputs.source }}" >> "$GITHUB_STEP_SUMMARY"
          ${{ steps.vetcmd.outputs.run }}

      - name: Check go.mod tidiness
        run: |
          set -euo pipefail
          go mod tidy
          git diff --exit-code go.mod go.sum

  security:
    name: Security
    needs: setup
    if: ${{ !inputs.skip-security }}
    runs-on: ubuntu-latest
    defaults:
      run:
        shell: bash
        working-directory: ${{ inputs.working-directory }}
    steps:
      - name: Checkout
        uses: actions/checkout@v7

      - name: Set up Go
        uses: actions/setup-go@v7
        with:
          go-version: ${{ needs.setup.outputs.primary-go-version }}
          cache-dependency-path: ${{ inputs.working-directory }}/go.mod

      - name: Run gosec
        run: |
          set -euo pipefail
          go install "github.com/securego/gosec/v2/cmd/gosec@${{ inputs.gosec-version }}"
          gosec -exclude=G115 -exclude-dir=vendor ./...

      - name: Run govulncheck
        run: |
          set -euo pipefail
          go install "golang.org/x/vuln/cmd/govulncheck@${{ inputs.govulncheck-version }}"
          govulncheck ./...
```

**Why the probe is written this way.** `make -n test` exits non-zero both when there is no Makefile rule and when `make` itself is absent — which is the case on Windows runners. Both conditions collapse to the raw-`go` fallback, which is the desired behavior, so no OS special-casing is needed.

- [ ] **Step 2: Run actionlint, expect it to pass**

```bash
cd /Users/rexraphael/Work/xraph/go-workflows && "$(go env GOPATH)/bin/actionlint" -color
```

Expected: exit 0, no output. Common failures to fix if reported: `fromJSON` used on a non-string input, a `needs.` reference to a job not listed in `needs:`, or `shell: bash` missing on a step that uses bash syntax on Windows.

- [ ] **Step 3: Add the two smoke jobs to `self-test.yml`**

Append to the `jobs:` block of `.github/workflows/self-test.yml`:

```yaml
  smoke-fallback:
    name: Smoke — no Makefile
    uses: ./.github/workflows/go-ci.yml
    with:
      working-directory: testdata/fixture-lib
      go-versions: '["1.26"]'
      os: '["ubuntu-latest","windows-latest"]'
      coverage: true
      skip-security: true

  smoke-partial-make:
    name: Smoke — Makefile without test-coverage
    uses: ./.github/workflows/go-ci.yml
    with:
      working-directory: testdata/fixture-partial-make
      go-versions: '["1.26"]'
      os: '["ubuntu-latest"]'
      coverage: true
      skip-security: true
```

`skip-security: true` keeps the smoke runs fast; the security job is exercised for real by `dtl` in Task 4. Including `windows-latest` in the first smoke job is deliberate — it is what proves the `make`-absent path.

- [ ] **Step 4: Write `examples/library-caller/ci.yml`**

```yaml
# Copy to .github/workflows/ci.yml in a Go library repository.
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  ci:
    uses: xraph/go-workflows/.github/workflows/go-ci.yml@v1
    with:
      # Match your go.mod directive. Older versions than it will not build.
      go-versions: '["1.24","1.25","1.26"]'
    secrets: inherit
```

- [ ] **Step 5: Run actionlint again, expect it to pass**

```bash
cd /Users/rexraphael/Work/xraph/go-workflows && "$(go env GOPATH)/bin/actionlint" -color
```

Expected: exit 0. `actionlint` validates the local `uses: ./.github/workflows/go-ci.yml` call, including that every `with:` key is a declared input — so a typo in an input name fails here rather than in CI.

- [ ] **Step 6: Commit**

```bash
cd /Users/rexraphael/Work/xraph/go-workflows
git add .github/workflows/go-ci.yml .github/workflows/self-test.yml examples/library-caller/ci.yml
git commit -m "feat: add reusable go-ci workflow with per-target make detection"
```

---

## Task 3: Publish `go-workflows` and cut `v1.0.0`

**REQUIRES CONFIRMATION** — this creates a public repository under the `xraph` org and pushes to it. Ask the user before Step 1 and do not proceed without an explicit yes.

**Files:**
- Create: `/Users/rexraphael/Work/xraph/go-workflows/.github/workflows/release.yml`

**Interfaces:**
- Consumes: everything from Tasks 1–2.
- Produces: the `xraph/go-workflows` remote, tag `v1.0.0`, and the moving tag `v1`. Task 4 depends on `v1` resolving.

- [ ] **Step 1: Write the self-release workflow**

`.github/workflows/release.yml` — retargets the major tag whenever a semver tag is pushed, so consumers on `@v1` pick up fixes.

```yaml
name: Release

on:
  push:
    tags: ['v[0-9]+.[0-9]+.[0-9]+']

permissions:
  contents: write

jobs:
  retag:
    name: Move major tag
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v7
        with:
          fetch-depth: 0

      - name: Retarget major tag
        shell: bash
        run: |
          set -euo pipefail
          MAJOR="${GITHUB_REF_NAME%%.*}"
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          git tag -f "$MAJOR" "$GITHUB_REF_NAME"
          git push -f origin "refs/tags/$MAJOR"
          echo "Moved \`$MAJOR\` to \`$GITHUB_REF_NAME\`" >> "$GITHUB_STEP_SUMMARY"

      - name: Create GitHub release
        uses: ncipollo/release-action@v1
        with:
          token: ${{ secrets.GITHUB_TOKEN }}
          tag: ${{ github.ref_name }}
          name: ${{ github.ref_name }}
          generateReleaseNotes: true
```

- [ ] **Step 2: Run actionlint and commit**

```bash
cd /Users/rexraphael/Work/xraph/go-workflows && "$(go env GOPATH)/bin/actionlint" -color
git add .github/workflows/release.yml
git commit -m "ci: tag releases and retarget the moving major tag"
```

- [ ] **Step 3: Create the remote repository** — CONFIRM FIRST

```bash
gh repo create xraph/go-workflows --public \
  --description "Reusable GitHub Actions workflows for xraph Go repositories" \
  --source /Users/rexraphael/Work/xraph/go-workflows \
  --remote origin \
  --push
```

- [ ] **Step 4: Force the remote to SSH and confirm**

`gh repo create` may set an HTTPS remote, which cannot push workflow files with the current token scopes.

```bash
cd /Users/rexraphael/Work/xraph/go-workflows
git remote set-url origin git@github.com:xraph/go-workflows.git
git remote -v
git push -u origin main
```

Expected: `origin` shows the `git@github.com:` form and the push succeeds.

- [ ] **Step 5: Watch the self-test run and confirm it is green**

```bash
cd /Users/rexraphael/Work/xraph/go-workflows
sleep 10
gh run watch "$(gh run list --workflow=self-test.yml --limit 1 --json databaseId -q '.[0].databaseId')" --exit-status
```

The `sleep` matters: `gh run list` immediately after a push often returns the *previous* run, and `gh run watch` on an already-finished run exits green without watching anything. If the returned run predates the push, wait and re-query.

Expected: every job passes — `actionlint`, `fixtures` (two matrix legs), and the jobs expanded from `smoke-fallback` and `smoke-partial-make` (each smoke call fans out into `setup`, `test`, `lint` and `verify`).

If `smoke-fallback` fails on `windows-latest` at "Run tests", the cause is almost certainly `-race` requiring a C toolchain. Fix by confirming the Windows runner's gcc is on PATH; if it is not, that is a genuine finding — report it rather than silently dropping `-race`.

- [ ] **Step 6: Verify the make-detection paths actually took the branches they should**

```bash
cd /Users/rexraphael/Work/xraph/go-workflows
gh run view "$(gh run list --workflow=self-test.yml --limit 1 --json databaseId -q '.[0].databaseId')" --json jobs -q '.jobs[].name'
```

Then open the run summary in a browser and read the "CI configuration" and resolved-command lines:

```bash
gh run view --web
```

Expected, and this is the actual assertion of Task 2's core behavior:
- `smoke-fallback` / ubuntu → tests: **fallback**, coverage: **fallback**
- `smoke-fallback` / windows → tests: **fallback** (no `make` on the runner)
- `smoke-partial-make` / ubuntu → tests: **Makefile target `test`**, coverage: **fallback**

That last line is the whole point of per-target detection. If it reads "Makefile target `test-coverage`", the probe is wrong.

- [ ] **Step 7: Tag `v1.0.0`**

```bash
cd /Users/rexraphael/Work/xraph/go-workflows
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
sleep 10
gh run watch "$(gh run list --workflow=release.yml --limit 1 --json databaseId -q '.[0].databaseId')" --exit-status
```

- [ ] **Step 8: Confirm the moving `v1` tag resolves**

```bash
cd /Users/rexraphael/Work/xraph/go-workflows
git fetch --tags --force
git rev-parse v1 v1.0.0
```

Expected: both print the same SHA.

---

## Task 4: Wire `dtl` CI to the shared workflow

**REQUIRES CONFIRMATION** — this pushes a branch to `xraph/dtl` and opens a pull request.

**Files:**
- Create: `/Users/rexraphael/Work/xraph/dtl/.github/workflows/ci.yml`

**Interfaces:**
- Consumes: `xraph/go-workflows/.github/workflows/go-ci.yml@v1` from Task 3.
- Produces: a green CI signal on `dtl` pull requests.

- [ ] **Step 1: Confirm dtl's Go floor before choosing the matrix**

```bash
cd /Users/rexraphael/Work/xraph/dtl && head -3 go.mod
```

Expected: `go 1.26.0`. That directive means only `1.26` can build the module, so the matrix is `'["1.26"]'`. If the maintainer has since lowered the floor, widen the matrix to match — never list a version below the `go` directive, because `setup-go` will install it and the build will fail on the directive check.

- [ ] **Step 2: Verify dtl is currently gofmt-clean, vet-clean and tidy**

The shared `verify` job enforces all three. Finding a violation now, locally, is cheaper than a red first CI run.

```bash
cd /Users/rexraphael/Work/xraph/dtl
gofmt -l .
go vet ./...
go mod tidy && git diff --exit-code go.mod go.sum
go test -race ./...
```

Expected: `gofmt -l .` prints nothing; the rest exit 0. Fix anything that fails and commit that fix separately, before adding CI — a formatting fix and a CI addition are two different reviews.

- [ ] **Step 3: Create the branch**

```bash
cd /Users/rexraphael/Work/xraph/dtl
git checkout -b ci/shared-workflows
mkdir -p .github/workflows
```

- [ ] **Step 4: Write `.github/workflows/ci.yml`**

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  ci:
    uses: xraph/go-workflows/.github/workflows/go-ci.yml@v1
    with:
      go-versions: '["1.26"]'
    secrets: inherit
```

- [ ] **Step 5: Lint it locally**

```bash
cd /Users/rexraphael/Work/xraph/dtl && "$(go env GOPATH)/bin/actionlint" -color
```

Expected: exit 0. `actionlint` cannot resolve the remote `@v1` reference, so this only checks local syntax — the real check is Step 7.

- [ ] **Step 6: Commit and push** — CONFIRM FIRST

```bash
cd /Users/rexraphael/Work/xraph/dtl
git add .github/workflows/ci.yml
git commit -m "ci: use shared xraph/go-workflows go-ci workflow"
git push -u origin ci/shared-workflows
```

- [ ] **Step 7: Open the pull request and watch CI** — CONFIRM FIRST

```bash
cd /Users/rexraphael/Work/xraph/dtl
gh pr create --fill --title "ci: adopt shared go-workflows" \
  --body "Adds CI via the new xraph/go-workflows reusable workflow. First consumer of the shared contract."
gh pr checks --watch
```

Expected: `setup`, `test (ubuntu-latest, go1.26)`, `test (macos-latest, go1.26)`, `test (windows-latest, go1.26)`, `lint`, `verify`, `security` all pass.

`dtl` has no `.golangci.yml`; golangci-lint v2 runs with its default linter set. If `lint` fails on defaults, do **not** loosen the shared workflow — add a `.golangci.yml` to `dtl`, which is where project-specific lint policy belongs.

- [ ] **Step 8: Merge**

```bash
cd /Users/rexraphael/Work/xraph/dtl
gh pr merge --squash --delete-branch
git checkout main && git pull
```

---

## Task 5: `go-release.yml` and `codeql.yml`

**Files:**
- Create: `/Users/rexraphael/Work/xraph/go-workflows/.github/workflows/go-release.yml`
- Create: `/Users/rexraphael/Work/xraph/go-workflows/.github/workflows/codeql.yml`
- Create: `/Users/rexraphael/Work/xraph/go-workflows/examples/library-caller/release.yml`
- Create: `/Users/rexraphael/Work/xraph/go-workflows/examples/library-caller/codeql.yml`
- Modify: `/Users/rexraphael/Work/xraph/go-workflows/README.md`

**Interfaces:**
- Consumes: nothing from Task 4; can be written in parallel with it.
- Produces: `go-release.yml` with inputs `version`, `go-version`, `module-path`, `submodules`, `doc-links`, `run-tests`, `update-changelog`, `golangci-lint-version`, `working-directory`; and `codeql.yml` with inputs `go-version`, `working-directory`, `schedule-cron`. Task 6 calls both from `dtl`.

- [ ] **Step 1: Write `go-release.yml`**

Release notes are assembled into a file and handed to `ncipollo/release-action` via `bodyFile`, rather than built with inline YAML expressions. That is what makes `module-path` derivable instead of hand-typed.

```yaml
name: Go Release

on:
  workflow_call:
    inputs:
      version:
        description: 'Version to release, e.g. v1.0.0. Empty means derive from the pushed tag.'
        type: string
        default: ''
      go-version:
        type: string
        default: '1.25'
      module-path:
        description: 'Go module path. Empty means github.com/<owner>/<repo>.'
        type: string
        default: ''
      submodules:
        description: 'JSON array of nested module suffixes, e.g. ["errs","log"]'
        type: string
        default: '[]'
      doc-links:
        description: 'JSON array of repo-relative doc paths, e.g. ["errs/README.md"]'
        type: string
        default: '[]'
      run-tests:
        type: boolean
        default: true
      update-changelog:
        description: 'Prepend a section to CHANGELOG.md. Dispatch path only.'
        type: boolean
        default: true
      golangci-lint-version:
        type: string
        default: 'v2.12.2'
      working-directory:
        type: string
        default: '.'

jobs:
  release:
    name: Release
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - name: Checkout
        uses: actions/checkout@v7
        with:
          fetch-depth: 0

      - name: Resolve mode and version
        id: ver
        shell: bash
        env:
          VERSION_IN: ${{ inputs.version }}
        run: |
          set -euo pipefail
          if [ -n "$VERSION_IN" ]; then
            MODE=dispatch
            VERSION="$VERSION_IN"
          else
            MODE=tag
            VERSION="${GITHUB_REF_NAME}"
          fi
          echo "mode=$MODE" >> "$GITHUB_OUTPUT"
          echo "version=$VERSION" >> "$GITHUB_OUTPUT"
          if printf '%s' "$VERSION" | grep -q -- '-'; then
            echo 'prerelease=true' >> "$GITHUB_OUTPUT"
          else
            echo 'prerelease=false' >> "$GITHUB_OUTPUT"
          fi
          echo "Releasing \`$VERSION\` via **$MODE**" >> "$GITHUB_STEP_SUMMARY"

      - name: Validate version format
        if: steps.ver.outputs.mode == 'dispatch'
        shell: bash
        env:
          VERSION: ${{ steps.ver.outputs.version }}
        run: |
          set -euo pipefail
          if ! printf '%s' "$VERSION" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$'; then
            echo "Version must look like v1.2.3 or v1.2.3-beta.1, got: $VERSION"
            exit 1
          fi

      - name: Ensure tag does not already exist
        if: steps.ver.outputs.mode == 'dispatch'
        shell: bash
        env:
          VERSION: ${{ steps.ver.outputs.version }}
        run: |
          set -euo pipefail
          if git rev-parse "$VERSION" >/dev/null 2>&1; then
            echo "Tag $VERSION already exists."
            exit 1
          fi

      - name: Ensure working tree is clean
        if: steps.ver.outputs.mode == 'dispatch'
        shell: bash
        run: |
          set -euo pipefail
          if [ -n "$(git status --porcelain)" ]; then
            echo "Working tree is not clean:"
            git status --short
            exit 1
          fi

      - name: Record previous tag
        id: prev
        shell: bash
        env:
          MODE: ${{ steps.ver.outputs.mode }}
        run: |
          set -euo pipefail
          if [ "$MODE" = 'tag' ]; then
            PREV=$(git describe --tags --abbrev=0 HEAD^ 2>/dev/null || echo '')
          else
            PREV=$(git describe --tags --abbrev=0 2>/dev/null || echo '')
          fi
          echo "tag=$PREV" >> "$GITHUB_OUTPUT"

      - name: Set up Go
        if: inputs.run-tests
        uses: actions/setup-go@v7
        with:
          go-version: ${{ inputs.go-version }}
          cache-dependency-path: ${{ inputs.working-directory }}/go.mod

      - name: Resolve test command
        id: cmd
        if: inputs.run-tests
        shell: bash
        working-directory: ${{ inputs.working-directory }}
        run: |
          set -euo pipefail
          if [ -f Makefile ] && make -n test >/dev/null 2>&1; then
            echo 'run=make test' >> "$GITHUB_OUTPUT"
          else
            echo 'run=go test -race ./...' >> "$GITHUB_OUTPUT"
          fi

      - name: Run tests
        if: inputs.run-tests
        shell: bash
        working-directory: ${{ inputs.working-directory }}
        env:
          TEST_CMD: ${{ steps.cmd.outputs.run }}
        run: |
          set -euo pipefail
          eval "$TEST_CMD"

      - name: Run golangci-lint
        if: inputs.run-tests
        uses: golangci/golangci-lint-action@v9
        with:
          version: ${{ inputs.golangci-lint-version }}
          working-directory: ${{ inputs.working-directory }}
          args: --timeout=5m

      - name: Build changelog body
        id: changelog
        shell: bash
        env:
          PREV: ${{ steps.prev.outputs.tag }}
        run: |
          set -euo pipefail
          if [ -z "$PREV" ]; then
            BODY=$(git log --oneline --decorate)
          else
            BODY=$(git log "${PREV}..HEAD" --oneline --decorate)
          fi
          DELIM="GHEOF_$(openssl rand -hex 8)"
          {
            echo "body<<$DELIM"
            printf '%s\n' "$BODY"
            echo "$DELIM"
          } >> "$GITHUB_OUTPUT"

      - name: Update CHANGELOG.md
        if: steps.ver.outputs.mode == 'dispatch' && inputs.update-changelog
        shell: bash
        env:
          VERSION: ${{ steps.ver.outputs.version }}
          PREV: ${{ steps.prev.outputs.tag }}
        run: |
          set -euo pipefail
          DATE=$(date +%Y-%m-%d)
          {
            echo '# Changelog'
            echo
            echo 'All notable changes to this project are documented in this file.'
            echo
            echo "## [$VERSION] - $DATE"
            echo
            if [ -z "$PREV" ]; then
              echo '### Initial release'
              echo
              git log --oneline --decorate | sed 's/^/- /'
            else
              echo "### Changes since $PREV"
              echo
              git log "${PREV}..HEAD" --oneline --decorate | sed 's/^/- /'
            fi
            echo
            if [ -f CHANGELOG.md ]; then
              tail -n +2 CHANGELOG.md
            fi
          } > CHANGELOG.md.new
          mv CHANGELOG.md.new CHANGELOG.md

      - name: Commit, tag and push
        if: steps.ver.outputs.mode == 'dispatch'
        shell: bash
        env:
          VERSION: ${{ steps.ver.outputs.version }}
        run: |
          set -euo pipefail
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          if [ -n "$(git status --porcelain)" ]; then
            git add CHANGELOG.md
            git commit -m "chore: update CHANGELOG.md for $VERSION"
            git push origin HEAD:"${GITHUB_REF_NAME}"
          fi
          git tag -a "$VERSION" -m "Release $VERSION"
          git push origin "$VERSION"

      - name: Assemble release notes
        id: notes
        shell: bash
        env:
          VERSION: ${{ steps.ver.outputs.version }}
          MODULE_IN: ${{ inputs.module-path }}
          SUBMODULES: ${{ inputs.submodules }}
          DOC_LINKS: ${{ inputs.doc-links }}
          CHANGELOG_BODY: ${{ steps.changelog.outputs.body }}
        run: |
          set -euo pipefail
          MODULE="$MODULE_IN"
          if [ -z "$MODULE" ]; then
            MODULE="github.com/${GITHUB_REPOSITORY}"
          fi
          REPO_URL="${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}"
          {
            echo "# Release $VERSION"
            echo
            echo '## Installation'
            echo
            echo '```bash'
            echo "go get ${MODULE}@${VERSION}"
            printf '%s' "$SUBMODULES" | jq -r --arg m "$MODULE" --arg v "$VERSION" \
              '.[] | "go get \($m)/\(.)@\($v)"'
            echo '```'
            echo
            echo '## Changes'
            echo
            echo '```'
            printf '%s\n' "$CHANGELOG_BODY"
            echo '```'
            echo
            echo '## Documentation'
            echo
            echo "- [README](${REPO_URL}/blob/${VERSION}/README.md)"
            printf '%s' "$DOC_LINKS" | jq -r --arg u "$REPO_URL" --arg v "$VERSION" \
              '.[] | "- [\(.)](\($u)/blob/\($v)/\(.))"'
            echo "- [pkg.go.dev](https://pkg.go.dev/${MODULE}@${VERSION})"
          } > RELEASE_NOTES.md
          echo "module=$MODULE" >> "$GITHUB_OUTPUT"
          cat RELEASE_NOTES.md >> "$GITHUB_STEP_SUMMARY"

      - name: Create GitHub release
        uses: ncipollo/release-action@v1
        with:
          token: ${{ secrets.GITHUB_TOKEN }}
          tag: ${{ steps.ver.outputs.version }}
          name: Release ${{ steps.ver.outputs.version }}
          bodyFile: RELEASE_NOTES.md
          draft: false
          prerelease: ${{ steps.ver.outputs.prerelease == 'true' }}

      - name: Warm the Go module proxy
        shell: bash
        env:
          VERSION: ${{ steps.ver.outputs.version }}
          MODULE: ${{ steps.notes.outputs.module }}
          SUBMODULES: ${{ inputs.submodules }}
        run: |
          set -euo pipefail
          curl -sSf "https://proxy.golang.org/${MODULE}/@v/${VERSION}.info" || true
          for sub in $(printf '%s' "$SUBMODULES" | jq -r '.[]'); do
            curl -sSf "https://proxy.golang.org/${MODULE}/${sub}/@v/${VERSION}.info" || true
          done
```

- [ ] **Step 2: Write `codeql.yml`**

```yaml
name: CodeQL

on:
  workflow_call:
    inputs:
      go-version:
        type: string
        default: '1.25'
      working-directory:
        type: string
        default: '.'

jobs:
  analyze:
    name: Analyze
    runs-on: ubuntu-latest
    permissions:
      actions: read
      contents: read
      security-events: write
    steps:
      - name: Checkout
        uses: actions/checkout@v7

      - name: Set up Go
        uses: actions/setup-go@v7
        with:
          go-version: ${{ inputs.go-version }}
          cache-dependency-path: ${{ inputs.working-directory }}/go.mod

      - name: Initialize CodeQL
        uses: github/codeql-action/init@v4
        with:
          languages: go

      - name: Autobuild
        uses: github/codeql-action/autobuild@v4

      - name: Perform CodeQL analysis
        uses: github/codeql-action/analyze@v4
        with:
          category: "/language:go"
```

Scheduling lives in the caller, not here — `workflow_call` has no `schedule` trigger, so a cron can only be declared by the calling workflow. That is why the example caller in Step 3 carries the weekly `cron` block rather than this file.

`codeql.yml` declares no inputs beyond `go-version` and `working-directory`. Resist adding a documentation-only input that no step reads; it reads as configuration and silently does nothing.

- [ ] **Step 3: Write the example callers**

`examples/library-caller/release.yml`:

```yaml
# Copy to .github/workflows/release.yml in a Go library repository.
name: Release

on:
  push:
    tags: ['v*']
  workflow_dispatch:
    inputs:
      version:
        description: 'Version to release (e.g. v1.0.0)'
        required: true
        type: string

# Required: reusable workflows do not grant themselves permissions.
permissions:
  contents: write

jobs:
  release:
    uses: xraph/go-workflows/.github/workflows/go-release.yml@v1
    with:
      version: ${{ inputs.version }}
      go-version: '1.26'
      # For repos with nested modules:
      # submodules: '["errs","log"]'
      # doc-links: '["errs/README.md"]'
    secrets: inherit
```

`examples/library-caller/codeql.yml`:

```yaml
# Copy to .github/workflows/codeql.yml in a Go repository.
name: CodeQL

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
  schedule:
    - cron: '0 0 * * 1'

jobs:
  analyze:
    uses: xraph/go-workflows/.github/workflows/codeql.yml@v1
    with:
      go-version: '1.26'
```

- [ ] **Step 4: Document inputs and the tag-recovery procedure in the README**

Append to `README.md`:

```markdown
## Recovering a failed dispatch release

On the `workflow_dispatch` path the tag is pushed before the GitHub release is
created. Tests and lint run first, so the common failures happen before anything
is mutated — but if release creation itself fails, the tag is already public.

To retry:

```bash
git push --delete origin v1.2.3
git fetch --prune --prune-tags
```

Then re-run the dispatch. A GitHub release requires its tag to exist, so
release-then-tag is not possible; this is a known trade-off.

## Caller permissions

`go-release.yml` needs `contents: write` **declared by the caller**. A reusable
workflow cannot grant itself permissions. Omitting it produces a failure at
release-creation time, not at parse time.
```

- [ ] **Step 5: Run actionlint, expect it to pass**

```bash
cd /Users/rexraphael/Work/xraph/go-workflows && "$(go env GOPATH)/bin/actionlint" -color
```

Expected: exit 0.

- [ ] **Step 6: Commit, push, and confirm self-test is still green**

```bash
cd /Users/rexraphael/Work/xraph/go-workflows
git add .
git commit -m "feat: add reusable go-release and codeql workflows"
git push
sleep 10
gh run watch "$(gh run list --workflow=self-test.yml --limit 1 --json databaseId -q '.[0].databaseId')" --exit-status
```

- [ ] **Step 7: Tag `v1.1.0` and confirm `v1` moved**

```bash
cd /Users/rexraphael/Work/xraph/go-workflows
git tag -a v1.1.0 -m "Release v1.1.0"
git push origin v1.1.0
sleep 10
gh run watch "$(gh run list --workflow=release.yml --limit 1 --json databaseId -q '.[0].databaseId')" --exit-status
git fetch --tags --force
git rev-parse v1 v1.1.0
```

Expected: both SHAs match.

---

## Task 6: Wire `dtl` release and CodeQL, then cut `v0.1.0`

**REQUIRES CONFIRMATION** — Step 7 publishes a real, public release of `dtl`. Confirm separately from the earlier steps.

**Files:**
- Create: `/Users/rexraphael/Work/xraph/dtl/.github/workflows/release.yml`
- Create: `/Users/rexraphael/Work/xraph/dtl/.github/workflows/codeql.yml`

**Interfaces:**
- Consumes: `go-release.yml@v1` and `codeql.yml@v1` from Task 5.
- Produces: `dtl v0.1.0` on GitHub and pkg.go.dev.

- [ ] **Step 1: Branch**

```bash
cd /Users/rexraphael/Work/xraph/dtl
git checkout main && git pull
git checkout -b ci/release-workflows
```

- [ ] **Step 2: Write `.github/workflows/release.yml`**

`dtl` is a single module with no nested modules, so `submodules` and `doc-links` are left at their defaults.

```yaml
name: Release

on:
  push:
    tags: ['v*']
  workflow_dispatch:
    inputs:
      version:
        description: 'Version to release (e.g. v0.1.0)'
        required: true
        type: string

permissions:
  contents: write

jobs:
  release:
    uses: xraph/go-workflows/.github/workflows/go-release.yml@v1
    with:
      version: ${{ inputs.version }}
      go-version: '1.26'
    secrets: inherit
```

- [ ] **Step 3: Write `.github/workflows/codeql.yml`**

```yaml
name: CodeQL

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
  schedule:
    - cron: '0 0 * * 1'

jobs:
  analyze:
    uses: xraph/go-workflows/.github/workflows/codeql.yml@v1
    with:
      go-version: '1.26'
```

- [ ] **Step 4: Lint locally**

```bash
cd /Users/rexraphael/Work/xraph/dtl && "$(go env GOPATH)/bin/actionlint" -color
```

Expected: exit 0.

- [ ] **Step 5: Commit, push, open the PR** — CONFIRM FIRST

```bash
cd /Users/rexraphael/Work/xraph/dtl
git add .github/workflows/release.yml .github/workflows/codeql.yml
git commit -m "ci: add shared release and CodeQL workflows"
git push -u origin ci/release-workflows
gh pr create --fill --title "ci: add release and CodeQL workflows"
gh pr checks --watch
```

Expected: CI plus the new CodeQL job pass. The release workflow does not run on a PR — it has no `pull_request` trigger.

- [ ] **Step 6: Merge**

```bash
cd /Users/rexraphael/Work/xraph/dtl
gh pr merge --squash --delete-branch
git checkout main && git pull
```

- [ ] **Step 7: Cut `v0.1.0` through the dispatch path** — CONFIRM FIRST

This publishes a public release. Ask explicitly before running.

```bash
cd /Users/rexraphael/Work/xraph/dtl
gh workflow run release.yml -f version=v0.1.0
sleep 10
gh run watch "$(gh run list --workflow=release.yml --limit 1 --json databaseId -q '.[0].databaseId')" --exit-status
```

- [ ] **Step 8: Verify the release end to end**

```bash
cd /Users/rexraphael/Work/xraph/dtl
git fetch --tags && git rev-parse v0.1.0
gh release view v0.1.0
git pull && head -20 CHANGELOG.md
curl -sSf "https://proxy.golang.org/github.com/xraph/dtl/@v/v0.1.0.info"
```

Expected, and this is the assertion that the whole design works:
- the tag exists,
- the release body's install line reads `go get github.com/xraph/dtl@v0.1.0` — **derived**, and therefore correct, unlike `confy`'s hand-typed `vessel`,
- the pkg.go.dev link points at `github.com/xraph/dtl`,
- `CHANGELOG.md` exists on `main` with a `## [v0.1.0]` section,
- the proxy returns JSON containing `"Version":"v0.1.0"`.

If the run fails after the tag was pushed, follow the recovery in the `go-workflows` README: delete the remote tag, then re-dispatch.

---

## Task 7: `go-binary-release.yml`

No consumer is wired in this task. `dtl` is a library; `forge`, `forgeui` and `smart-form` are migrated separately, later.

**Files:**
- Create: `/Users/rexraphael/Work/xraph/go-workflows/.github/workflows/go-binary-release.yml`
- Create: `/Users/rexraphael/Work/xraph/go-workflows/examples/cli-caller/release.yml`
- Modify: `/Users/rexraphael/Work/xraph/go-workflows/README.md`

**Interfaces:**
- Consumes: nothing from Task 6.
- Produces: `go-binary-release.yml` with inputs `go-version`, `goreleaser-version`, `goreleaser-args`, `working-directory`; optional secrets `HOMEBREW_TAP_TOKEN`, `SCOOP_BUCKET_TOKEN`, `GPG_PRIVATE_KEY`.

- [ ] **Step 1: Write `go-binary-release.yml`**

```yaml
name: Go Binary Release

on:
  workflow_call:
    inputs:
      go-version:
        type: string
        default: '1.25'
      goreleaser-version:
        type: string
        default: 'v2.17.1'
      goreleaser-args:
        type: string
        default: 'release --clean'
      working-directory:
        type: string
        default: '.'
    secrets:
      HOMEBREW_TAP_TOKEN:
        required: false
      SCOOP_BUCKET_TOKEN:
        required: false
      GPG_PRIVATE_KEY:
        required: false

jobs:
  goreleaser:
    name: GoReleaser
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - name: Checkout
        uses: actions/checkout@v7
        with:
          fetch-depth: 0

      - name: Set up Go
        uses: actions/setup-go@v7
        with:
          go-version: ${{ inputs.go-version }}
          cache-dependency-path: ${{ inputs.working-directory }}/go.mod

      - name: Confirm a GoReleaser config is present
        shell: bash
        working-directory: ${{ inputs.working-directory }}
        run: |
          set -euo pipefail
          if ! ls .goreleaser.y*ml >/dev/null 2>&1; then
            echo "No .goreleaser.yml found in ${{ inputs.working-directory }}."
            echo "This workflow expects the consumer repository to own its GoReleaser config."
            exit 1
          fi

      - name: Check for a GPG key
        id: gpgcheck
        env:
          GPG_PRIVATE_KEY: ${{ secrets.GPG_PRIVATE_KEY }}
        shell: bash
        run: |
          set -euo pipefail
          if [ -n "${GPG_PRIVATE_KEY:-}" ]; then
            echo 'present=true' >> "$GITHUB_OUTPUT"
          else
            echo 'present=false' >> "$GITHUB_OUTPUT"
            echo "No GPG_PRIVATE_KEY set; artifacts will not be signed." >> "$GITHUB_STEP_SUMMARY"
          fi

      - name: Import GPG key
        id: gpg
        if: steps.gpgcheck.outputs.present == 'true'
        env:
          GPG_PRIVATE_KEY: ${{ secrets.GPG_PRIVATE_KEY }}
        shell: bash
        run: |
          set -euo pipefail
          printf '%s' "$GPG_PRIVATE_KEY" | gpg --batch --import
          echo "fingerprint=$(gpg --list-secret-keys --with-colons | awk -F: '/^fpr:/ {print $10; exit}')" >> "$GITHUB_OUTPUT"

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v7
        with:
          version: ${{ inputs.goreleaser-version }}
          args: ${{ inputs.goreleaser-args }}
          workdir: ${{ inputs.working-directory }}
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}
          SCOOP_BUCKET_TOKEN: ${{ secrets.SCOOP_BUCKET_TOKEN }}
          GPG_FINGERPRINT: ${{ steps.gpg.outputs.fingerprint }}
```

**Why the GPG check is two steps.** A step's own `env:` block is not in scope in that step's `if:` expression — only workflow- and job-level `env` is. Writing `if: env.GPG_PRIVATE_KEY != ''` alongside a step-level `env:` silently evaluates to `'' != ''`, which is false, so the step never runs. The same two-step shape is used for `CODECOV_TOKEN` in `go-ci.yml`.

- [ ] **Step 2: Write `examples/cli-caller/release.yml`**

```yaml
# Copy to .github/workflows/release.yml in a Go CLI repository.
# The repository keeps its own .goreleaser.yml.
name: Release

on:
  push:
    tags: ['v*']

permissions:
  contents: write

jobs:
  binaries:
    uses: xraph/go-workflows/.github/workflows/go-binary-release.yml@v1
    with:
      go-version: '1.26'
    secrets:
      HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}
      SCOOP_BUCKET_TOKEN: ${{ secrets.SCOOP_BUCKET_TOKEN }}
```

- [ ] **Step 3: Document the binary track in the README**

Append to `README.md`:

```markdown
## Binary track

`go-binary-release.yml` wraps GoReleaser. The consumer repository owns its
`.goreleaser.yml` — the config is genuinely per-project (build matrix, ldflags,
Homebrew and Scoop blocks) and is not something this repository should dictate.

Secrets, all optional: `HOMEBREW_TAP_TOKEN`, `SCOOP_BUCKET_TOKEN`,
`GPG_PRIVATE_KEY`. Pass them explicitly rather than with `secrets: inherit` so
the blast radius of a token stays visible in the caller.

No repository consumes this yet. `forge`, `forgeui` and `smart-form` already
carry a `.goreleaser.yml` and are the intended first consumers.
```

- [ ] **Step 4: Run actionlint, expect it to pass**

```bash
cd /Users/rexraphael/Work/xraph/go-workflows && "$(go env GOPATH)/bin/actionlint" -color
```

Expected: exit 0.

- [ ] **Step 5: Commit, push, confirm self-test green**

```bash
cd /Users/rexraphael/Work/xraph/go-workflows
git add .
git commit -m "feat: add reusable go-binary-release workflow for the CLI track"
git push
sleep 10
gh run watch "$(gh run list --workflow=self-test.yml --limit 1 --json databaseId -q '.[0].databaseId')" --exit-status
```

- [ ] **Step 6: Tag `v1.2.0`** — CONFIRM FIRST

```bash
cd /Users/rexraphael/Work/xraph/go-workflows
git tag -a v1.2.0 -m "Release v1.2.0"
git push origin v1.2.0
sleep 10
gh run watch "$(gh run list --workflow=release.yml --limit 1 --json databaseId -q '.[0].databaseId')" --exit-status
git fetch --tags --force && git rev-parse v1 v1.2.0
```

Expected: both SHAs match.

---

## Follow-up, explicitly not in this plan

- Migrating `confy`, `vessel`, `go-utils` and `farp` to the shared workflows, and fixing the `confy`-says-`vessel` bugs in `confy/.github/workflows/release.yml` and `auto-release.yml` in the process.
- Migrating `forge`, `forgeui` and `smart-form` to `go-binary-release.yml`.
- Deciding whether `dtl`'s `go.mod` floor should drop from `1.26.0` to widen the supported Go range.

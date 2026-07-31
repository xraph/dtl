# Shared Go Release Workflows (`xraph/go-workflows`)

Date: 2026-07-30
Status: Approved design, not yet implemented

## Problem

Every xraph Go repository carries its own copy of the same GitHub Actions setup.
`confy`, `vessel`, `go-utils`, and `farp` each hold four files — `ci.yml`,
`release.yml`, `auto-release.yml`, `codeql.yml` — that are byte-identical apart
from the repository name, the Go version, and the list of submodules.
`confy/.github/workflows/ci.yml` and `vessel/.github/workflows/ci.yml` are both
124 lines and differ only in the Go matrix and one Codecov flag name.

The copies have already diverged incorrectly:

- `confy/.github/workflows/release.yml` tells users to run
  `go get github.com/xraph/vessel@<version>` — the wrong repository.
- `confy/.github/workflows/auto-release.yml` links release readers to
  `pkg.go.dev/github.com/xraph/vessel`.

Both are live bugs in `confy`'s published release notes, produced by an
incomplete find-and-replace. A fix applied to one repository does not reach the
others, and today there is no mechanism that would make it.

`dtl` has no `.github/` directory at all. It also has no `Makefile` and no
`.golangci.yml`, while the existing workflows invoke `make test`, `make vet`,
and `make test-coverage`. Copying the existing files into `dtl` would fail on
the first step.

## Goal

A single repository, `xraph/go-workflows`, holding reusable GitHub Actions
workflows (`on: workflow_call`) that cover both Go tracks in the org:

1. **Go library** — CI, release, CodeQL. Consumers: `dtl`, and later `confy`,
   `vessel`, `go-utils`, `farp`.
2. **Go CLI binary** — GoReleaser cross-platform build publishing to GitHub
   Releases plus the existing `homebrew-tap` and `scoop-bucket`. Consumers
   later: `forge`, `forgeui`, `smart-form`, which already carry
   `.goreleaser.yml`.

`dtl` is wired up as the first consumer. No existing repository is migrated in
this effort.

Explicitly out of scope: `octopus` (Rust — crates, Helm, Docker), `game-cli`
(Dart), and `forge`'s npm and VS Code extension publishing. Those are different
toolchains and belong in separate tracks or repositories.

## Approach

GitHub reusable workflows, not composite actions and not a file-sync bot.

- **Composite actions** deduplicate a sequence of steps inside a job the caller
  still has to write. The duplication here is mostly job structure, matrix
  definitions, triggers, and permissions — none of which a composite action can
  carry.
- **A file-sync bot** (repo-file-sync-action and similar) leaves N physical
  copies that drift between syncs. That is the current problem on a timer.
- **Reusable workflows** carry the entire job graph. A consumer repository is
  reduced to a trigger block and a `uses:` line.

## Repository layout

```
xraph/go-workflows
├── .github/workflows/
│   ├── go-ci.yml               # reusable: test matrix, lint, verify, security
│   ├── go-release.yml          # reusable: tag-push + dispatch release
│   ├── go-binary-release.yml   # reusable: GoReleaser + Homebrew/Scoop
│   ├── codeql.yml              # reusable: Go CodeQL analysis
│   ├── self-test.yml           # this repo's own CI: actionlint + smoke call
│   └── release.yml             # tags this repo, retargets the moving v1 tag
├── testdata/
│   ├── fixture-lib/            # minimal Go module, no Makefile — exercises the fallback path
│   │   ├── go.mod
│   │   ├── fixture.go
│   │   └── fixture_test.go
│   └── fixture-partial-make/   # same, plus a Makefile defining `test` but not `test-coverage`
│       ├── Makefile
│       ├── go.mod
│       ├── fixture.go
│       └── fixture_test.go
├── examples/
│   ├── library-caller/         # copy-paste starter: ci.yml, release.yml, codeql.yml
│   └── cli-caller/             # copy-paste starter for the binary track
└── README.md
```

Reusable workflows must live in `.github/workflows/` of the host repository;
subdirectories are not supported by GitHub.

## Versioning and pinning

`go-workflows` is tagged with semver (`v1.0.0`, `v1.1.0`, …). A moving `v1` tag
is retargeted to the newest `v1.x.y` on every release by `release.yml`.

Consumers pin to the moving tag:

```yaml
uses: xraph/go-workflows/.github/workflows/go-ci.yml@v1
```

The mutability of `v1` is deliberate. It is what makes "fix the release-note
bug once and every repository gets it" true, which is the entire point of the
exercise. `go-workflows` is first-party org code, so the supply-chain argument
for SHA-pinning does not apply the way it would to a third-party action.

Third-party actions *inside* the reusable workflows stay pinned to major version
tags (`actions/checkout@v4`, `actions/setup-go@v5`, `ncipollo/release-action@v1`),
matching current practice across the org. Tightening those to SHAs is a separate
decision, not taken here.

## `go-ci.yml`

Four jobs, preserving the structure of the current `ci.yml`:

| job | runs on | does |
|---|---|---|
| `test` | OS × Go matrix | `go mod download`, tests, coverage on the primary combination only |
| `lint` | ubuntu, primary Go | `golangci/golangci-lint-action` |
| `verify` | ubuntu, primary Go | gofmt check, `go vet`, `go mod tidy` leaves no diff |
| `security` | ubuntu, primary Go | `gosec`, `govulncheck` |

### Inputs

All optional. Defaults reproduce today's behavior for the existing repositories.

| input | type | default | notes |
|---|---|---|---|
| `go-versions` | string | `'["1.24","1.25"]'` | JSON array as a string; `workflow_call` inputs cannot be arrays |
| `primary-go-version` | string | last element of `go-versions` | the combination that lints, covers, and scans |
| `os` | string | `'["ubuntu-latest","macos-latest","windows-latest"]'` | JSON array as a string |
| `working-directory` | string | `.` | monorepo subdirectories and the self-test fixture |
| `golangci-lint-version` | string | `v2.6.1` | |
| `gosec-version` | string | see "Pinning tool versions" | |
| `govulncheck-version` | string | see "Pinning tool versions" | |
| `coverage` | boolean | `true` | |
| `codecov-flag-name` | string | `""` | empty resolves to `<repo-name>-coverage` inside the job |
| `skip-security` | boolean | `false` | |

### Secrets

`CODECOV_TOKEN`, declared optional. When unset, the upload step is skipped
rather than failing the job. Callers use `secrets: inherit`.

### Make auto-detection

The existing workflows hardcode `make test`, `make vet`, and `make test-coverage`.
`dtl` has no Makefile; `confy`, `vessel`, `farp`, and `go-utils` do.

Detection is **per-target, not per-file**. Each step probes whether that specific
target resolves and falls back to a raw `go` command otherwise:

```bash
if [ -f Makefile ] && make -n test >/dev/null 2>&1; then
  echo "cmd=make test" >> "$GITHUB_OUTPUT"
else
  echo "cmd=go test -race ./..." >> "$GITHUB_OUTPUT"
fi
```

Fallbacks:

| target | fallback |
|---|---|
| `test` | `go test -race ./...` |
| `test-coverage` | `go test -race -coverprofile=coverage.out -covermode=atomic ./...` |
| `vet` | `go vet ./...` |

A Makefile that defines `test` but not `test-coverage` therefore works. Every job
writes the command it selected, and why, to `$GITHUB_STEP_SUMMARY`, so the
effective command is recoverable from the run page without reading the workflow
source.

### Pinning tool versions

The current workflows install both scanners with `@latest`:

```yaml
go install github.com/securego/gosec/v2/cmd/gosec@latest
go install golang.org/x/vuln/cmd/govulncheck@latest
```

An upstream release can turn every xraph repository's CI red overnight with no
change on our side. Both become inputs with an explicit pinned default.

The rule for choosing every pinned default in this repository — `gosec-version`,
`govulncheck-version`, `goreleaser-version` — is: at implementation time, take
the newest stable release of that tool, verify the smoke test passes against it,
and write the exact version into the workflow's `default:` and into the README's
version table. Bumps are then deliberate commits to `go-workflows`, visible in
its own changelog, rather than silent drift. No exact numbers are fixed in this
document because they will have moved by the time it is implemented.

## `go-release.yml`

Replaces the current `release.yml` + `auto-release.yml` pair with one reusable
workflow accepting both entry points.

The two files exist today because a tag pushed with `GITHUB_TOKEN` does not
trigger other workflows — GitHub's recursion guard. So `auto-release.yml` cannot
delegate to `release.yml` after pushing its tag, and instead repeats the entire
release body. That is the source of the duplication, and of the drift, since
the release-notes template is written out twice per repository.

The caller declares both triggers and passes the dispatch input through:

```yaml
on:
  push:
    tags: ['v*']
  workflow_dispatch:
    inputs:
      version:
        description: 'Version to release (e.g. v1.0.0)'
        required: true
        type: string
```

### Dispatch path

1. Validate the version against `^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$`.
2. Fail if the tag already exists.
3. Fail if the working tree is dirty.
4. Run tests and lint.
5. Prepend a new section to `CHANGELOG.md`, creating the file if absent.
6. Commit as `github-actions[bot]`, create an annotated tag, push both.

### Tag-push path

`VERSION` is `github.ref_name`. Steps 1–3 and 5–6 are skipped; the workflow
enters the shared tail directly.

### Shared tail

1. Generate the changelog body from `git log <prev-tag>..HEAD --oneline --decorate`,
   falling back to full history when no previous tag exists.
2. Create the GitHub release via `ncipollo/release-action@v1`.
3. Warm `proxy.golang.org` for the root module and each submodule.
4. Write a summary to `$GITHUB_STEP_SUMMARY`.

`prerelease` is derived from the version string containing `-`.

### Inputs

| input | type | default | notes |
|---|---|---|---|
| `version` | string | `""` | set on the dispatch path; empty means derive from the pushed tag |
| `go-version` | string | `1.25` | |
| `module-path` | string | `github.com/${{ github.repository }}` | |
| `submodules` | string | `'[]'` | JSON array of path suffixes, e.g. `'["errs","log"]'` |
| `doc-links` | string | `'[]'` | JSON array of repo-relative paths, e.g. `'["errs/README.md"]'`, rendered as extra links in the Documentation section |
| `run-tests` | boolean | `true` | |
| `update-changelog` | boolean | `true` | dispatch path only |
| `golangci-lint-version` | string | `v2.6.1` | |
| `working-directory` | string | `.` | |

### The structural fix

`module-path` defaults to `github.com/${{ github.repository }}`, and the
`go get` lines, documentation links, and proxy-warm URLs are all generated from
it plus `submodules`. Nothing in the release-notes template is hand-typed per
repository, which makes the confy-says-vessel class of bug unrepresentable.

### Accepted sharp edge

On the dispatch path the tag is pushed before the GitHub release is created.
Tests and lint run before the tag is created, so the common failure modes happen
while nothing has been mutated. But if release creation itself fails, the tag is
already public and re-running requires deleting it first.

This is accepted rather than solved: a GitHub release requires its tag to exist,
so release-then-tag is not possible. The README documents the recovery
(`git push --delete origin <tag>`, then re-dispatch).

### Caller permissions

Reusable workflows do not grant themselves permissions. The caller must declare:

```yaml
permissions:
  contents: write
```

This is documented in `examples/library-caller/` and in the README, because a
missing permission block produces a failure at release-creation time rather than
at parse time.

## `go-binary-release.yml`

For the CLI track. The consumer keeps its own `.goreleaser.yml` — `forge`,
`forgeui`, and `smart-form` already have one, and the config is genuinely
per-project. The workflow supplies the surrounding machinery: checkout with
`fetch-depth: 0`, Go setup, `goreleaser release --clean`, and token plumbing.

| input | type | default |
|---|---|---|
| `go-version` | string | `1.25` |
| `goreleaser-version` | string | see "Pinning tool versions" |
| `goreleaser-args` | string | `release --clean` |
| `working-directory` | string | `.` |

Secrets: `HOMEBREW_TAP_TOKEN`, `SCOOP_BUCKET_TOKEN`, and `GPG_PRIVATE_KEY`, all
optional.

Built in this effort, wired to no repository. `dtl` is a library and does not
use it. Migrating `forge`, `forgeui`, and `smart-form` is deferred, because that
track touches Homebrew and Scoop publishing and signing secrets — the riskiest
surface to change before the shared contract has proven itself.

## `codeql.yml`

A near-verbatim lift of the identical CodeQL workflow present in `confy`,
`vessel`, `go-utils`, and `farp`. Inputs: `go-version`, `working-directory`,
and `schedule-cron` (default weekly).

## dtl's consumer files

Three files, each a trigger block and a `uses:` line.

`.github/workflows/ci.yml`:

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

`.github/workflows/release.yml`:

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

`.github/workflows/codeql.yml` follows the same shape.

`dtl` needs no Makefile and no `.golangci.yml`. Make auto-detection falls back
to raw `go` commands, and the existing `boundary_test.go` plus package tests run
unchanged. `CHANGELOG.md` does not exist yet and is created by the first
dispatch release.

### Open item: dtl's Go version floor

`go.mod` declares `go 1.26.0`, so `dtl`'s CI matrix can only be `["1.26"]` —
older toolchains cannot build it. For a library whose stated purpose is being
embeddable, that is a narrow door.

Lowering the directive to `go 1.24` would allow a `["1.24","1.25","1.26"]`
matrix and widen adoption. This is flagged for the maintainer's decision and is
**not** changed as part of this work. If the floor is lowered later, the only
edit needed is the `go-versions` value in `dtl`'s `ci.yml`.

## Testing

Three layers, in increasing fidelity:

1. **`actionlint`** over every workflow file in `go-workflows`, on push and pull
   request. Catches YAML errors, invalid `${{ }}` expressions, unknown contexts,
   and bad `runs-on` values before any consumer sees them.
2. **Smoke call** — `self-test.yml` calls `./.github/workflows/go-ci.yml` with
   `working-directory: testdata/fixture-lib`, proving the reusable workflow
   actually executes. That fixture ships *without* a Makefile, so the raw-`go`
   fallback is the path under test. A second smoke job runs against
   `testdata/fixture-partial-make`, which defines `test` but not `test-coverage`,
   covering per-target detection in both directions within one run.
3. **First real consumer** — `dtl`. The genuine proof is a green CI run on a
   pull request, followed by cutting `dtl v0.1.0` through the dispatch path and
   confirming the tag, the release, the notes, and the pkg.go.dev entry.

`working-directory` earns its place here: it is what makes layer 2 possible
without putting a fake Go module at the root of `go-workflows`, and it also
serves monorepo consumers later.

## Failure modes and handling

| failure | handling |
|---|---|
| Makefile exists but lacks a target | per-target probe falls back to the `go` command; selection logged to the step summary |
| `CODECOV_TOKEN` unset | upload step skipped, job still passes |
| gosec/govulncheck upstream breakage | versions pinned as inputs, bumped deliberately |
| Dispatch version malformed | regex validation fails before any mutation |
| Tag already exists on dispatch | explicit check fails before any mutation |
| Dirty tree on dispatch | explicit check fails before any mutation |
| Release creation fails after tag push | tag must be deleted and the dispatch re-run; documented in the README |
| Caller omits `permissions: contents: write` | fails at release creation; the examples and README both carry the block |
| Commit message contains shell metacharacters | every value reaches bash through the step's `env:` as a quoted variable; no `${{ }}` appears inside any `run:` block |
| Commit-log line collides with the output delimiter | multiline `$GITHUB_OUTPUT` uses a per-run randomized heredoc delimiter |

### Release notes are assembled as data, never as code

`go-release.yml` builds `RELEASE_NOTES.md` in bash and hands it to
`ncipollo/release-action` via `bodyFile`, rather than composing the body from
inline `${{ }}` expressions the way the current xraph workflows do.

That choice is load-bearing for more than readability. GitHub substitutes
`${{ }}` into a `run:` script's *source* before bash parses it, so an
expression carrying `git log` output — arbitrary commit messages — becomes
executable code. An apostrophe in a commit message breaks the quoting; a
crafted message runs shell in a job holding `contents: write` and
`GITHUB_TOKEN`. The first implementation of this workflow shipped exactly that
bug and it was fixed in `go-workflows v1.1.1`.

The rule that follows: values reach bash through the step's `env:` block as
quoted variables. `${{ }}` belongs in `if:`, `with:`, `env:`, and
`working-directory:` — never in `run:`.

## Sequencing

1. Create `xraph/go-workflows` with `go-ci.yml`, `self-test.yml`, `actionlint`,
   and both fixtures. Tag `v1.0.0`, create the moving `v1`.
2. Add `dtl/.github/workflows/ci.yml`. Confirm green on a pull request.
3. Add `go-release.yml` and `codeql.yml` to `go-workflows`. Tag `v1.1.0`, move `v1`.
4. Add `dtl`'s `release.yml` and `codeql.yml`. Cut `dtl v0.1.0` through the
   dispatch path end to end.
5. Add `go-binary-release.yml` and `examples/cli-caller/`. Tag `v1.2.0`, move
   `v1`. No consumer wired.

Migrating `confy`, `vessel`, `go-utils`, and `farp` — and fixing the
confy-says-vessel bugs in the process — is a follow-up, deliberately deferred
until the contract has survived a real release.

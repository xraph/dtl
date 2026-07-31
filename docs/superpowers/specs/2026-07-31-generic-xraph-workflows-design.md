# Generic `xraph/workflows` — Phase 0 + 1

Date: 2026-07-31
Status: Approved design, not yet implemented
Supersedes nothing; extends `2026-07-30-shared-go-release-workflows-design.md`

## Context

`xraph/go-workflows` shipped `v1.3.0` with `go-ci.yml`, `go-release.yml`,
`go-binary-release.yml` and `codeql.yml`, proven on `xraph/dtl` (which published
`v1.0.1` through them). Migrating the remaining repos surfaced two facts that
change the shape of the work.

**First, the org is not one family of repos, it is two.**

| Family | Repos | Shape |
|---|---|---|
| A — copy-paste, manual release | `confy`, `vessel`, `go-utils` (and `dtl`, migrated) | Identical four-file sets, Makefiles, manual version bumps, no nested modules |
| B — already ahead | `spindle`, `farp` | semantic-release, CI-gated releases, SARIF security reporting, badges |

Family B is not a migration target. `spindle` derives versions from conventional
commits, gates releases on a green CI run, and publishes gosec findings to the
GitHub Security tab. `farp` does all of that plus tags six nested Go modules in
lockstep, builds and publishes a Rust crate to crates.io, and asserts that
`Cargo.toml`'s version matches the git tag. Replacing either with `go-release.yml`
would delete working infrastructure.

The correct direction is to harvest Family B's ideas upward into the shared repo.

**Second, the org is not only Go.**

| Track | Count | Repos |
|---|---|---|
| Go | 13 | `ai-sdk` `confy` `controlplane` `dtl` `dtl-history` `farp` `forge` `forge-cloud` `forgeui` `go-utils` `smart-form` `spindle` `vessel` |
| Node/TS | 6 | `forge-js` `stockgist` `tkm` `tkm-website` `website` `xraph` |
| Rust | 2 | `octopus` (cargo workspace: CLI + 6 plugins), `farp/farp-rust` |
| Dart/Flutter | 2 | `game-cli` `gameframework` |

The Node repos are predominantly private Next.js applications rather than
publishable packages, so a future Node track needs CI and deploy, not npm
publishing.

## Goal

Turn the repository into `xraph/workflows` — one home for every language's
CI and release workflows — and complete the Go track by harvesting Family B's
practices. Then migrate `confy`, `vessel` and `go-utils`.

Out of scope for this spec, each its own later phase: Rust (harvest from
`octopus`), Node/TS, Dart/Flutter. `spindle` and `farp` are not migrated.

## Decomposition and phasing

This spec covers Phase 0 and Phase 1 only.

| Phase | Content |
|---|---|
| **0** | Rename to `xraph/workflows`; add the language-agnostic `semantic-release.yml` |
| **1** | Harvest `spindle`'s hardening into the Go track; migrate `confy`, `vessel`, `go-utils` |
| 2 | Rust track, harvested from `octopus` and `farp-rust` |
| 3 | Node/TS track — CI and deploy for six application repos |
| 4 | Dart/Flutter track — `game-cli`'s cross-platform binaries and Homebrew |

## Architecture: one spine, thin per-language tracks

The idea that makes a single repository coherent rather than four repositories
sharing a name: **semantic-release is language-agnostic.** It is a Node tool,
but it does not care what it releases. It reads conventional commits, computes
the next version, writes the changelog, creates the GitHub release, and shells
out through `@semantic-release/exec` for anything ecosystem-specific — which is
exactly how `farp` drives `scripts/update-version.sh` today.

So the structure is not four parallel stacks. It is one shared release spine
plus a thin CI workflow per language, and a publish step only where an ecosystem
genuinely differs.

```
.github/workflows/
  semantic-release.yml       # the spine — every track calls this
  go-ci.yml                  # hardened this phase
  go-release.yml             # retained: manual/tag-driven, for repos not on semantic-release
  go-binary-release.yml
  codeql.yml                 # gains a query-pack input
  self-test.yml
  release.yml
examples/
  go-library/                # renamed from library-caller
  go-cli/                    # renamed from cli-caller
testdata/
  fixture-lib/ fixture-partial-make/ fixture-nodeps/
```

`rust-ci.yml`, `node-ci.yml` and `dart-ci.yml` land beside these in later phases
without further restructuring.

## Renaming without breaking consumers

`xraph/go-workflows` becomes `xraph/workflows`. GitHub serves redirects for
renamed repositories, so `dtl`'s existing `uses: xraph/go-workflows/...@v1`
continues to resolve during the transition. That redirect is a safety net, not
the plan: `dtl`'s three caller files are updated to the new path in the same
phase.

**Existing workflow filenames do not change.** `go-ci.yml` stays `go-ci.yml`.
Renaming files would force every consumer onto a new major tag for no benefit,
so `v1` continues across the rename. Only the two `examples/` directories are
renamed, and nothing references them programmatically.

## `semantic-release.yml`

A reusable workflow with no language-specific logic beyond one documented
affordance.

| input | type | default | notes |
|---|---|---|---|
| `node-version` | string | `22` | see below — not `20` |
| `semantic-release-version` | string | `25` | current major |
| `extra-plugins` | string | `'[]'` | JSON array of npm specs, e.g. `["@semantic-release/exec@7"]` |
| `dry-run` | boolean | `false` | used by this repo's own smoke test |
| `warm-go-proxy` | boolean | `false` | the single language-specific affordance |

Outputs: `version` (the released version, empty when nothing was released) and
`released` (`true`/`false`), so callers can chain follow-on jobs.

### Why the defaults diverge from `spindle` and `farp`

Both pin `semantic-release@23` on Node 20. The current plugin generation cannot
run there: `semantic-release@25`, `@semantic-release/github@12`,
`@semantic-release/git@11`, `@semantic-release/changelog@7` and
`conventional-changelog-conventionalcommits@10` all declare
`engines.node: ^22.14.0 || >=24.10.0`. Node 20 fails at install.

So the pinned set is Node 22 with the current plugin majors, not the versions
`spindle` runs. This is the same reasoning applied to `golangci-lint` in the
previous phase — a brand-new shared workflow should not ship two majors behind
on day one. `spindle` and `farp` keep their working Node 20 / v23 setup; neither
is being migrated, so nothing breaks.

Each consumer owns its own `.releaserc.json`. That is where genuinely
per-project configuration belongs — release rules, changelog preamble, and any
`@semantic-release/exec` hooks.

### Permissions the caller must declare

A `workflow_call` callee cannot widen the caller's token, so every caller of
`semantic-release.yml` declares:

```yaml
permissions:
  contents: write         # tag, and commit CHANGELOG.md via @semantic-release/git
  issues: write           # @semantic-release/github comments on resolved issues
  pull-requests: write    # and on the PRs that closed them
```

This matches what `spindle` and `farp` declare today. Omitting `issues:` or
`pull-requests:` does not fail the release — `@semantic-release/github` logs a
permissions warning and skips the comment — but omitting `contents: write`
fails it outright.

The workflow checks out with `persist-credentials: false` and passes
`GITHUB_TOKEN` to semantic-release explicitly, as `spindle` does, so the token
is never left configured in the git remote for later steps.

### Why `warm-go-proxy` exists

It is three lines, derived entirely from `github.repository`, and it preserves
behavior that `confy`, `vessel` and `go-utils` get from their current release
workflows. The alternative is four repositories copy-pasting a `curl` into their
callers, which is the problem this repository exists to solve. One small
impurity in the spine is the cheaper trade.

### Branch protection interacts with this

`@semantic-release/git` commits the updated `CHANGELOG.md` back to `main`. If
`main` is protected without an allowance for `github-actions[bot]`, that push
fails and the release aborts partway. `spindle` and `farp` run this today, so
the configuration is proven — but it is a real constraint, and it is in tension
with the recommendation from the previous phase's review that `xraph/workflows`
protect its own `main`. Resolve per repository: either exempt the bot, or drop
`@semantic-release/git` and let the changelog live only in the GitHub release.

## Hardening harvested from `spindle`

### gosec with SARIF, configurable failure

Today `go-ci.yml` runs gosec and fails the job on any finding, with findings
visible only in job logs. `spindle` runs gosec with `-no-fail -fmt sarif` and
uploads the result, so findings land in the GitHub Security tab with history,
triage and dismissal.

The shared workflow takes both: always produce and upload SARIF, then fail only
when `gosec-fail-on-findings` (new input, default `true`) is set. Repositories
that want gosec advisory-only opt out; the default preserves today's behavior.

Two consequences that must be handled rather than discovered:

- The `security` job needs `security-events: write`. The previous phase added a
  `contents: read` ceiling to every job in `go-ci.yml`; this job — and only this
  job — widens to include `security-events: write`.
- **Fork pull requests have no write token**, so SARIF upload fails there. The
  upload step is guarded so a fork PR degrades to log-only output instead of a
  confusing permissions error.

Note that `dtl` needed two `#nosec G404` annotations only because the current
policy fails the build. Those annotations remain accurate documentation and are
not removed.

### CodeQL query packs

`codeql.yml` gains a `queries` input defaulting to
`security-extended,security-and-quality`, matching `spindle`. This is a stronger
default than CodeQL's standard set and will surface findings on already-migrated
repositories including `dtl`. That is the intended behavior for a shared
workflow whose purpose is raising the floor; a repository that wants the
standard set passes `queries: ''`.

### CI-gated releases

`spindle` and `farp` both trigger release on `workflow_run` completion of the CI
workflow, and refuse to proceed unless `github.event.workflow_run.conclusion ==
'success'`. A tagged version is something downstream consumers pin, so releasing
off a red build is worse than releasing late.

`workflow_call` workflows cannot declare `workflow_run`, so this lives in the
caller. It ships in `examples/go-library/release.yml` and is documented in the
README, not embedded in a reusable workflow.

## Migrating `confy`, `vessel`, `go-utils`

All three are Family A: identical copied workflows, Makefiles, single modules,
conventional commit messages already in use.

Each repository receives:

| file | action |
|---|---|
| `.github/workflows/ci.yml` | replace — calls `go-ci.yml@v1` |
| `.github/workflows/codeql.yml` | replace — calls `codeql.yml@v1` |
| `.github/workflows/release.yml` | replace — calls `semantic-release.yml@v1`, triggered by `workflow_run` on CI success plus `workflow_dispatch` |
| `.github/workflows/auto-release.yml` | delete |
| `.releaserc.json` | create |

Deleting the old `release.yml` and `auto-release.yml` is what removes `confy`'s
live release-note bugs, which currently instruct users to
`go get github.com/xraph/vessel` and link `pkg.go.dev/github.com/xraph/vessel`.

Their Makefiles are handled by `go-ci.yml`'s existing per-target detection, so
no Makefile changes are needed.

### Version baselines

| repo | latest tag | first semantic-release bump |
|---|---|---|
| `confy` | `v0.5.2` | `feat` → `v0.6.0`; a breaking change → `v1.0.0` |
| `vessel` | `v1.0.2` | normal semver |
| `go-utils` | `v1.1.3` | normal semver |

`confy` is pre-1.0, so its jumps are larger than a manual bump would have been.
This is expected semantic-release behavior on a `0.x` line and is called out so
it is not mistaken for a defect.

None of the three has a `CHANGELOG.md`; `@semantic-release/changelog` creates
one on first release.

## Testing

1. **`actionlint`** over every workflow, with `shellcheck`, as today.
2. **Existing fixtures** — `fixture-lib` (no Makefile), `fixture-partial-make`
   (Makefile missing `test-coverage`), `fixture-nodeps` (no `go.sum`).
3. **New: a `semantic-release.yml` smoke job** running with `dry-run: true`
   against this repository itself. It exercises the spine — Node setup, plugin
   install, conventional-commit analysis — without cutting a release.
4. **First real consumer: `confy`**, chosen deliberately because its release
   notes carry the `vessel` bug. Verifying that the first semantic-release
   output names `confy` is the migration's proof.

## Rules carried forward from the previous phase

These remain absolute and apply to every new workflow:

- No `${{ }}` expression inside any `run:` block. Values pass through the step's
  `env:` and are referenced as quoted bash variables.
- A step's own `env:` block is not in scope in that step's `if:`; optional-secret
  gating uses a two-step check-then-use shape.
- A `workflow_call` callee's job-level `permissions:` can only narrow the
  caller's token, so every caller declares what its workflow needs.
- Repository identity is derived from `github.repository`, never hand-typed.
- `cache-dependency-path` references `go.mod`, never `go.sum`.
- No `Co-Authored-By` trailers in any commit.

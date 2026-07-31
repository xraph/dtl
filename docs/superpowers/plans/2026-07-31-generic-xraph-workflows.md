# Generic `xraph/workflows` — Phase 0 + 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rename `xraph/go-workflows` to `xraph/workflows`, add a language-agnostic `semantic-release.yml` spine, harden the Go track with `spindle`'s security practices, and migrate `confy`, `vessel` and `go-utils` onto automated releases.

**Architecture:** One shared release spine (`semantic-release.yml`) that any language track calls, plus thin per-language CI workflows. semantic-release is language-agnostic — it reads conventional commits, computes the version, writes the changelog, cuts the release, and shells out via `@semantic-release/exec` for ecosystem-specific work. Existing workflow filenames are unchanged so `v1` continues across the rename.

**Tech Stack:** GitHub Actions (`workflow_call`), semantic-release 25 on Node 22, gosec + SARIF, CodeQL, Go 1.24–1.26.

**Spec:** [`docs/superpowers/specs/2026-07-31-generic-xraph-workflows-design.md`](../specs/2026-07-31-generic-xraph-workflows-design.md)

## Global Constraints

**Four working trees.** Every `git` command below is prefixed with its directory. Never cross-commit.

| repo | path | state at plan time |
|---|---|---|
| workflows | `/Users/rexraphael/Work/xraph/go-workflows` | `main` at `0197cde`, tag `v1.3.0`, moving `v1` |
| confy | `/Users/rexraphael/Work/xraph/confy` | remote tag `v0.5.2` |
| vessel | `/Users/rexraphael/Work/xraph/vessel` | remote tag `v1.0.2` |
| go-utils | `/Users/rexraphael/Work/xraph/go-utils` | remote tag `v1.1.3` |

`dtl` at `/Users/rexraphael/Work/xraph/dtl` is touched only in Task 1, to repoint its three callers.

**Pinned action majors** (latest as of 2026-07-31):

| action | pin |
|---|---|
| `actions/checkout` | `v7` |
| `actions/setup-go` | `v7` |
| `actions/setup-node` | `v7` |
| `codecov/codecov-action` | `v7` |
| `golangci/golangci-lint-action` | `v9` |
| `ncipollo/release-action` | `v1` |
| `goreleaser/goreleaser-action` | `v7` |
| `github/codeql-action` | `v4` |

**Pinned tool defaults:** `golangci-lint v2.12.2`, `gosec v2.28.0`, `govulncheck v1.6.0`, `goreleaser v2.17.1`, `actionlint v1.7.12`.

**semantic-release pinned set — Node 22, not Node 20.** `semantic-release@25`, `@semantic-release/changelog@7`, `@semantic-release/git@11`, `@semantic-release/github@12`, `conventional-changelog-conventionalcommits@10`. All declare `engines.node: ^22.14.0 || >=24.10.0`; Node 20 fails at install. `spindle` and `farp` run `@23` on Node 20 and are **not** migrated.

**Absolute rules carried forward from the previous phase.** These are not style preferences — each one has already caused a real defect in this project:

1. **No `${{ }}` expression inside any `run:` block.** GitHub substitutes it into the script *source* before bash parses it, so the value becomes code. Pass through the step's `env:` and reference as a quoted bash variable. `${{ }}` in `if:`, `with:`, `env:`, `working-directory:` is correct.
2. **A step's own `env:` is not in scope in that step's `if:`.** Optional-secret gating uses a two-step check-then-use shape.
3. **A `workflow_call` callee's `permissions:` can only narrow the caller's token.** Every caller declares what its workflow needs.
4. **Repository identity is derived from `github.repository`, never hand-typed.**
5. **`cache-dependency-path` references `go.mod`, never `go.sum`.**
6. **Conventional commits. No `Co-Authored-By` trailers of any kind.**

**Two environment facts that bite:**
- `gh run list` immediately after a push often returns a **stale** run, and `gh run watch` on a finished run exits green instantly. Always match the run's `headSha` against the commit you pushed.
- `actionlint` runs `shellcheck` (installed at `/opt/homebrew/bin/shellcheck` and on runners). For `echo` strings containing literal markdown backticks it reports SC2016; follow the existing precedent in `go-ci.yml` — a narrowly-scoped `# shellcheck disable=SC2016` above the specific line — after confirming the string really is literal.

**Outward-facing gates.** Tasks 1, 4 and 5 rename a public repository, push to four public repos, and publish real releases. Each is marked **REQUIRES CONFIRMATION**.

---

## File Structure

**`xraph/workflows`** (renamed from `go-workflows`):

| file | responsibility |
|---|---|
| `.github/workflows/semantic-release.yml` | **new** — language-agnostic release spine |
| `.github/workflows/go-ci.yml` | modified — gosec SARIF, `gosec-fail-on-findings` |
| `.github/workflows/codeql.yml` | modified — `queries` input |
| `.github/workflows/go-release.yml` | unchanged — manual/tag-driven, retained |
| `.github/workflows/go-binary-release.yml` | unchanged |
| `.github/workflows/self-test.yml` | modified — dry-run semantic-release smoke job |
| `examples/go-library/` | renamed from `examples/library-caller/`, gains `release.yml` for the semantic-release path |
| `examples/go-cli/` | renamed from `examples/cli-caller/` |
| `README.md` | modified |

**Each of `confy`, `vessel`, `go-utils`:**

| file | action |
|---|---|
| `.github/workflows/ci.yml` | replace — calls `go-ci.yml@v1` |
| `.github/workflows/codeql.yml` | replace — calls `codeql.yml@v1` |
| `.github/workflows/release.yml` | replace — calls `semantic-release.yml@v1` |
| `.github/workflows/auto-release.yml` | delete |
| `.releaserc.json` | create |

---

## Task 1: Rename to `xraph/workflows` and repoint `dtl`

**REQUIRES CONFIRMATION** — renames a public repository and pushes to `xraph/dtl`.

**Files:**
- Rename: `examples/library-caller/` → `examples/go-library/`, `examples/cli-caller/` → `examples/go-cli/`
- Modify: `README.md`, `examples/go-library/ci.yml`, `examples/go-library/release.yml`, `examples/go-library/codeql.yml`, `examples/go-cli/release.yml`
- Modify: `/Users/rexraphael/Work/xraph/dtl/.github/workflows/{ci,release,codeql}.yml`

**Interfaces:**
- Consumes: nothing.
- Produces: the repository at `github.com/xraph/workflows`, with all `uses:` references in examples and `dtl` reading `xraph/workflows/.github/workflows/<file>@v1`. Every later task depends on this path.

- [ ] **Step 1: Rename the GitHub repository** — CONFIRM FIRST

```bash
gh repo rename workflows --repo xraph/go-workflows --yes
gh repo view xraph/workflows --json name,url,visibility
```

Expected: `name: workflows`, visibility `PUBLIC`. GitHub keeps a redirect from the old path, so `dtl` keeps working until Step 5.

- [ ] **Step 2: Repoint the local remote**

```bash
cd /Users/rexraphael/Work/xraph/go-workflows
git remote set-url origin git@github.com:xraph/workflows.git
git remote -v
git fetch --tags --force
```

Expected: `origin` shows `xraph/workflows.git` and the fetch succeeds.

The local directory keeps its `go-workflows` name — renaming it would invalidate every path in this plan. That mismatch is cosmetic.

- [ ] **Step 3: Rename the example directories**

```bash
cd /Users/rexraphael/Work/xraph/go-workflows
git mv examples/library-caller examples/go-library
git mv examples/cli-caller examples/go-cli
git status --short
```

- [ ] **Step 4: Update every `uses:` reference in the repo**

```bash
cd /Users/rexraphael/Work/xraph/go-workflows
grep -rl 'xraph/go-workflows' README.md examples/ | while read -r f; do
  sed -i '' 's|xraph/go-workflows|xraph/workflows|g' "$f"
done
grep -rn 'xraph/go-workflows' . --exclude-dir=.git || echo "no stale references"
```

Expected: the final grep prints `no stale references`. Note `sed -i ''` is the BSD/macOS form.

Also update any prose in `README.md` that names the repository, and its opening `uses:` snippet.

- [ ] **Step 5: Repoint `dtl`'s three callers** — CONFIRM FIRST

```bash
cd /Users/rexraphael/Work/xraph/dtl
git checkout main && git pull
sed -i '' 's|xraph/go-workflows|xraph/workflows|g' .github/workflows/ci.yml .github/workflows/release.yml .github/workflows/codeql.yml
grep -rn 'uses: xraph' .github/workflows/
```

Expected: three lines, all reading `xraph/workflows/.github/workflows/<file>@v1`.

- [ ] **Step 6: Lint both repos**

```bash
cd /Users/rexraphael/Work/xraph/go-workflows && "$(go env GOPATH)/bin/actionlint" -color; echo "exit=$?"
cd /Users/rexraphael/Work/xraph/dtl && "$(go env GOPATH)/bin/actionlint" -color; echo "exit=$?"
```

Expected: both exit 0.

- [ ] **Step 7: Commit and push both** — CONFIRM FIRST

```bash
cd /Users/rexraphael/Work/xraph/go-workflows
git add -A
git commit -m "refactor: rename to xraph/workflows and namespace examples by language"
git push

cd /Users/rexraphael/Work/xraph/dtl
git add .github/workflows
git commit -m "ci: point workflow callers at renamed xraph/workflows"
git push
```

- [ ] **Step 8: Confirm both repos are green on the renamed path**

```bash
cd /Users/rexraphael/Work/xraph/go-workflows
sleep 10
gh run watch "$(gh run list --workflow=self-test.yml --limit 1 --json databaseId -q '.[0].databaseId')" --exit-status

cd /Users/rexraphael/Work/xraph/dtl
sleep 10
gh run watch "$(gh run list --workflow=ci.yml --limit 1 --json databaseId -q '.[0].databaseId')" --exit-status
```

Expected: both green. `dtl`'s run is the real proof that `xraph/workflows/...@v1` resolves — if it fails with "workflow was not found", the tag did not survive the rename and that is a blocker, not something to work around.

---

## Task 2: `semantic-release.yml` — the spine

**Files:**
- Create: `.github/workflows/semantic-release.yml`
- Modify: `.github/workflows/self-test.yml` (dry-run smoke job)
- Create: `examples/go-library/release-semantic.yml`
- Modify: `README.md`

**Interfaces:**
- Consumes: the renamed repo from Task 1.
- Produces: `semantic-release.yml` with inputs `node-version`, `semantic-release-version`, `extra-plugins`, `dry-run`, `warm-go-proxy`; outputs `version` (no leading `v`, empty when nothing released) and `released` (`true`/`false`). Tasks 4 and 5 call it from `confy`, `vessel` and `go-utils`.

- [ ] **Step 1: Write `semantic-release.yml`**

```yaml
name: Semantic Release

on:
  workflow_call:
    inputs:
      node-version:
        description: 'Node version. Must be 22+ for the pinned plugin set.'
        type: string
        default: '22'
      semantic-release-version:
        type: string
        default: '25'
      extra-plugins:
        description: 'JSON array of extra npm specs, e.g. ["@semantic-release/exec@7"]'
        type: string
        default: '[]'
      dry-run:
        description: 'Compute the next release without publishing anything.'
        type: boolean
        default: false
      warm-go-proxy:
        description: 'After a release, prime proxy.golang.org for this module.'
        type: boolean
        default: false
    outputs:
      version:
        description: 'Released version without a leading v. Empty if nothing was released.'
        value: ${{ jobs.release.outputs.version }}
      released:
        description: 'true when a release was published.'
        value: ${{ jobs.release.outputs.released }}

jobs:
  release:
    name: Semantic Release
    runs-on: ubuntu-latest
    permissions:
      contents: write
      issues: write
      pull-requests: write
    outputs:
      version: ${{ steps.semrel.outputs.version }}
      released: ${{ steps.semrel.outputs.released }}
    steps:
      - name: Checkout
        uses: actions/checkout@v7
        with:
          fetch-depth: 0
          persist-credentials: false

      - name: Set up Node.js
        uses: actions/setup-node@v7
        with:
          node-version: ${{ inputs.node-version }}

      - name: Install semantic-release
        shell: bash
        env:
          SEMREL_VERSION: ${{ inputs.semantic-release-version }}
          EXTRA_PLUGINS: ${{ inputs.extra-plugins }}
        run: |
          set -euo pipefail
          BASE="semantic-release@${SEMREL_VERSION}"
          BASE="$BASE @semantic-release/changelog@7"
          BASE="$BASE @semantic-release/git@11"
          BASE="$BASE @semantic-release/github@12"
          BASE="$BASE conventional-changelog-conventionalcommits@10"
          EXTRA=$(printf '%s' "$EXTRA_PLUGINS" | jq -r 'join(" ")')
          echo "Installing: $BASE $EXTRA"
          # npm package specs never contain spaces, so word splitting is intended here.
          # shellcheck disable=SC2086
          npm install -g $BASE $EXTRA

      - name: Run semantic-release
        id: semrel
        shell: bash
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          GIT_AUTHOR_NAME: github-actions[bot]
          GIT_AUTHOR_EMAIL: github-actions[bot]@users.noreply.github.com
          GIT_COMMITTER_NAME: github-actions[bot]
          GIT_COMMITTER_EMAIL: github-actions[bot]@users.noreply.github.com
          DRY_RUN: ${{ inputs.dry-run }}
        run: |
          set -euo pipefail
          if [ "$DRY_RUN" = 'true' ]; then
            npx semantic-release --dry-run 2>&1 | tee semantic-release.log
          else
            npx semantic-release 2>&1 | tee semantic-release.log
          fi

          VERSION=""
          RELEASED=false
          if grep -qE 'Published release|Release note for version' semantic-release.log; then
            VERSION=$(grep -oE '(Published release|Release note for version) [0-9]+\.[0-9]+\.[0-9]+[-+.0-9A-Za-z]*' semantic-release.log \
              | tail -1 | grep -oE '[0-9]+\.[0-9]+\.[0-9]+[-+.0-9A-Za-z]*')
            if [ "$DRY_RUN" != 'true' ]; then
              RELEASED=true
            fi
          fi
          echo "version=$VERSION" >> "$GITHUB_OUTPUT"
          echo "released=$RELEASED" >> "$GITHUB_OUTPUT"
          {
            echo "### Semantic release"
            if [ -n "$VERSION" ]; then
              echo "- next version: \`$VERSION\`"
            else
              echo "- no release warranted by the commits in range"
            fi
            echo "- dry run: \`$DRY_RUN\`"
          } >> "$GITHUB_STEP_SUMMARY"

      - name: Warm the Go module proxy
        if: inputs.warm-go-proxy && steps.semrel.outputs.released == 'true'
        shell: bash
        env:
          VERSION: ${{ steps.semrel.outputs.version }}
        run: |
          set -euo pipefail
          curl -sSf "https://proxy.golang.org/github.com/${GITHUB_REPOSITORY}/@v/v${VERSION}.info" || true
```

**Two details that matter.** `semantic-release` emits `Published release X` on a real run and `Release note for version X` on a dry run, which is why the grep matches both — a dry run must still report the computed version, but must never set `released=true`. And `pipefail` is deliberate: with `| tee`, a semantic-release failure would otherwise be masked by `tee`'s exit code.

- [ ] **Step 2: Run actionlint, expect it to pass**

```bash
cd /Users/rexraphael/Work/xraph/go-workflows && "$(go env GOPATH)/bin/actionlint" -color; echo "exit=$?"
```

Expected: exit 0.

- [ ] **Step 3: Add the dry-run smoke job to `self-test.yml`**

Append to the `jobs:` block:

```yaml
  smoke-semantic-release:
    name: Smoke — semantic-release dry run
    permissions:
      contents: write
      issues: write
      pull-requests: write
    uses: ./.github/workflows/semantic-release.yml
    with:
      dry-run: true
```

This exercises the whole spine — Node setup, plugin install, conventional-commit analysis against this repo's own history — without publishing. The `permissions:` block is required because a callee cannot widen the caller's token.

- [ ] **Step 4: Write `examples/go-library/release-semantic.yml`**

```yaml
# Copy to .github/workflows/release.yml in a Go library repository that
# releases automatically from conventional commits.
#
# Releases are gated on CI passing: a tagged version is something downstream
# consumers pin, so releasing off a red build is worse than releasing late.
name: Release

on:
  workflow_dispatch:
  workflow_run:
    workflows: ["CI"]
    types: [completed]
    branches: [main]

# Required: a called workflow can only narrow the caller's token, never widen it.
permissions:
  contents: write
  issues: write
  pull-requests: write

jobs:
  release:
    # Never release off a red build.
    if: github.event_name == 'workflow_dispatch' || github.event.workflow_run.conclusion == 'success'
    uses: xraph/workflows/.github/workflows/semantic-release.yml@v1
    with:
      warm-go-proxy: true
```

- [ ] **Step 5: Document the spine in `README.md`**

Append:

```markdown
## semantic-release

`semantic-release.yml` is language-agnostic. It reads conventional commits,
computes the next version, writes the changelog, and creates the GitHub release.
Anything ecosystem-specific belongs in the consumer's own `.releaserc.json`,
via `@semantic-release/exec`.

The pinned set is Node 22 with `semantic-release@25`. The current plugin
generation declares `engines.node: ^22.14.0 || >=24.10.0`, so Node 20 fails at
install. `spindle` and `farp` run `@23` on Node 20; they are unaffected.

### Caller permissions

```yaml
permissions:
  contents: write         # tag, and commit CHANGELOG.md via @semantic-release/git
  issues: write           # comment on resolved issues
  pull-requests: write    # and on the PRs that closed them
```

Omitting `issues:` or `pull-requests:` only logs a warning and skips the
comment. Omitting `contents: write` fails the release.

### Branch protection

`@semantic-release/git` commits `CHANGELOG.md` back to `main`. If `main` is
protected without an allowance for `github-actions[bot]`, that push fails and
the release aborts partway. Either exempt the bot, or drop
`@semantic-release/git` and let the changelog live only in the GitHub release.
```

- [ ] **Step 6: Run actionlint again, expect it to pass**

```bash
cd /Users/rexraphael/Work/xraph/go-workflows && "$(go env GOPATH)/bin/actionlint" -color; echo "exit=$?"
```

Expected: exit 0. This run also validates the local `uses: ./.github/workflows/semantic-release.yml` call, so an input-name typo fails here rather than in CI.

- [ ] **Step 7: Commit, push, and confirm the smoke job passes**

```bash
cd /Users/rexraphael/Work/xraph/go-workflows
git add .github/workflows/semantic-release.yml .github/workflows/self-test.yml examples/go-library/release-semantic.yml README.md
git commit -m "feat: add language-agnostic semantic-release reusable workflow"
git push
sleep 10
gh run watch "$(gh run list --workflow=self-test.yml --limit 1 --json databaseId -q '.[0].databaseId')" --exit-status
```

Expected: all jobs pass, including `smoke-semantic-release`. Open the run summary and confirm the dry run reported either a computed next version or "no release warranted" — a crash in plugin install would fail the job, but a silently empty result would not, so read the summary rather than trusting the green tick.

**If the dry run fails, read the error before changing anything.** This repository has no `.releaserc.json`, so semantic-release falls back to its default configuration — that is intentional, since the point is to exercise the workflow's plumbing, not this repo's release policy. Two known failure modes and their correct responses:

- `EGITNOPERMISSION` or a push-permission check — dry-run mode normally skips this. If it appears anyway, add `--no-ci` to the dry-run invocation rather than granting the smoke job more permissions.
- A branch-configuration error, because semantic-release's default `branches` may not include this repo's `main` in the way the default preset expects — add a minimal `.releaserc.json` to `xraph/workflows` itself declaring `{"branches":["main"]}`, and say so in the report.

Do not make the smoke job pass by removing it or by setting `continue-on-error`.

---

## Task 3: Harden the Go track

**Files:**
- Modify: `.github/workflows/go-ci.yml` (`security` job, new input)
- Modify: `.github/workflows/codeql.yml` (new input)
- Modify: `README.md`

**Interfaces:**
- Consumes: nothing from Task 2.
- Produces: `go-ci.yml` gains input `gosec-fail-on-findings` (boolean, default `true`); `codeql.yml` gains input `queries` (string, default `security-extended,security-and-quality`).

- [ ] **Step 1: Add the `gosec-fail-on-findings` input to `go-ci.yml`**

In the `inputs:` block, after `skip-security`:

```yaml
      gosec-fail-on-findings:
        description: 'Fail the build on gosec findings. SARIF is uploaded either way.'
        type: boolean
        default: true
```

- [ ] **Step 2: Replace the `security` job's gosec step with the SARIF flow**

The job's `permissions:` block widens by exactly one scope:

```yaml
    permissions:
      contents: read
      security-events: write
```

Replace the existing `Run gosec` step with these four steps, keeping the `Run govulncheck` step that follows unchanged:

```yaml
      - name: Run gosec
        shell: bash
        env:
          GOSEC_VERSION: ${{ inputs.gosec-version }}
        run: |
          set -euo pipefail
          go install "github.com/securego/gosec/v2/cmd/gosec@${GOSEC_VERSION}"
          gosec -no-fail -fmt sarif -out gosec.sarif -exclude=G115 -exclude-dir=vendor ./...

      - name: Determine whether SARIF can be uploaded
        id: sarif
        shell: bash
        env:
          IS_FORK: ${{ github.event_name == 'pull_request' && github.event.pull_request.head.repo.fork }}
        run: |
          set -euo pipefail
          if [ "$IS_FORK" = 'true' ]; then
            echo 'upload=false' >> "$GITHUB_OUTPUT"
            echo "Fork pull request: no write token, so SARIF upload is skipped." >> "$GITHUB_STEP_SUMMARY"
          else
            echo 'upload=true' >> "$GITHUB_OUTPUT"
          fi

      - name: Upload gosec SARIF
        if: steps.sarif.outputs.upload == 'true'
        uses: github/codeql-action/upload-sarif@v4
        with:
          sarif_file: ${{ inputs.working-directory }}/gosec.sarif
          category: gosec

      - name: Report gosec findings
        shell: bash
        env:
          FAIL_ON_FINDINGS: ${{ inputs.gosec-fail-on-findings }}
        run: |
          set -euo pipefail
          COUNT=$(jq '[.runs[].results[]] | length' gosec.sarif)
          echo "- gosec findings: **${COUNT}**" >> "$GITHUB_STEP_SUMMARY"
          if [ "$COUNT" -gt 0 ]; then
            jq -r '.runs[].results[] | "\(.ruleId): \(.message.text)"' gosec.sarif
            if [ "$FAIL_ON_FINDINGS" = 'true' ]; then
              echo "Failing because gosec-fail-on-findings is true."
              exit 1
            fi
            echo "gosec-fail-on-findings is false; reporting only."
          fi
```

**Why this shape.** One gosec run produces both behaviors: `-no-fail` always writes SARIF so findings reach the Security tab with history and dismissal, and the separate reporting step decides whether findings block the build. The fork guard exists because a pull request from a fork has a read-only token and `upload-sarif` would otherwise fail with a confusing permissions error rather than degrading to log-only.

- [ ] **Step 3: Add the `queries` input to `codeql.yml`**

In `inputs:`:

```yaml
      queries:
        description: 'CodeQL query packs. Empty string selects the default pack.'
        type: string
        default: 'security-extended,security-and-quality'
```

And in the init step:

```yaml
      - name: Initialize CodeQL
        uses: github/codeql-action/init@v4
        with:
          languages: go
          queries: ${{ inputs.queries }}
```

This is a stronger default than CodeQL's standard set and will surface new findings on already-migrated repositories including `dtl`. That is intended for a workflow whose job is raising the floor.

- [ ] **Step 4: Document both in `README.md`**

Append:

```markdown
## Security reporting

`go-ci.yml` runs gosec with `-no-fail -fmt sarif` and uploads the result, so
findings appear in the repository's Security tab with history, triage and
dismissal — not only in job logs. It then fails the build when
`gosec-fail-on-findings` (default `true`) is set. Pass `false` for
advisory-only scanning.

SARIF upload is skipped on pull requests from forks, which have no write token.
Those runs degrade to log-only output.

`codeql.yml` defaults to the `security-extended,security-and-quality` query
packs. Pass `queries: ''` for CodeQL's standard set.
```

- [ ] **Step 5: Run actionlint, expect it to pass**

```bash
cd /Users/rexraphael/Work/xraph/go-workflows && "$(go env GOPATH)/bin/actionlint" -color; echo "exit=$?"
```

Expected: exit 0.

- [ ] **Step 6: Commit, push, and confirm the smoke jobs still pass**

```bash
cd /Users/rexraphael/Work/xraph/go-workflows
git add .github/workflows/go-ci.yml .github/workflows/codeql.yml README.md
git commit -m "feat: upload gosec SARIF and make build failure configurable"
git push
sleep 10
gh run watch "$(gh run list --workflow=self-test.yml --limit 1 --json databaseId -q '.[0].databaseId')" --exit-status
```

The three existing smoke jobs pass `skip-security: true`, so they will not exercise the new steps. That is covered in Step 7.

- [ ] **Step 7: Prove the SARIF path actually runs**

Temporarily flip one smoke job to exercise it:

```bash
cd /Users/rexraphael/Work/xraph/go-workflows
sed -i '' 's|      skip-security: true|      skip-security: false|' .github/workflows/self-test.yml
git diff --stat
```

Apply this to the `smoke-fallback` job only — leave the other two skipping security so the run stays fast. Commit, push, watch, then confirm in the run:

- the `Security` job passed,
- the step summary shows a `gosec findings:` line,
- the run's SARIF reached the Security tab: `gh api "repos/xraph/workflows/code-scanning/analyses?tool_name=gosec" --jq '.[0] | {created_at, results_count}'`.

Leave `skip-security: false` on that one job permanently — a security path nothing exercises is a security path nobody notices breaking.

```bash
git add .github/workflows/self-test.yml
git commit -m "test: exercise the gosec SARIF path in the fallback smoke job"
git push
sleep 10
gh run watch "$(gh run list --workflow=self-test.yml --limit 1 --json databaseId -q '.[0].databaseId')" --exit-status
```

- [ ] **Step 8: Tag `v1.4.0`** — CONFIRM FIRST

```bash
cd /Users/rexraphael/Work/xraph/go-workflows
git tag -a v1.4.0 -m "Release v1.4.0 - semantic-release spine, gosec SARIF, CodeQL query packs"
git push origin v1.4.0
sleep 10
gh run watch "$(gh run list --workflow=release.yml --limit 1 --json databaseId -q '.[0].databaseId')" --exit-status
git fetch --tags --force
git rev-parse v1^{commit} v1.4.0^{commit}
```

Expected: both print the same SHA.

---

## Task 4: Migrate `confy`

**REQUIRES CONFIRMATION** — pushes to `xraph/confy` and publishes a real release.

`confy` goes first deliberately: its current release notes carry the bug this whole project exists to fix, telling users to `go get github.com/xraph/vessel`.

**Files (all in `/Users/rexraphael/Work/xraph/confy`):**
- Create: `.releaserc.json`
- Replace: `.github/workflows/ci.yml`, `.github/workflows/codeql.yml`, `.github/workflows/release.yml`
- Delete: `.github/workflows/auto-release.yml`

**Interfaces:**
- Consumes: `semantic-release.yml@v1`, `go-ci.yml@v1`, `codeql.yml@v1` from Tasks 1–3.
- Produces: the migration pattern that Task 5 repeats for `vessel` and `go-utils`.

- [ ] **Step 1: Confirm the starting state**

```bash
cd /Users/rexraphael/Work/xraph/confy
git checkout main && git pull
git fetch --tags
git ls-remote --tags origin | awk -F/ '{print $NF}' | grep -v '\^{}' | sort -V | tail -3
head -3 go.mod
ls Makefile .golangci.yml 2>/dev/null
grep -rn 'xraph/vessel' .github/workflows/ | head
```

Expected: latest tag `v0.5.2`; `go 1.25.3`; a `Makefile` exists. The final grep shows the `vessel` references that are about to be deleted — record them, they are the before-picture.

- [ ] **Step 2: Verify `confy` passes the shared checks locally before wiring CI**

```bash
cd /Users/rexraphael/Work/xraph/confy
gofmt -l .
go vet ./...
go mod tidy && git diff --exit-code -- go.mod go.sum
go test -race ./...
```

Expected: `gofmt -l .` prints nothing; the rest exit 0. Fix anything failing in its own separate commit before the migration commit — a formatting fix and a CI migration are two different reviews.

- [ ] **Step 3: Create the branch**

```bash
cd /Users/rexraphael/Work/xraph/confy
git checkout -b ci/shared-workflows
```

- [ ] **Step 4: Write `.releaserc.json`**

```json
{
  "branches": ["main"],
  "plugins": [
    [
      "@semantic-release/commit-analyzer",
      {
        "preset": "conventionalcommits",
        "releaseRules": [
          { "type": "feat", "release": "minor" },
          { "type": "fix", "release": "patch" },
          { "type": "perf", "release": "patch" },
          { "type": "revert", "release": "patch" },
          { "type": "docs", "release": "patch" },
          { "type": "refactor", "release": "patch" },
          { "type": "chore", "release": false },
          { "type": "test", "release": false },
          { "type": "build", "release": false },
          { "type": "ci", "release": false },
          { "breaking": true, "release": "major" }
        ]
      }
    ],
    [
      "@semantic-release/release-notes-generator",
      { "preset": "conventionalcommits" }
    ],
    [
      "@semantic-release/changelog",
      { "changelogFile": "CHANGELOG.md" }
    ],
    "@semantic-release/github",
    [
      "@semantic-release/git",
      {
        "assets": ["CHANGELOG.md"],
        "message": "chore(release): ${nextRelease.version} [skip ci]\n\n${nextRelease.notes}"
      }
    ]
  ]
}
```

These release rules are `spindle`'s, which are already proven in this org.

- [ ] **Step 5: Write `.github/workflows/ci.yml`**

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  ci:
    uses: xraph/workflows/.github/workflows/go-ci.yml@v1
    with:
      go-versions: '["1.25","1.26"]'
    secrets:
      CODECOV_TOKEN: ${{ secrets.CODECOV_TOKEN }}
```

`go.mod` declares `go 1.25.3`, so `1.25` is the floor. Never list a version below the `go` directive — `setup-go` installs it and the build then fails the directive check.

- [ ] **Step 6: Write `.github/workflows/release.yml`**

```yaml
name: Release

on:
  workflow_dispatch:
  workflow_run:
    workflows: ["CI"]
    types: [completed]
    branches: [main]

permissions:
  contents: write
  issues: write
  pull-requests: write

jobs:
  release:
    if: github.event_name == 'workflow_dispatch' || github.event.workflow_run.conclusion == 'success'
    uses: xraph/workflows/.github/workflows/semantic-release.yml@v1
    with:
      warm-go-proxy: true
```

- [ ] **Step 7: Write `.github/workflows/codeql.yml`**

```yaml
name: CodeQL

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
  schedule:
    - cron: '0 0 * * 1'

permissions:
  actions: read
  contents: read
  security-events: write

jobs:
  analyze:
    uses: xraph/workflows/.github/workflows/codeql.yml@v1
    with:
      go-version: '1.26'
```

- [ ] **Step 8: Delete the old workflows**

```bash
cd /Users/rexraphael/Work/xraph/confy
git rm .github/workflows/auto-release.yml
grep -rn 'xraph/vessel' .github/ || echo "vessel references gone"
```

Expected: the grep prints `vessel references gone`. That is the bug fix.

- [ ] **Step 9: Lint and push** — CONFIRM FIRST

```bash
cd /Users/rexraphael/Work/xraph/confy
"$(go env GOPATH)/bin/actionlint" -color; echo "exit=$?"
git add -A
git commit -m "ci: adopt shared xraph/workflows with automated releases"
git push -u origin ci/shared-workflows
gh pr create --fill --title "ci: adopt shared xraph/workflows"
gh pr checks --watch
```

Expected: CI and CodeQL green. The release workflow does not run on a PR — it has no `pull_request` trigger.

If `lint` fails, `confy` has its own lint policy to fix; do not relax the shared workflow, which is published and consumed by other repositories.

- [ ] **Step 10: Merge and watch the first automated release** — CONFIRM FIRST

```bash
cd /Users/rexraphael/Work/xraph/confy
gh pr merge --squash --delete-branch
git checkout main && git pull
sleep 20
gh run list --workflow=release.yml --limit 3
gh run watch "$(gh run list --workflow=release.yml --limit 1 --json databaseId -q '.[0].databaseId')" --exit-status
```

The merge commit is `ci: ...`, which `.releaserc.json` maps to `release: false` — so the first post-merge run may correctly decide **no release is warranted**. That is success, not failure. Read the step summary to confirm which happened.

- [ ] **Step 11: Verify the outcome**

```bash
cd /Users/rexraphael/Work/xraph/confy
git pull
gh release list --repo xraph/confy --limit 3
```

If a release was cut, read its body and confirm **every reference names `confy`, not `vessel`** — that is the whole point of this task. If no release was warranted, confirm instead that the run summary says so, and that the next `feat:`/`fix:` merge will cut `v0.6.0` or `v0.5.3` respectively.

`confy` is pre-1.0 at `v0.5.2`, so a `feat:` takes it to `v0.6.0` and a breaking change to `v1.0.0` — larger jumps than a manual bump. This is expected semantic-release behavior on a `0.x` line.

---

## Task 5: Migrate `vessel` and `go-utils`

**REQUIRES CONFIRMATION** — pushes to two public repos and enables automated releases on both.

**Files:** the same five files as Task 4, in `/Users/rexraphael/Work/xraph/vessel` and `/Users/rexraphael/Work/xraph/go-utils`.

**Interfaces:**
- Consumes: the pattern proven on `confy` in Task 4.
- Produces: nothing later depends on.

- [ ] **Step 1: Confirm both starting states**

```bash
for r in vessel go-utils; do
  cd "/Users/rexraphael/Work/xraph/$r"
  echo "=== $r ==="
  git checkout main && git pull && git fetch --tags
  git ls-remote --tags origin | awk -F/ '{print $NF}' | grep -v '\^{}' | sort -V | tail -2
  head -3 go.mod
done
```

Expected: `vessel` at `v1.0.2` with `go 1.25.0`; `go-utils` at `v1.1.3` with `go 1.25.0`. Both have Makefiles.

- [ ] **Step 2: Verify both pass the shared checks locally**

```bash
for r in vessel go-utils; do
  cd "/Users/rexraphael/Work/xraph/$r"
  echo "=== $r ==="
  gofmt -l .
  go vet ./...
  go mod tidy && git diff --exit-code -- go.mod go.sum
  go test -race ./...
done
```

Expected: no `gofmt` output, all commands exit 0. Fix failures in their own commits first.

- [ ] **Step 3: Apply the migration to `vessel`**

None of these files contains a repository name — the module path is derived from `github.repository` — so the same four files apply to both repos verbatim.

```bash
cd /Users/rexraphael/Work/xraph/vessel
git checkout -b ci/shared-workflows
mkdir -p .github/workflows
```

`.releaserc.json`:

```json
{
  "branches": ["main"],
  "plugins": [
    [
      "@semantic-release/commit-analyzer",
      {
        "preset": "conventionalcommits",
        "releaseRules": [
          { "type": "feat", "release": "minor" },
          { "type": "fix", "release": "patch" },
          { "type": "perf", "release": "patch" },
          { "type": "revert", "release": "patch" },
          { "type": "docs", "release": "patch" },
          { "type": "refactor", "release": "patch" },
          { "type": "chore", "release": false },
          { "type": "test", "release": false },
          { "type": "build", "release": false },
          { "type": "ci", "release": false },
          { "breaking": true, "release": "major" }
        ]
      }
    ],
    [
      "@semantic-release/release-notes-generator",
      { "preset": "conventionalcommits" }
    ],
    [
      "@semantic-release/changelog",
      { "changelogFile": "CHANGELOG.md" }
    ],
    "@semantic-release/github",
    [
      "@semantic-release/git",
      {
        "assets": ["CHANGELOG.md"],
        "message": "chore(release): ${nextRelease.version} [skip ci]\n\n${nextRelease.notes}"
      }
    ]
  ]
}
```

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
    uses: xraph/workflows/.github/workflows/go-ci.yml@v1
    with:
      go-versions: '["1.25","1.26"]'
    secrets:
      CODECOV_TOKEN: ${{ secrets.CODECOV_TOKEN }}
```

`.github/workflows/release.yml`:

```yaml
name: Release

on:
  workflow_dispatch:
  workflow_run:
    workflows: ["CI"]
    types: [completed]
    branches: [main]

permissions:
  contents: write
  issues: write
  pull-requests: write

jobs:
  release:
    if: github.event_name == 'workflow_dispatch' || github.event.workflow_run.conclusion == 'success'
    uses: xraph/workflows/.github/workflows/semantic-release.yml@v1
    with:
      warm-go-proxy: true
```

`.github/workflows/codeql.yml`:

```yaml
name: CodeQL

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
  schedule:
    - cron: '0 0 * * 1'

permissions:
  actions: read
  contents: read
  security-events: write

jobs:
  analyze:
    uses: xraph/workflows/.github/workflows/codeql.yml@v1
    with:
      go-version: '1.26'
```

Then remove the old workflow and lint:

```bash
cd /Users/rexraphael/Work/xraph/vessel
git rm .github/workflows/auto-release.yml
"$(go env GOPATH)/bin/actionlint" -color; echo "exit=$?"
```

Expected: exit 0.

- [ ] **Step 4: Push `vessel` and merge** — CONFIRM FIRST

```bash
cd /Users/rexraphael/Work/xraph/vessel
git add -A
git commit -m "ci: adopt shared xraph/workflows with automated releases"
git push -u origin ci/shared-workflows
gh pr create --fill --title "ci: adopt shared xraph/workflows"
gh pr checks --watch
gh pr merge --squash --delete-branch
```

- [ ] **Step 5: Apply the same migration to `go-utils`** — CONFIRM FIRST

`go-utils` also declares `go 1.25.0`, so **all four files are byte-identical to the ones written for `vessel` in Step 3** — including `go-versions: '["1.25","1.26"]'`. Write the same `.releaserc.json`, `ci.yml`, `release.yml` and `codeql.yml` shown in Step 3 into `/Users/rexraphael/Work/xraph/go-utils`.

```bash
cd /Users/rexraphael/Work/xraph/go-utils
git checkout -b ci/shared-workflows
mkdir -p .github/workflows
# write the four files exactly as in Step 3
git rm .github/workflows/auto-release.yml
"$(go env GOPATH)/bin/actionlint" -color; echo "exit=$?"
git add -A
git commit -m "ci: adopt shared xraph/workflows with automated releases"
git push -u origin ci/shared-workflows
gh pr create --fill --title "ci: adopt shared xraph/workflows"
gh pr checks --watch
gh pr merge --squash --delete-branch
```

Expected: `actionlint` exit 0, CI and CodeQL green on the PR.

- [ ] **Step 6: Confirm the end state across all four Go repos**

```bash
for r in dtl confy vessel go-utils; do
  echo "=== $r ==="
  gh api "repos/xraph/$r/contents/.github/workflows" --jq '.[].name' 2>/dev/null | tr '\n' ' '
  echo
  gh run list --repo "xraph/$r" --limit 2 --json conclusion,name -q '.[] | "  \(.conclusion // "running")  \(.name)"'
done
```

Expected: `confy`, `vessel` and `go-utils` each show exactly `ci.yml`, `codeql.yml`, `release.yml` — no `auto-release.yml` — with green recent runs. `dtl` keeps its manual `go-release.yml` caller and is unchanged by this task.

---

## Follow-up, explicitly not in this plan

- **Phase 2 — Rust track**, harvested from `octopus` (cargo workspace, crates.io, Docker, Helm, Windows) and `farp/farp-rust`.
- **Phase 3 — Node/TS track** for six repos, which are predominantly private Next.js applications needing CI and deploy rather than npm publishing.
- **Phase 4 — Dart/Flutter track**, covering `game-cli`'s cross-platform binary matrix and Homebrew formula.
- Migrating `ai-sdk`, `controlplane`, `forge`, `forge-cloud`, `forgeui`, `smart-form` — the remaining Go repos, not surveyed in detail.
- `spindle` and `farp` stay as they are. Converging them onto the shared spine is only worth doing once the spine can express everything they do, including `farp`'s nested-module tagging and Rust publishing.
- Branch protection on `xraph/workflows` `main`, and SHA-pinning `ncipollo/release-action` — both carried over from the previous phase's review, both still open.

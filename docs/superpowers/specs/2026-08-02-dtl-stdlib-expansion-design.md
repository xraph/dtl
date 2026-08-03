# DTL Standard Library Expansion — Design

Date: 2026-08-02
Status: Approved for implementation planning

## Goal

Close the gaps that made real transformations awkward to write, and bring the
library to rough parity with jq, JSONata, and CEL. Roughly 99 new functions
across two phases, plus one corrective breaking change held back for a
deliberate major release.

Four gap clusters drove this, all named as real friction:

1. Null and default handling — no `coalesce`, so every optional field needs a
   verbose `if is_null(x) then ... else ...`.
2. Nested path access — no `get`/`set` on dotted paths, so reshaping nested
   objects is hand-rolled per level.
3. Strings — only `match_regex` exists; no regex replace/split/capture, no case
   conversion, no encoding, no hashing.
4. JSON — no way to read a JSON string into a value or serialize one out.

## Naming model

The rule, stated so a future contributor can apply it without asking:

> A function that extends an existing family gets a **bare name**. A function
> that opens a **new capability area** — one with no home in the library today —
> gets a **two-segment namespace**.

So `coalesce`, `snake_case`, `flat_map`, and `exp` are bare, joining their
existing bare families. Five new namespaces appear: `json::`, `regex::`,
`encoding::`, `hash::`, `path::`.

No `system::` aliases are added for anything new. The existing `system::text::*`,
`system::math::*`, and `system::collections::*` aliases remain registered and
supported as legacy; they are not a pattern to extend.

### SPEC.md amendment

`SPEC.md` currently presents `::` as an ownership tier — `system::`, `shared::`,
`team::`, `user::`, `app::` — which implies stdlib names are three-segment
(`system::json::parse`). The code already contradicts this: `time::now` and
`id::uuid` in `stdlib/namespace_helpers.go` are two-segment.

SPEC.md's namespace section is amended to state that tier prefixes govern host-
and user-registered functions, while stdlib topics are two-segment. This makes
the spec describe what the code has always done.

## Phase 1 — the pain clusters (53 functions)

Ships as additive `feat` commits. Target release: **1.3.0**.

### Null and default — 2, bare, `core.go`

| Function | Behavior |
|---|---|
| `coalesce(a, b, ...)` | First non-null argument; null if all are null. Variadic, min 1 arg. |
| `default(x, fallback)` | `fallback` when `x` is *blank*, else `x`. |

`default` reuses the existing `is_blank` semantics — null, empty or
whitespace-only string, empty array, empty object — so the pair covers null-only
(`coalesce`) and blank-or-empty (`default`) distinctly, and `default` behaves the
way authors already understand `is_blank` to behave.

### `path::` — 5, new file `path.go`

| Function | Behavior |
|---|---|
| `path::get(obj, path, default?)` | Value at path, else `default`, else null. |
| `path::set(obj, path, value)` | Copy of `obj` with path set. Intermediate objects created as needed. |
| `path::has(obj, path)` | Whether the path resolves. |
| `path::delete(obj, path)` | Copy of `obj` without that path. |
| `path::flatten(obj)` | Single-level object mapping leaf dotted-paths to values. |

All non-mutating; `set` and `delete` return copies rather than modifying input.

**Path syntax.** Dot-separated. Numeric segments index into arrays, so
`path::get(o, "items.0.name")` walks an array. There is no escaping mechanism:
keys that literally contain a dot are unreachable through these functions. This
is a documented limitation matching jq and lodash conventions, and it keeps the
cluster to a segment split rather than a path grammar.

### Objects — 5, bare, `objects.go`

| Function | Behavior |
|---|---|
| `deep_merge(a, b, ...)` | Recursive merge, later wins. Nested objects merged; arrays replaced, not concatenated. |
| `map_values(obj, fn)` | Apply `fn` to each value. |
| `map_keys(obj, fn)` | Apply `fn` to each key. |
| `from_entries(arr)` | Inverse of the existing `entries`. |
| `invert(obj)` | Values become keys. |

### Text — 18, bare, `text.go` and new `text_case.go`

Case (in `text_case.go`, sharing a word-splitting helper): `snake_case`,
`camel_case`, `pascal_case`, `kebab_case`.

Remainder (appended to `text.go`): `trim_start`, `trim_end`,
`trim_chars(s, chars)`, `index_of`, `last_index_of`, `count_occurrences`,
`left(s, n)`, `right(s, n)`, `char_at(s, i)`, `lines(s)`, `normalize_space(s)`,
`strip_prefix`, `strip_suffix`, `mask(s, keep_last?, char?)`.

`mask` defaults to `keep_last = 4` and `char = "*"`, so `mask("4111111111111111")`
yields `************1111`.

All operate on runes — see "Unicode consistency" below.

The case conversions move to their own file because 18 additions would push
`text.go` past 450 lines, and the four case functions are the group with a
shared helper.

### `regex::` — 7, new file `regex.go`

| Function | Behavior |
|---|---|
| `regex::test(s, pattern)` | Whether the pattern matches. |
| `regex::find(s, pattern)` | First match, or null. |
| `regex::find_all(s, pattern)` | All matches. |
| `regex::replace(s, pattern, repl)` | Replace all, with `$1` group expansion. |
| `regex::split(s, pattern)` | Split on matches. |
| `regex::groups(s, pattern)` | Named and numbered captures of the first match, as an object. |
| `regex::escape(s)` | Quote regex metacharacters. |

**`match_regex` relationship.** The existing `match_regex(s, pattern)`
(`stdlib/text.go`) already returns all matches — it *is* `regex::find_all` under
an older name. Both names route to one shared implementation. `match_regex`
keeps working and is documented as legacy. There is no second regex code path
and no behavioral fork between the two spellings.

**Compiled-pattern cache.** `match_regex` currently calls `regexp.Compile` on
every invocation, so `map(rows, r => match_regex(r.name, "..."))` over 100k rows
compiles the same pattern 100k times. Adding seven regex functions multiplies
this, so the cache is part of Phase 1 rather than a follow-up.

Design: mutex-guarded LRU, capped at 256 entries, caching compile *failures* as
well as successes. The cap is required, not an optimization — patterns can be
constructed from data at runtime, so an unbounded map would grow without limit
on adversarial or merely generated input. Negative caching stops invalid
patterns from recompiling on every call.

Concurrency: `registry` is `sync.RWMutex`-guarded and builtins are shared across
goroutines, so the cache must be race-safe. Covered by `make test-race`.

### `json::` — 3, new file `json.go`

| Function | Behavior |
|---|---|
| `json::parse(s)` | Parse to a DTL value; error on invalid input. |
| `json::stringify(v, indent?)` | Serialize to a string. `indent` is an int count of spaces; omitted or `0` produces compact output. |
| `json::is_valid(s)` | Whether `s` parses. |

**Integer preservation.** `json::parse` uses `Decoder.UseNumber` and converts
integral values to `int64`. Plain `encoding/json` yields `float64` for every
number, which would silently turn every integer in a payload into a float —
visible to users through `type_of` and through arithmetic behavior. DTL has
distinct `int64` and `float64` runtime types and must preserve the distinction.

**Depth cap.** Parsing enforces a nesting-depth limit (default 64) via token
streaming. The executor's only limits are `Timeout` and `MaxDepth`
(`executor/executor.go`), neither of which bounds a parser, so deeply nested
untrusted input would otherwise be unconstrained.

### `encoding::` — 8, new file `encoding.go`

`base64_encode`, `base64_decode`, `base64url_encode`, `base64url_decode`,
`hex_encode`, `hex_decode`, `url_encode`, `url_decode`.

### `hash::` — 5, new file `hash.go`

`sha256`, `sha512`, `sha1`, `md5`, `crc32`.

`md5`, `sha1`, and `crc32` are included for interop — matching legacy checksums
and third-party API signatures is a genuine transformation need. Their `Doc`
strings state plainly that they are unsuitable for security purposes. Their
implementations carry `#nosec G401`/`G501` annotations with a stated rationale,
consistent with the existing suppression style in `stdlib/math.go`.

## Phase 2 — parity fill (46 functions)

Ships after Phase 1, also additive.

**Collections** (17, bare): `flat_map`, `partition`, `index_by`, `count_by`,
`sum_by`, `min_by`, `max_by`, `avg_by`, `compact`, `slice`, `concat`,
`intersection`, `union`, `difference`, `windows`, `pluck`, `unzip`.

**Math** (13, bare): `exp`, `mod`, `trunc`, `gcd`, `lcm`, `hypot`, `log2`, `sin`,
`cos`, `tan`, `atan2`, `is_nan`, `is_finite`.

**Datetime** (10, bare): `dt_parse`, `to_unix`, `from_unix`, `day_of_year`,
`iso_week`, `dt_in_zone`, `is_before`, `is_after`, `is_between`,
`duration_between`.

**Stats** (6, bare): `mode`, `quantile`, `covariance`, `cv`, `sum_squares`,
`linreg`.

### Phase 2 constraints

**No duration type.** DTL has no runtime duration value. `durationUnit` in
`stdlib/datetime.go` builds a `time.Duration` internally, but `dt_add` takes an
int and a unit string and returns a `time.Time` — no duration ever reaches DTL,
and `type_of` has no case for one. Accordingly `duration_between` returns a
plain number in a unit the caller names — fractional, so 36 hours is 1.5 days —
rather than implying a type that does not exist.

*As built:* it returns `float` rather than the `int` seconds sketched here. A
fractional result is what distinguishes it from `diff`, which already returns
whole units; an integer version would have duplicated `diff` instead of
complementing it.

**`dt_in_zone` needs tzdata.** `time.LoadLocation` depends on system zone data,
which is absent in scratch containers. Implementation must decide between
importing `time/tzdata` (embeds ~450KB into every binary that links DTL) and
returning a clear error when zone data is unavailable.

*As built:* it returns an error naming the zone and pointing at
`import _ "time/tzdata"`. Embedding 450KB into every binary that links DTL is
the host's decision to make, not the language's, and a silent fall back to UTC
would be a wrong answer that looks like a right one.

## Cross-cutting changes

### Builtin documentation — one source of truth

`executor.BuiltinFunc` carries `Name`, `MinArgs`, `MaxArgs`, `Fn`, `CtxFn` — no
documentation field. The LSP therefore keeps a parallel hand-written map in
`lang/helpers.go`, which has already drifted: it documents `dataset::query` and
`http::get`, which do not exist in this repository, while omitting dozens of
functions that do.

Change: add `Doc string` to `executor.BuiltinFunc`. `lang/helpers.go` reads the
registry first and falls back to its map. Every construction site uses named
struct fields, so this is backward-compatible for external embedders — and it
lets host-registered builtins document themselves, which they cannot today.

**Scope.** The stdlib `register()` helper is changed to require a doc string.
There are 160 existing call sites, of which `lang/helpers.go` documents roughly
70 — so this includes writing roughly 90 doc strings for functions that have
never had any, and deleting the stale `dataset::`/`http::` entries.

**Sequencing.** This lands as its own commit *before* any new functions, so
Phase 1 arrives into a library where docs are already mandatory and the guard
test already passes.

### Unicode consistency

The existing library is rune-based except in one place. Verified by probe:

```
len("café")              => 5        ← bytes
substr("café", 0, 3)     => "caf"    ← runes
pad_left("café", 6, "-") => "--café" ← runes
```

`substr`, `pad_left`, and `reverse_text` all convert to `[]rune`. Only `len`
(`stdlib/core.go`) returns `len(v)`, a byte count.

This matters for Phase 1 because `index_of` must return either a rune index or a
byte offset. Go's `strings.Index` returns bytes; if `index_of` did too, then
`substr(s, index_of(s, "é"))` would silently produce garbage on non-ASCII input.

**Decision.** All new text functions are rune-based, so they compose correctly
with `substr` and with each other. Separately, `len` is corrected to count runes.

### Versioning and sequencing

`.releaserc.json` runs semantic-release with `breaking: true → major`, and the
repository is at 1.2.1. Correcting `len` is a breaking change that would cut
2.0.0.

**Decision.** The `len` fix is held back and lands *last*, as its own isolated
major release, decoupled from the expansion. Phase 1 and Phase 2 ship as purely
additive `feat` commits (1.3.0 onward). New text functions are rune-based from
birth, so they compose correctly among themselves regardless of when `len` is
fixed.

> **As built — this decision was wrong, and the repository now forbids it.**
>
> A `v2.0.0` tag on this module is not installable. Go requires major version 2
> and above to carry a matching path suffix, so `go get` against a `v2.x.y` tag
> here fails with *"module path must match major version
> (`github.com/xraph/dtl/v2`)"*. Cutting a major is therefore not a release
> decision that can be made on its own — it requires renaming the module and
> every import in the same change.
>
> The plan above reasoned only from `releaseRules`, which did say
> `breaking: true → major`. That was the right reading of the config and the
> wrong reading of the situation: the config could authorise a tag the module
> path could not support.
>
> `releaseRules` now maps `breaking: true → minor`, and README.md documents why.
> Breaking changes stay on the `v1` line, keep their `⚠ BREAKING CHANGES`
> changelog entry and migration note, and only the version arithmetic differs.
> The `len` fix shipped in **1.5.0** on exactly those terms.

Consequence, accepted knowingly: between Phase 1 and the `len` fix, `index_of`
and `len` disagree on non-ASCII strings. This is the pre-existing state of the
library — `substr` already disagrees with `len` today — so the window introduces
no new inconsistency, it merely fails to close an old one yet.

Commit order:

1. `Doc` field, 160-site backfill, LSP registry wiring, guard test.
2. Phase 1 clusters, one commit per cluster.
3. Phase 2 clusters, one commit per cluster.
4. `len` rune fix, with a `BREAKING CHANGE:` footer. Under the corrected rule
   this resolves to a minor bump; it shipped in 1.5.0.

## Testing

- Table-driven tests matching the existing style in `stdlib/stdlib_test.go`, one
  table per cluster.
- Round-trip test pinning integer preservation through
  `json::parse` → `json::stringify`.
- Guard test asserting every registered stdlib name carries a non-empty `Doc`.
- `make test-race` for the regex cache.
- Non-ASCII cases for every new text function.
- `make verify` (fmt-check, vet, tidy-check, lint) and `make security` clean,
  including the `#nosec` rationales in `hash.go`.

## Documentation updates

- `SPEC.md`: amend the namespace section per "SPEC.md amendment" above; extend
  the Standard Library section with the new namespaces.
- `README.md`: the stdlib row in the "What's in the box" table gains the new
  topics.

## Out of scope

**The type-list contradiction.** Auditing turned up that DTL's type annotations
are unenforced: `parser.go`'s `parseType` accepts any identifier, verified by
probe — `fn h(x: banana) -> zebra => x` compiles and runs. Four different type
lists exist across `SPEC.md` (9 entries), `lang/lang.go` (7), the compiler's
`normalizeType` (6 spellings, the only list with behavioral effect), and
`type_of` (8 runtime types), and no two agree.

Whether to close the type set or declare annotations advisory is a language
design decision with its own breaking-change implications. It is deliberately
not folded into this work, and needs its own conversation.

**Retrofitting `system::` aliases.** Existing aliases stay as they are. No
backfill, no deprecation, no renaming of the existing inconsistent set.

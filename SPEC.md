# DTL Language Specification

DTL — Data Transformation Language — is a small, embeddable expression and function
language. This document defines the language itself: its syntax, semantics,
type system, and pure standard library.

It does **not** define what a DTL program can reach outside the interpreter.
Datastores, HTTP, message buses and the like are supplied by the embedding
host as registered builtins under its own namespaces — see the Embedding
section of [README.md](README.md). Examples below that call such a builtin are
marked as host-provided.

## Design Goals

1. **Readable by non-programmers** — Python-like, no semicolons, no braces for simple functions
2. **Powerful for developers** — type annotations, pattern matching, error handling, multi-line logic
3. **Shareable** — functions have namespaces, versions, and stable fully-qualified names
4. **Composable** — functions call other functions; expressions embed anywhere the host allows
5. **Safe** — no side effects by default, sandboxed execution, depth and timeout limits

## Syntax at a glance

### One-liner

```
fn celsius_to_fahrenheit(temp: float) -> float =>
    temp * 1.8 + 32
```

### Multi-line

```
fn classify_temperature(temp: float, unit: string = "celsius") -> string:
    let normalized = if unit == "fahrenheit" then (temp - 32) / 1.8 else temp

    match normalized:
        when < 0    => "freezing"
        when < 15   => "cold"
        when < 25   => "comfortable"
        when < 35   => "warm"
        when >= 35  => "hot"
```

### Collection operations

```
fn anomaly_score(values: float[], window_size: int = 10) -> float:
    let recent = values | tail(window_size)
    let mean = recent | avg()
    let std = recent | stdev()
    let latest = values | last()

    if std == 0 then 0.0
    else abs(latest - mean) / std
```

---

## Language Reference

### 1. Function Declaration

```
fn <name>(<params>) -> <return_type>:
    <body>

-- OR one-liner --

fn <name>(<params>) -> <return_type> => <expression>
```

**Parameters**:
```
name: type                    -- required parameter
name: type = default_value    -- optional with default
...name: type                 -- variadic (must be last)
```

**Type annotations are advisory.** They document intent and drive editor
completion; they are not enforced. The parser accepts any identifier in type
position, and the compiler recognises a handful of names for light inference
while leaving the rest unchecked. Nothing is rejected for having an unfamiliar
type, so an annotation is a note to the reader rather than a constraint on the
caller.

The type names DTL itself understands are the ones `type_of` reports:

| Name | Example |
|------|---------|
| `null` | `null` |
| `bool` | `true` |
| `int` | `42` |
| `float` | `1.5` |
| `string` | `"text"` |
| `array` | `[1, 2, 3]` |
| `object` | `{a: 1}` |
| `datetime` | `now()` |

Conventional annotations are `string`, `int`, `float`, `bool`, `datetime`,
`object`, and `any`, plus array variants written with a suffix — `string[]`,
`float[]`, `object[]`. A `record { ... }` type may be written inline or bound
with `type`.

There is no duration value. `dt_add` takes an amount and a unit name and
returns a datetime, and `duration_between` returns a number in a named unit —
neither produces a distinct duration type.

### 2. Variables

```
let x = 42                     -- immutable binding
let name = "sensor_01"
let values = [1, 2, 3, 4, 5]
```

Variables are **immutable** — once bound, they cannot be reassigned. This ensures functions have no side effects and are safe to parallelize.

### 3. Conditionals

```
-- Inline if
let status = if temp > 80 then "hot" else "ok"

-- Multi-line if
let category =
    if score > 90 then "excellent"
    else if score > 70 then "good"
    else if score > 50 then "average"
    else "poor"
```

### 4. Pattern Matching

```
match value:
    when < 0       => "negative"
    when 0         => "zero"
    when 1..10     => "small"
    when 11..100   => "medium"
    when > 100     => "large"

match status_code:
    when "OK"      => 0
    when "WARN"    => 1
    when "ERROR"   => 2
    when _         => -1      -- default/wildcard
```

### 5. Pipe Operator

The pipe operator `|` chains transformations left-to-right. This is the core ergonomic feature that makes functions readable.

```
-- Instead of: round(avg(filter(values, x => x > 0)), 2)
-- Write:
values | filter(> 0) | avg() | round(2)

-- Pipes also chain over whatever the host exposes. If an embedder registers
-- a `query` builtin returning a row list, it composes the same way:
query("sensor_readings")
    | where timestamp > $start_date
    | where sensor_id == "s1"
    | select temperature, humidity
    | order_by timestamp desc
    | limit 100
```

### 6. Collection Operations

These work on arrays and are designed to pipe naturally:

```
values | map(x => x * 2)              -- transform each element
values | filter(x => x > threshold)   -- keep matching elements
values | filter(> 0)                  -- shorthand: implicit argument
values | reduce(0, (acc, x) => acc + x)  -- fold
values | sort()                        -- ascending
values | sort(desc)                    -- descending
values | tail(10)                      -- last N elements
values | head(5)                       -- first N elements
values | unique()                      -- deduplicate
values | flatten()                     -- flatten nested arrays
values | zip(other_values)             -- pair up two arrays
values | group_by(x => x.category)     -- group into {key: [values]}
values | chunk(5)                      -- split into groups of N
```

### 7. Aggregation Functions

```
values | sum()
values | avg()
values | min()
values | max()
values | count()
values | stdev()
values | variance()
values | median()
values | percentile(95)
values | count_where(> 100)            -- count matching a condition
values | sum_where(> 0)                -- sum matching a condition
```

### 8. String Operations

```
name | upper()                         -- "HELLO"
name | lower()                         -- "hello"
name | trim()                          -- strip whitespace
name | replace("old", "new")
name | split(",")                      -- → string[]
parts | join(", ")                     -- → string
name | starts_with("pre")             -- → bool
name | contains("sub")                -- → bool
name | substr(0, 5)                   -- first 5 chars
"Hello {name}, temp is {temp | round(1)}"  -- string interpolation
```

### 9. Date/Time Operations

```
now()                                  -- current datetime
today()                                -- current date
dt | dt_add(7, "days")                 -- advance by 7 days
dt | dt_subtract(1, "hours")
dt | diff(other_dt, "minutes")         -- difference in minutes
dt | format("YYYY-MM-DD")
dt | year()
dt | month()
dt | day()
dt | hour()
dt | day_of_week()                     -- 0=Sunday
dt | start_of("month")                -- first moment of month
dt | end_of("week")                   -- last moment of week
```

### 10. Math Operations

```
abs(x)
round(x, decimals)
ceil(x)
floor(x)
power(x, n)
sqrt(x)
log(x)
log10(x)
clamp(x, min, max)                    -- constrain to range
lerp(a, b, t)                         -- linear interpolation
```

### 11. Error Handling

```
-- try/catch for graceful degradation
let result = try some_function(x) catch default_value

-- Explicit null handling
let safe_value = value ?? default_value     -- null coalescing
let safe_result = value?.nested?.field      -- optional chaining
```

### 12. Comments

```
-- single line comment

{- 
   multi-line 
   comment 
-}
```

### 13. Type Casting

```
value | as_float()
value | as_int()
value | as_string()
value | as_bool()
value | as_datetime("YYYY-MM-DD")
```

---


### Namespaces

`::` separates namespace segments. It serves two distinct purposes.

**Standard library topics are two-segment**, naming a capability area:

```
json::parse
regex::replace
encoding::base64_encode
hash::sha256
path::get
time::now
id::uuid
```

**Host- and user-registered functions may use a leading tier segment** for
organization and access control. The tier is the host's convention, not
something the language enforces:

```
shared::analytics::anomaly   -- shared across all users
team::data_eng::normalize    -- team-scoped
user::jane::my_helper        -- private to a user
app::iot_connector::decode   -- provided by an app
```

A small set of `system::`-prefixed spellings — `system::math::clamp`,
`system::text::title_case` and similar — remain registered as aliases of their
bare counterparts. They are supported for compatibility and are not the pattern
new functions follow.


## Standard Library

The standard library is domain-neutral and always registered. Anything reaching
outside the interpreter — a datastore, an HTTP client, a message bus — is
registered by the host under its own namespace.

Names below are the callable spellings. Every builtin also carries its own
signature and description, which the language server reads directly from the
registration table rather than from a separate list.

### Core

`len` `type_of` `is_null` `is_blank` `coalesce` `default` `to_string` `abs`
`round` `ceil` `floor` `power` `sqrt` `log` `log10` `log2` `exp` `mod` `trunc`
`gcd` `lcm` `hypot` `sin` `cos` `tan` `atan2` `is_nan` `is_finite` `sign`
`clamp` `lerp` `normalize` `random` `random_int` `now` `today`

`coalesce(a, b, ...)` returns the first argument that is not null.
`default(x, fallback)` substitutes when `x` is *blank* — null, empty or
whitespace-only string, empty array, or empty object.

### Text

`upper` `lower` `trim` `trim_start` `trim_end` `trim_chars` `normalize_space`
`replace` `split` `join` `lines` `starts_with` `ends_with` `contains`
`index_of` `last_index_of` `count_occurrences` `strip_prefix` `strip_suffix`
`substr` `left` `right` `char_at` `truncate` `pad_left` `pad_right` `repeat`
`reverse_text` `capitalize` `title_case` `snake_case` `camel_case`
`pascal_case` `kebab_case` `slugify` `word_count` `extract_number` `mask`

Text functions measure and index in **characters, not bytes**, so
`substr(s, index_of(s, x))` composes correctly on non-ASCII input.

### Collections

`map` `flat_map` `filter` `reduce` `partition` `sort` `sort_by` `reverse`
`unique` `distinct_by` `group_by` `index_by` `count_by` `chunk` `windows`
`flatten` `compact` `slice` `concat` `zip` `unzip` `pluck` `head` `tail`
`first` `last` `take_while` `drop_while` `find` `find_index` `includes`
`every` `some` `range` `seq` `top_n` `intersection` `union` `difference`

### Aggregation and statistics

`sum` `avg` `min` `max` `count` `count_where` `sum_where` `sum_by` `avg_by`
`min_by` `max_by` `median` `mode` `percentile` `quantile` `stdev` `variance`
`cv` `sum_squares` `covariance` `correlation` `linreg` `z_score`
`outlier_bounds` `histogram` `moving_avg` `ewma`

`percentile` takes 0-100; `quantile` takes a fraction from 0 to 1.

### Objects

`keys` `values` `entries` `from_entries` `merge` `deep_merge` `map_values`
`map_keys` `invert` `pick` `omit` `has_key`

`merge` overwrites at the top level; `deep_merge` recurses into nested objects
and replaces arrays rather than concatenating them.

### `path::` — nested access

`path::get(obj, path, default?)` `path::has` `path::set` `path::delete`
`path::flatten`

Paths are dot-separated, and a numeric segment indexes an array:
`path::get(o, "items.0.name")`. Keys containing a literal dot are not
addressable this way. `path::set` and `path::delete` return copies; nothing is mutated.

### `regex::`

`regex::test` `regex::find` `regex::find_all` `regex::replace` `regex::split`
`regex::groups` `regex::escape`

Patterns are RE2, so matching is linear in the input with no catastrophic
backtracking. Compiled patterns are cached. `match_regex` is a legacy spelling
of `regex::find_all`.

### `json::`

`json::parse` `json::stringify` `json::is_valid`

Whole numbers survive a parse as integers rather than becoming floats. Parsing
enforces a nesting-depth limit.

### `encoding::`

`encoding::base64_encode` `encoding::base64_decode` `encoding::base64url_encode`
`encoding::base64url_decode` `encoding::hex_encode` `encoding::hex_decode`
`encoding::url_encode` `encoding::url_decode`

Decoders report malformed input as an error rather than returning empty.

### `hash::`

`hash::sha256` `hash::sha512` `hash::sha1` `hash::md5` `hash::crc32`

`hash::sha1`, `hash::md5` and `hash::crc32` are provided for interop with
systems that already use them. They are **not suitable for security
purposes**.

### Date and time

`now` `today` `time::now` `dt_add` `dt_subtract` `dt_format` `dt_parse` `diff`
`duration_between` `to_unix` `from_unix` `year` `month` `day` `hour` `minute`
`second` `day_of_week` `day_of_year` `iso_week` `start_of` `end_of`
`time_bucket` `dt_in_zone` `is_before` `is_after` `is_between`
`is_business_day` `business_days_between`

Unit names are case-insensitive and accept singular or plural, so `"Days"`,
`"days"` and `"day"` are the same unit. **An unrecognised unit is an error.**
`diff` returns whole units; `duration_between` returns a fractional count.

### Casting and formatting

`as_int` `as_float` `as_string` `as_bool` `as_datetime` (also spelled `to_int`,
`to_float`, `to_bool`, `to_date`, `to_datetime`) `format_number`
`format_currency` `format_percent`

### Identity

`id::uuid` `id::slug`

---


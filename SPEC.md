# DTL Language Specification

DTL is a small, embeddable expression and function
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

**Supported types**: `string`, `int`, `float`, `bool`, `datetime`, `duration`, `object`, `any`, `void`
Plus array variants: `string[]`, `float[]`, `object[]`, etc.

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
dt | add(7, "days")                    -- add duration
dt | subtract(1, "hours")
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

Functions live in namespaces for organization and access control:

```
system::math::clamp          -- built-in system functions
shared::analytics::anomaly   -- shared across all users
team::data_eng::normalize    -- team-scoped
user::jane::my_helper        -- private to a user
app::iot_connector::decode   -- provided by an app
```


## Standard Library

The following function namespaces should be pre-loaded in `extensions/function/stdlib/`:

### `system::math`

```
fn clamp(value: float, min: float, max: float) -> float =>
    if value < min then min else if value > max then max else value

fn lerp(a: float, b: float, t: float) -> float =>
    a + (b - a) * t

fn normalize(value: float, min: float, max: float) -> float =>
    if max == min then 0.0 else (value - min) / (max - min)

fn moving_avg(values: float[], window: int) -> float =>
    values | tail(window) | avg()

fn ewma(values: float[], alpha: float = 0.3) -> float:
    values | reduce(values | first(), (acc, v) => alpha * v + (1 - alpha) * acc)
```

### `system::text`

```
fn slugify(text: string) -> string =>
    text | lower() | trim() | replace(" ", "-") | replace("[^a-z0-9-]", "")

fn truncate(text: string, max_len: int, suffix: string = "...") -> string =>
    if text | len() <= max_len then text
    else (text | substr(0, max_len - (suffix | len()))) ++ suffix

fn extract_number(text: string) -> float =>
    text | match_regex("[0-9]+\\.?[0-9]*") | first() | as_float()
```

### `system::datetime`

```
fn business_days_between(start: datetime, end: datetime) -> int:
    let days = start | diff(end, "days") | as_int()
    let weeks = days / 7
    let remaining = days % 7
    weeks * 5 + (remaining | clamp(0, 5))

fn is_business_day(dt: datetime) -> bool =>
    dt | day_of_week() >= 1 && dt | day_of_week() <= 5

fn time_bucket(dt: datetime, interval: string) -> datetime =>
    dt | start_of(interval)
```

### `system::stats`

```
fn z_score(value: float, values: float[]) -> float:
    let mean = values | avg()
    let std = values | stdev()
    if std == 0 then 0.0
    else (value - mean) / std

fn outlier_bounds(values: float[], factor: float = 1.5) -> object:
    let q1 = values | percentile(25)
    let q3 = values | percentile(75)
    let iqr = q3 - q1
    return { lower: q1 - factor * iqr, upper: q3 + factor * iqr }

fn correlation(x: float[], y: float[]) -> float:
    let n = x | count()
    let sum_xy = x | zip(y) | map((a, b) => a * b) | sum()
    let sum_x = x | sum()
    let sum_y = y | sum()
    let sum_x2 = x | map(v => v * v) | sum()
    let sum_y2 = y | map(v => v * v) | sum()
    (n * sum_xy - sum_x * sum_y) /
        sqrt((n * sum_x2 - sum_x * sum_x) * (n * sum_y2 - sum_y * sum_y))
```

### `system::collections`

```
fn top_n(values: any[], n: int, key: string = "") -> any[]:
    if key == "" then values | sort(desc) | head(n)
    else values | sort_by(key, desc) | head(n)

fn histogram(values: float[], bins: int = 10) -> object[]:
    let min_val = values | min()
    let max_val = values | max()
    let bin_width = (max_val - min_val) / bins
    values
        | map(v => ((v - min_val) / bin_width) | floor() | clamp(0, bins - 1) | as_int())
        | group_by(bin => bin)
        | map((bin, items) => {
            bin: bin,
            range_start: min_val + bin * bin_width,
            range_end: min_val + (bin + 1) * bin_width,
            count: items | count()
          })
```

---


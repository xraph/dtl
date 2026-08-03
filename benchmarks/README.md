# DTL comparative benchmarks

How DTL performs against five widely embedded expression and scripting
languages for Go. The point of this suite is calibration, not marketing — the
workloads include the ones DTL loses, and it loses several.

## Running

```bash
cd benchmarks && go test -run '^$' -bench . -benchmem
```

Parity first — the timings mean nothing without it:

```bash
cd benchmarks && go test -run TestParity -v
```

## Why this is a separate module

`benchmarks/` has its own `go.mod`. DTL's root module depends on exactly one
package (`github.com/google/uuid`), and that is worth protecting: a test-only
dependency of the root module still lands in consumers' module graph. Six
comparison engines pull in ANTLR, protobuf, `golang.org/x/exp` and more. A
nested module keeps every one of them out of `go get github.com/xraph/dtl`.

It uses `replace github.com/xraph/dtl => ../`, so it always benchmarks the
working tree, not a published tag.

## Engines

| Engine | Kind |
|---|---|
| **DTL** | tree-walking interpreter |
| [expr](https://github.com/expr-lang/expr) | bytecode VM |
| [cel-go](https://github.com/google/cel-go) | Common Expression Language |
| [goja](https://github.com/dop251/goja) | JavaScript interpreter |
| [gopher-lua](https://github.com/yuin/gopher-lua) | Lua 5.1 VM |
| [starlark-go](https://go.starlark.net) | Starlark (Python-like) |

## Methodology

**Compile once, evaluate many.** Every engine here separates compilation from
evaluation, and every host embeds them that way: compile rules at load,
evaluate per request. `BenchmarkEval` times only evaluation of an
already-compiled program. Timing compile+eval together would measure something
nobody runs in a hot path — and would flatter whichever engine parses fastest.

**Parity is enforced, not assumed.** `TestParity` compiles and runs every
workload on every engine and asserts the results match after normalization.
Comparing programs that compute different things is the most common way these
benchmarks go wrong. The test fails the build if a spelling drifts.

**No integer fast paths.** All numeric inputs are `float64`, so no engine wins
on an integer path another one lacks.

**n/a is a real answer.** The `collection` workload has no cel-go column. Base
CEL has `filter` but no fold or sum, and cel-go ships no standard sum
extension. Emulating one with a nested comprehension would measure a different
program, so it is reported as n/a rather than bent to fit.

### The one asymmetry worth knowing

gopher-lua and starlark-go need Go values marshalled into VM-native values
(`lua.LTable`, `starlark.Dict`) on **every call**. DTL, expr and cel-go consume
`map[string]any` directly and pay nothing.

This is a genuine architectural advantage of the latter three, and a real cost
an embedder pays for the former two — so it is left in. But it is why
gopher-lua's `field` result (2710 ns, 9.9 KB) is an outlier: almost all of it
is rebuilding a three-level nested table per call, not traversing it. Read that
cell as marshalling cost, not interpreter speed.

## Results

Apple M3 Max, Go 1.26.3, darwin/arm64. `ns/op` — lower is better.
Best in each row is **bold**.

### Evaluation (steady state)

| workload | DTL | expr | cel-go | goja | gopher-lua | starlark |
|---|--:|--:|--:|--:|--:|--:|
| arith | 197 | 107 | 126 | 155 | **92** | 313 |
| cond | 165 | 63 | 132 | 161 | **74** | 627 |
| string | 328 | **157** | 235 | 403 | 285 | 487 |
| field | 146 | 148 | **85** | 424 | 2733 ◆ | 767 |
| collection | 11113 | **5038** | n/a | 11536 | 5153 | 6342 |

◆ dominated by per-call marshalling, see above.

Allocations per evaluation:

| workload | DTL | expr | cel-go | goja | gopher-lua | starlark |
|---|--:|--:|--:|--:|--:|--:|
| arith | 264 B / 5 | 120 B / 5 | **40 B / 5** | 160 B / 2 | 16 B / 2 | 448 B / 14 |
| cond | 240 B / 2 | 32 B / 1 | 24 B / 3 | 128 B / 3 | **8 B / 1** | 744 B / 28 |
| collection | 21760 B / 116 | **3784 B / 56** | n/a | 7824 B / 115 | 7512 B / 104 | 3312 B / 151 |

### Compilation (cold start)

Each iteration builds a fresh isolated environment *and* compiles — `registry.New`,
`cel.NewEnv`, `lua.NewState` respectively. That is the like-for-like shape, but
it hides how the cost splits. For DTL it splits sharply:

| step | ns/op | B/op | allocs |
|---|--:|--:|--:|
| `registry.New` (registers whole stdlib) | 29330 | 48232 | 316 |
| `Register` on a warm registry | **2315** | 4664 | 31 |
| combined (what the table below shows) | 29412 | 53144 | 350 |

| workload | DTL | expr | cel-go | goja | gopher-lua | starlark |
|---|--:|--:|--:|--:|--:|--:|
| arith | 29412 | 7881 | 79328 | 4422 | 85518 | **4355** |
| cond | 31033 | 10195 | 106171 | 6169 | 129344 | **6214** |
| collection | 33029 | 9367 | n/a | 9489 | 93778 | **6868** |

## Reading the results

**DTL is a tree-walking interpreter competing against bytecode VMs.** expr and
gopher-lua compile to bytecode; DTL walks the AST. On evaluation it is
still behind expr on every workload — around 2× on the small ones and on the
collection workload — though it now matches expr on `field` and has closed most
of the gap elsewhere.
That is the expected shape, not a defect — but it is the honest headline.

**Per-call overhead still sets the floor, but it is much lower than it was.**

```
BenchmarkDTLExecuteNoop    106 ns/op    240 B/op    2 allocs/op
```

That is `fn f(a: float) -> float => a` — a bare parameter reference, and the
minimum any DTL call can cost. It was 269 ns and 6 allocations before the
per-call overhead work; what remains is one allocation for the root scope and
one for the returned `ExecuteResult`.

The four that went away were all scaffolding rather than evaluation:

| allocation | why it is gone |
|---|---|
| scope map + its first bucket | scopes hold their bindings inline (`envInline`) |
| debug buffer | allocated on first `debug()`/`print()`, not per call |
| `context.WithValue` for that buffer | the sink is passed directly, not through the context |

`field` is the clearest illustration: 274 ns → 146 ns, now level with expr,
because that workload is almost entirely call overhead plus three map reads.

**Where DTL already wins.** Compiling a function into a warm registry costs
**2315 ns** — faster than expr (7881), cel-go (79328) and gopher-lua (85518).
DTL's front end is genuinely quick; the cost is concentrated in
`registry.New`'s stdlib registration and in per-call evaluation overhead.

**What this suite does not measure.** Concurrency, memory under sustained load,
deep recursion, or large-input scaling. It also does not measure the things
DTL is actually chosen for — readable syntax, static type checking at
registration, and a capability hook. Those do not show up in `ns/op`.

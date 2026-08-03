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

Apple M3 Max, Go 1.26.3, darwin/arm64. `ns/op` — lower is better, best of
three runs. Treat differences under ~15% as noise: run-to-run variance on a
laptop moved some engines by 2× between runs while the code under test was
identical.
Best in each row is **bold**.

### Evaluation (steady state)

| workload | DTL | expr | cel-go | goja | gopher-lua | starlark |
|---|--:|--:|--:|--:|--:|--:|
| arith | 221 | 109 | 130 | 164 | **92** | 338 |
| cond | 172 | **63** | 133 | 167 | 75 | 645 |
| string | 337 | **161** | 231 | 397 | 293 | 503 |
| field | 162 | 156 | **115** | 461 | 2843 ◆ | 814 |
| collection | **4106** | 5186 | n/a | 12144 | 5310 | 6496 |

◆ dominated by per-call marshalling, see above.

Allocations per evaluation:

| workload | DTL | expr | cel-go | goja | gopher-lua | starlark |
|---|--:|--:|--:|--:|--:|--:|
| arith | 280 B / 5 | 120 B / 5 | **40 B / 5** | 160 B / 2 | 16 B / 2 | 448 B / 14 |
| cond | 256 B / 2 | 32 B / 1 | 24 B / 3 | 128 B / 3 | **8 B / 1** | 744 B / 28 |
| collection | **2464 B / 11** | 3784 B / 56 | n/a | 7824 B / 115 | 7512 B / 104 | 3312 B / 151 |

### Compilation (cold start)

Each iteration builds a fresh isolated environment *and* compiles —
`registry.New`, `cel.NewEnv`, `lua.NewState` respectively. Best of three:

| workload | DTL | expr | cel-go | goja | gopher-lua | starlark |
|---|--:|--:|--:|--:|--:|--:|
| arith | **2075** | 6834 | 81184 | 4523 | 81971 | 4396 |
| cond | **2881** | 9811 | 101456 | 5857 | 89018 | 5979 |
| string | **2508** | 8165 | 84672 | 5394 | 89890 | 5086 |
| field | **2436** | 8445 | 79943 | 4391 | 85966 | 4736 |
| collection | **3199** | 9852 | n/a | 9802 | 99124 | 7303 |

DTL is the fastest here by 2–3× over the next engine, and 30× over cel-go and
gopher-lua, which build a substantial environment per call.

That was not true before the standard library became process-wide. Each
`registry.New` used to construct ~400 `BuiltinFunc` values:

| step | before | after |
|---|--:|--:|
| `registry.New` | 29330 ns / 48232 B / 316 allocs | **72 ns / 176 B / 3 allocs** |
| `Register` on a warm registry | 2315 ns / 4664 B / 31 | 2132 ns / 4664 B / 31 |

Nothing about a `BuiltinFunc` is per-registry — they close over no registry
state — so every registry was rebuilding an identical table. It is now built
once and shared, with host-registered builtins kept in a per-registry overlay
so that sharing cannot leak between them.

## Reading the results

**DTL is a tree-walking interpreter competing against bytecode VMs.** expr and
gopher-lua compile to bytecode; DTL walks the AST. On evaluation it is
behind expr on the small expressions — around 2× — where per-call overhead
dominates and a bytecode VM has less of it to pay. It matches expr on `field`
and is now fastest of the six on `collection`, where the work per call is
large enough for the evaluator itself to matter more than the call.
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

**Where DTL wins.** Cold start, by a wide margin — see the compile table
above. Its front end was always quick; what had hidden that was `registry.New`
rebuilding the whole standard library, which is now shared process-wide.

Also `collection`, which used to be its worst result at 20 µs and 320
allocations. A lambda's argument scope is now reused across elements instead
of allocated per element, and the higher-order functions no longer take a
clock reading per element — that call alone was 30% of the workload's CPU.
Both changes came out of a profile; neither touched the evaluator's design.

For a host that builds a registry per tenant or per request, that single
change is worth more than everything else here: 29 µs and 316 allocations per
registry became 72 ns and 3.

**What this suite does not measure.** Concurrency, memory under sustained load,
deep recursion, or large-input scaling. It also does not measure the things
DTL is actually chosen for — readable syntax, static type checking at
registration, and a capability hook. Those do not show up in `ns/op`.

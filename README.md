# DTL — Data Transformation Language

A small, embeddable programming language for expressions and user-defined
functions, written in Go. DTL is designed to be readable by analysts and
engineers alike, safe to evaluate on untrusted input, and easy to embed in a
host application that supplies its own domain vocabulary.

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

See [SPEC.md](SPEC.md) for the full language definition.

> **On the name.** DTL is *Data Transformation Language*. It transforms data
> the host hands it and assumes no domain of its own — the companion to DQL,
> Data Query Language: one queries, one transforms.

## Install

```bash
go get github.com/xraph/dtl
```

## Evaluate a function

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/xraph/dtl/registry"
)

func main() {
	reg := registry.New(registry.Config{})

	if err := reg.Register("double", "fn double(x: float) -> float => x * 2"); err != nil {
		log.Fatal(err)
	}

	res, err := reg.Execute(context.Background(), "double", map[string]any{"x": 21.0})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(res.Value) // 42
}
```

## What's in the box

| Package | Purpose |
|---------|---------|
| `lexer`, `parser`, `ast` | Front end — tokens, grammar, typed syntax tree |
| `compiler` | Resolves and type-checks a function against a registry |
| `executor` | Tree-walking evaluator with depth and timeout limits |
| `stdlib` | Pure standard library — collections, text, math, stats, time, casting, plus `path::` `regex::` `json::` `encoding::` `hash::` |
| `registry` | Compiled-function cache, builtin table, execution entry point |
| `capability` | Hook for narrowing what a function may do (see below) |

## Embedding

DTL ships a **domain-neutral** standard library. Anything that reaches outside
the interpreter — a datastore, an HTTP client, a message bus — is registered by
you, under your own namespace, so the language stays free of any one host's
vocabulary:

```go
reg.RegisterBuiltin("store::get", &executor.BuiltinFunc{
	Name: "store::get", MinArgs: 1, MaxArgs: 1,
	CtxFn: func(ctx context.Context, args []any) (any, error) {
		return myStore.Get(ctx, args[0].(string))
	},
})
```

### Confining what a function may do

DTL has no notion of who owns a function or what it is allowed to touch — that
is your model. The `capability` package gives you the hook; you supply the
meaning. The executor calls `capability.Enter` as execution crosses into each
user-defined function:

```go
ctx = capability.WithInterceptor(ctx, func(ctx context.Context, fn string) context.Context {
	// Re-scope authority for the function about to run. Return ctx unchanged
	// to leave the caller's authority in place.
	return withAuthorityFor(ctx, ownerOf(fn))
})
```

With no interceptor installed the gate is inert and you get a plain
interpreter — absence means unrestricted, deliberately.

### Execution limits

`Config.MaxCallDepth` bounds nested user-function calls. Left at zero it
selects `registry.DefaultMaxCallDepth` (1000), which is deliberate: unbounded
recursion exhausts the goroutine stack, and Go raises that as
`fatal error: stack overflow` — not a panic, so `recover()` cannot catch it and
the host process dies. A negative value disables the limit and restores that
exposure; set it only for input you control.

`Config.DefaultTimeout` bounds wall-clock time per execution and is **not**
defaulted — zero means no timeout. A limit that aborts a legitimately slow
transformation is a policy only you can set, so if you evaluate untrusted
source, set one:

```go
reg := registry.New(registry.Config{
	DefaultTimeout: 100 * time.Millisecond,
})
```

Both limits apply inside lambdas passed to `map`, `filter`, `reduce` and the
rest of the higher-order library: a lambda carries the depth and deadline in
force where it was written. Before v1.5.4 it did not, and recursion routed
through a collection function could exhaust the stack despite `MaxCallDepth`
being set.

Neither limit constrains a builtin you register yourself; those run your Go
code, and bounding it is yours to do.

## Status

DTL has been in production use as an embedded language before being published
here as a standalone project. The language and its specification are frozen
across that move — behavior is unchanged, and the git history predates it.

### Versioning

Releases stay on the `v1` line. A conventional-commit `!` or `BREAKING CHANGE:`
footer therefore resolves to a **minor** bump, not a major one — see
`releaseRules` in `.releaserc.json`.

That is deliberate, and it is a property of the module path rather than a
statement about stability. Go requires major version 2 and above to carry a
matching path suffix (`github.com/xraph/dtl/v2`). Until that migration happens,
a `v2.x.y` tag on this repository is not installable at all:

```
go: github.com/xraph/dtl@v2.0.0: invalid version: module contains a go.mod
    file, so module path must match major version ("github.com/xraph/dtl/v2")
```

So cutting a major is not a release decision that can be made on its own — it
requires renaming the module and every import in the same change. Coupling the
two here means a stray `!` in a commit message cannot silently publish a tag no
one can install. Breaking changes still get an entry in `CHANGELOG.md` under
"⚠ BREAKING CHANGES" and a migration note; only the version arithmetic differs.
When a genuine `v2` is warranted, migrate the path and restore
`"release": "major"` in the same pull request.

## Editor support

Highlighting and language intelligence both ship with the language, so an
editor needs no bespoke client code:

| Want | Use |
|------|-----|
| Syntax highlighting | [`syntaxes/`](syntaxes/) — TextMate grammar, scope `source.dtl` |
| Completion, hover, diagnostics | [`cmd/dtl-lsp`](cmd/dtl-lsp) — a Language Server Protocol server |
| To build your own | [`lang`](lang/) — the same features as plain functions |

```bash
go install github.com/xraph/dtl/cmd/dtl-lsp@latest
```

The server works on a file on disk with nothing else running. A host that knows
more — which datasets exist, which functions are registered — passes that in and
gets richer completions; without it, the language itself is still there.

## License

Apache License 2.0 — see [LICENSE](LICENSE), [NOTICE](NOTICE), and
[TRADEMARKS](TRADEMARKS).

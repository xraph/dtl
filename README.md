# DTL

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

> **On the name.** DTL is a name, not an acronym — there is no expansion you
> are missing. It is a general-purpose expression language; the host supplies
> the vocabulary of whatever domain it is embedded in.

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
| `stdlib` | Pure standard library — collections, text, math, stats, time, casting |
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

## Status

DTL has been in production use as an embedded language before being published
here as a standalone project. The language and its specification are frozen
across that move — behavior is unchanged, and the git history predates it.

## License

Apache License 2.0 — see [LICENSE](LICENSE), [NOTICE](NOTICE), and
[TRADEMARKS](TRADEMARKS).

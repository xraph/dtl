package registry

import (
	"context"
	"testing"
)

func BenchmarkRegistry_Execute_Arithmetic(b *testing.B) {
	reg := newTestRegistry()
	_ = reg.Register("bench::arith", `fn arith(a: float, b: float) -> float => a + b * 2 - 1`)
	ctx := context.Background()
	args := map[string]any{"a": 10.0, "b": 5.0}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := reg.Execute(ctx, "bench::arith", args)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRegistry_Execute_IfThenElse(b *testing.B) {
	reg := newTestRegistry()
	_ = reg.Register("bench::cond", `fn cond(x: float) -> string => if x > 0 then "pos" else "neg"`)
	ctx := context.Background()
	args := map[string]any{"x": 42.0}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := reg.Execute(ctx, "bench::cond", args)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRegistry_Execute_Inline_Arithmetic(b *testing.B) {
	reg := newTestRegistry()
	_ = reg.Register("bench::inline_arith", `fn inline_arith(x: float, y: float) -> float => x + y * 2`)
	ctx := context.Background()
	args := map[string]any{"x": 42.0, "y": 10.0}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := reg.Execute(ctx, "bench::inline_arith", args)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRegistry_Execute_BuiltinCall(b *testing.B) {
	reg := newTestRegistry()
	_ = reg.Register("bench::builtin", `fn builtin(s: string) -> string => upper(trim(s))`)
	ctx := context.Background()
	args := map[string]any{"s": "  Hello World  "}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := reg.Execute(ctx, "bench::builtin", args)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRegistry_Execute_Pipe(b *testing.B) {
	reg := newTestRegistry()
	_ = reg.Register("bench::pipe", `fn pipe(s: string) -> string => s | trim | upper`)
	ctx := context.Background()
	args := map[string]any{"s": "  Hello World  "}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := reg.Execute(ctx, "bench::pipe", args)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRegistry_Execute_FieldAccess(b *testing.B) {
	reg := newTestRegistry()
	_ = reg.Register("bench::field", `fn field(obj: object) -> any => obj.a.b.c`)
	ctx := context.Background()
	args := map[string]any{
		"obj": map[string]any{"a": map[string]any{"b": map[string]any{"c": int64(42)}}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := reg.Execute(ctx, "bench::field", args)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRegistry_Execute_NullCoalesce(b *testing.B) {
	reg := newTestRegistry()
	_ = reg.Register("bench::coalesce", `fn coalesce(x: any, y: any) -> any => x ?? y`)
	ctx := context.Background()
	args := map[string]any{"x": nil, "y": int64(42)}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := reg.Execute(ctx, "bench::coalesce", args)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRegistry_Validate(b *testing.B) {
	reg := newTestRegistry()
	src := `fn analyze(data: any[], threshold: float) -> object:
  let total = sum(data)
  let avg_val = avg(data)
  if len(data) > 0 then {total: total, avg: avg_val} else {total: 0, avg: 0}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := reg.Validate(src)
		if !result.Valid {
			b.Fatal("validation failed")
		}
	}
}

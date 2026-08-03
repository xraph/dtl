package benchmarks

import (
	"context"
	"testing"

	"github.com/xraph/dtl/registry"
)

// These break DTL's own numbers down so the cross-engine table can be read
// correctly. BenchmarkCompile builds a fresh isolated environment per
// iteration for every engine (cel.NewEnv, lua.NewState, registry.New), which
// is the fair like-for-like shape — but it hides how that cost splits for DTL.

const detailSrc = `fn f(a: float, b: float) -> float => a + b * 2 - 1`

// BenchmarkDTLRegistryNew isolates registry construction, which registers the
// whole standard library into a fresh builtin map.
func BenchmarkDTLRegistryNew(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = registry.New(registry.Config{})
	}
}

// BenchmarkDTLRegisterWarm isolates parse + compile + cache on a registry that
// already exists — what a long-lived host actually pays per new function.
func BenchmarkDTLRegisterWarm(b *testing.B) {
	reg := registry.New(registry.Config{})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := reg.Register("bench::warm", detailSrc); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDTLExecuteNoop measures the floor of Registry.Execute: a function
// whose body is a single parameter reference. Whatever this costs is paid by
// every DTL call regardless of the expression's complexity.
func BenchmarkDTLExecuteNoop(b *testing.B) {
	reg := registry.New(registry.Config{})
	if err := reg.Register("bench::noop", `fn f(a: float) -> float => a`); err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	args := map[string]any{"a": 1.0}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := reg.Execute(ctx, "bench::noop", args); err != nil {
			b.Fatal(err)
		}
	}
}

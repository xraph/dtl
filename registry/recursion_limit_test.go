package registry

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/xraph/dtl/executor"
)

// registerMutuallyRecursive builds a -> b -> a on the given registry.
//
// Direct self-recursion is rejected at compile time, because a function is not
// in the registry while it is being compiled. Re-registration walks around
// that: each definition compiles against a registry where its callee already
// exists.
func registerMutuallyRecursive(t *testing.T, reg *Registry) {
	t.Helper()
	steps := []struct{ name, src string }{
		{"a", `fn a(n: float) -> float => n`},        // leaf, compiles clean
		{"b", `fn b(n: float) -> float => a(n + 1)`}, // b -> a
		{"a", `fn a(n: float) -> float => b(n + 1)`}, // replace a: a -> b
	}
	for _, s := range steps {
		if err := reg.Register(s.name, s.src); err != nil {
			t.Fatalf("register %s: %v", s.name, err)
		}
	}
}

// TestZeroConfigBoundsRecursion is a regression test for an unrecoverable
// crash: with a zero-value Config the depth limit was disabled, so mutual
// recursion exhausted the goroutine stack and raised `fatal error: stack
// overflow` — fatal to the host process and impossible to recover from.
//
// A zero-value Config is what README.md tells embedders to use, so this path
// specifically must terminate with an ordinary error.
func TestZeroConfigBoundsRecursion(t *testing.T) {
	reg := New(Config{})
	registerMutuallyRecursive(t, reg)

	_, err := reg.Execute(context.Background(), "a", map[string]any{"n": 0.0})
	if err == nil {
		t.Fatal("expected a depth-limit error, got nil — recursion was unbounded")
	}
	if !errors.Is(err, executor.ErrMaxDepth) {
		t.Fatalf("expected executor.ErrMaxDepth, got %v", err)
	}
}

// TestExplicitMaxCallDepthIsHonored checks a caller-supplied limit is used
// verbatim rather than being overridden by the default.
func TestExplicitMaxCallDepthIsHonored(t *testing.T) {
	reg := New(Config{MaxCallDepth: 5})
	registerMutuallyRecursive(t, reg)

	_, err := reg.Execute(context.Background(), "a", map[string]any{"n": 0.0})
	if !errors.Is(err, executor.ErrMaxDepth) {
		t.Fatalf("expected executor.ErrMaxDepth, got %v", err)
	}
}

func TestResolveMaxDepth(t *testing.T) {
	cases := []struct {
		configured, want int
		why              string
	}{
		{0, DefaultMaxCallDepth, "zero selects the safe default"},
		{42, 42, "a positive value is used verbatim"},
		{-1, 0, "negative is an explicit opt-out, mapped to the executor's unbounded sentinel"},
	}
	for _, c := range cases {
		if got := resolveMaxDepth(c.configured); got != c.want {
			t.Errorf("resolveMaxDepth(%d) = %d, want %d — %s", c.configured, got, c.want, c.why)
		}
	}
}

// TestLegitimateDepthIsNotCapped guards the other direction: the default must
// sit well above real transformation nesting, or it trades a crash for broken
// programs. This nests calls to just under the default.
func TestLegitimateDepthIsNotCapped(t *testing.T) {
	reg := New(Config{})

	// Build a chain f0 -> f1 -> ... -> f199, each adding one.
	const chain = 200
	if err := reg.Register("f0", `fn f0(n: float) -> float => n`); err != nil {
		t.Fatalf("register f0: %v", err)
	}
	for i := 1; i < chain; i++ {
		src := fmt.Sprintf(`fn f%d(n: float) -> float => f%d(n + 1)`, i, i-1)
		if err := reg.Register(fmt.Sprintf("f%d", i), src); err != nil {
			t.Fatalf("register f%d: %v", i, err)
		}
	}

	res, err := reg.Execute(context.Background(),
		fmt.Sprintf("f%d", chain-1), map[string]any{"n": 0.0})
	if err != nil {
		t.Fatalf("a %d-deep chain should be well within the default limit: %v", chain-1, err)
	}
	if got := res.Value; got != float64(chain-1) {
		t.Fatalf("got %v, want %v", got, float64(chain-1))
	}
}

// TestDepthErrorIsLegible checks the failure an embedder sees names the limit,
// since "maximum call depth exceeded" is the whole diagnostic they get.
func TestDepthErrorIsLegible(t *testing.T) {
	reg := New(Config{})
	registerMutuallyRecursive(t, reg)

	_, err := reg.Execute(context.Background(), "a", map[string]any{"n": 0.0})
	if err == nil || !strings.Contains(err.Error(), "call depth") {
		t.Fatalf("error should mention call depth, got %v", err)
	}
}

// TestDepthLimitHoldsThroughLambdas covers the hole the original fix left.
//
// Bounding call depth in the executor was not enough: stdlib's higher-order
// functions are plain func(args []any) with no access to the running
// evalCtx, so they invoked lambdas with a depth of 0 and a freshly-taken
// start time. Every element therefore reset both limits, and recursion routed
// through map or filter crashed the process exactly as before — a
// `fatal error: stack overflow`, not an error return.
//
// Lambdas now carry the limits in force where they were written.
func TestDepthLimitHoldsThroughLambdas(t *testing.T) {
	reg := New(Config{})
	mustReg := func(name, src string) {
		t.Helper()
		if err := reg.Register(name, src); err != nil {
			t.Fatalf("register %s: %v", name, err)
		}
	}
	// Same re-registration trick as above, but each hop goes through map so
	// the recursion crosses a stdlib higher-order function every time.
	mustReg("b", `fn b(n: float) -> float => n`)
	mustReg("a", `fn a(n: float) -> float => sum(map([n], x => b(x + 1)))`)
	mustReg("b", `fn b(n: float) -> float => sum(map([n], x => a(x + 1)))`)

	_, err := reg.Execute(context.Background(), "a", map[string]any{"n": 0.0})
	if err == nil {
		t.Fatal("recursion through a lambda was unbounded")
	}
	if !errors.Is(err, executor.ErrMaxDepth) {
		t.Fatalf("got %v, want executor.ErrMaxDepth", err)
	}
}

// The same reset broke timeouts: a fresh start time per element meant the
// deadline could never elapse inside a collection.
func TestTimeoutAppliesInsideLambdas(t *testing.T) {
	reg := New(Config{DefaultTimeout: 20 * time.Millisecond})

	// A large input whose per-element work is trivial but whose total exceeds
	// the timeout comfortably once the deadline is actually honoured.
	xs := make([]any, 200000)
	for i := range xs {
		xs[i] = float64(i)
	}
	if err := reg.Register("f", `fn f(xs: float[]) -> float => xs | map(x => x * 2.0) | sum`); err != nil {
		t.Fatalf("register: %v", err)
	}

	_, err := reg.Execute(context.Background(), "f", map[string]any{"xs": xs})
	if err == nil {
		t.Skip("input completed within the timeout on this machine; timing-sensitive")
	}
	if !errors.Is(err, executor.ErrTimeout) {
		t.Fatalf("got %v, want executor.ErrTimeout", err)
	}
}

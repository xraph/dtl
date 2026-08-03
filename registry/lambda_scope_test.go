package registry

import (
	"context"
	"testing"
	"time"

	"github.com/xraph/dtl/executor"
)

// A lambda's argument scope is now reused across invocations, since filter,
// map and reduce call the same closure once per element. The failure that
// buys is subtle and data-dependent: anything that outlives one iteration
// while still pointing at that scope would observe the *last* iteration's
// values instead of its own. These tests are built to catch exactly that —
// each asserts per-element results that would collapse to a single repeated
// value if reuse leaked.

func exec(t *testing.T, src string, args map[string]any) any {
	t.Helper()
	reg := New(Config{})
	if err := reg.Register("f", src); err != nil {
		t.Fatalf("register: %v", err)
	}
	res, err := reg.Execute(context.Background(), "f", args)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	return res.Value
}

// The baseline: each element must see its own binding.
func TestLambdaSeesItsOwnArgument(t *testing.T) {
	got := exec(t, `fn f(xs: float[]) -> float[] => map(xs, x => x * 2)`,
		map[string]any{"xs": []any{1.0, 2.0, 3.0}})

	arr, ok := got.([]any)
	if !ok || len(arr) != 3 {
		t.Fatalf("got %#v, want a 3-element array", got)
	}
	for i, want := range []float64{2, 4, 6} {
		if arr[i] != want {
			t.Errorf("element %d = %v, want %v — scope reuse leaked between iterations", i, arr[i], want)
		}
	}
}

// The case reuse could actually break: an inner lambda created inside the
// outer lambda's body captures the outer scope. If that scope is the one
// being rebound each iteration, every inner closure would see the final x.
func TestNestedLambdaCapturesItsOwnIteration(t *testing.T) {
	got := exec(t, `fn f(xs: float[]) -> any => map(xs, x => map([10.0], y => x + y))`,
		map[string]any{"xs": []any{1.0, 2.0, 3.0}})

	outer, ok := got.([]any)
	if !ok || len(outer) != 3 {
		t.Fatalf("got %#v, want a 3-element array", got)
	}
	for i, want := range []float64{11, 12, 13} {
		inner, ok := outer[i].([]any)
		if !ok || len(inner) != 1 {
			t.Fatalf("element %d = %#v, want a 1-element array", i, outer[i])
		}
		if inner[0] != want {
			t.Errorf("element %d = %v, want %v — the inner closure saw another iteration's x",
				i, inner[0], want)
		}
	}
}

// Two levels of nesting, so the snapshot has to copy a chain rather than a
// single scope.
func TestDoublyNestedLambdaCaptures(t *testing.T) {
	got := exec(t,
		`fn f(xs: float[]) -> any => map(xs, x => map([10.0], y => map([100.0], z => x + y + z)))`,
		map[string]any{"xs": []any{1.0, 2.0}})

	outer, ok := got.([]any)
	if !ok || len(outer) != 2 {
		t.Fatalf("got %#v, want a 2-element array", got)
	}
	for i, want := range []float64{111, 112} {
		mid := outer[i].([]any)
		inner := mid[0].([]any)
		if inner[0] != want {
			t.Errorf("element %d = %v, want %v — a captured scope was rebound", i, inner[0], want)
		}
	}
}

// A lambda over a nested collection reaches the same closure while an outer
// call is still using its scope. The reentrancy guard has to notice.
func TestLambdaOverNestedCollections(t *testing.T) {
	got := exec(t,
		`fn f(rows: any[]) -> any => map(rows, r => sum(map(r, v => v * 2)))`,
		map[string]any{"rows": []any{
			[]any{1.0, 2.0},
			[]any{10.0, 20.0},
		}})

	arr, ok := got.([]any)
	if !ok || len(arr) != 2 {
		t.Fatalf("got %#v, want a 2-element array", got)
	}
	for i, want := range []float64{6, 60} {
		if arr[i] != want {
			t.Errorf("row %d = %v, want %v", i, arr[i], want)
		}
	}
}

// filter and reduce drive the same reuse path as map.
func TestFilterAndReduceOverReusedScope(t *testing.T) {
	got := exec(t, `fn f(xs: float[]) -> float => xs | filter(x => x > 2.0) | sum`,
		map[string]any{"xs": []any{1.0, 2.0, 3.0, 4.0}})
	if got != 7.0 {
		t.Errorf("filter+sum = %v, want 7", got)
	}

	got = exec(t, `fn f(xs: float[]) -> float => reduce(xs, 0, (acc, x) => acc + x)`,
		map[string]any{"xs": []any{1.0, 2.0, 3.0, 4.0}})
	if got != 10.0 {
		t.Errorf("reduce = %v, want 10", got)
	}
}

// A closure that outlives the collection call must keep its own bindings.
func TestClosureOutlivingTheLoopKeepsItsBindings(t *testing.T) {
	// group_by builds keys via a lambda; the result must key each element by
	// its own value rather than by whatever the last iteration left behind.
	got := exec(t, `fn f(xs: float[]) -> any => group_by(xs, x => x)`,
		map[string]any{"xs": []any{1.0, 2.0, 2.0}})

	groups, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("got %#v, want an object", got)
	}
	if len(groups) != 2 {
		t.Errorf("got %d groups (%v), want 2 — every element grouped under one key", len(groups), groups)
	}
}

// A lambda binding more names than a scope holds inline must still be reset
// correctly between iterations, including its spill storage.
func TestReusedScopeResetsSpillStorage(t *testing.T) {
	got := exec(t,
		`fn f(xs: float[]) -> float[] => map(xs, x => sum([x, x, x]))`,
		map[string]any{"xs": []any{1.0, 2.0, 3.0}})

	arr := got.([]any)
	for i, want := range []float64{3, 6, 9} {
		if arr[i] != want {
			t.Errorf("element %d = %v, want %v", i, arr[i], want)
		}
	}
}

// TestClosuresEscapingToHostKeepTheirBindings is the test that actually
// constrains scope reuse.
//
// The nested-lambda tests above pass with or without the snapshot in
// evalLambda, because an inner closure created and consumed inside one
// iteration never outlives the scope it captured. The corruption only becomes
// observable when a closure escapes the loop entirely — which DTL can express,
// since a lambda may return a lambda, and the host can invoke the result via
// executor.CallLambda.
//
// With the snapshot removed, every closure below returns 103: they all end up
// sharing the final iteration's scope. Verified by disabling it.
func TestClosuresEscapingToHostKeepTheirBindings(t *testing.T) {
	reg := New(Config{})
	if err := reg.Register("f", `fn f(xs: float[]) -> any => map(xs, x => (y => x + y))`); err != nil {
		t.Fatalf("register: %v", err)
	}

	res, err := reg.Execute(context.Background(), "f", map[string]any{"xs": []any{1.0, 2.0, 3.0}})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	closures, ok := res.Value.([]any)
	if !ok || len(closures) != 3 {
		t.Fatalf("got %#v, want three closures", res.Value)
	}

	for i, want := range []float64{101, 102, 103} {
		got, err := executor.CallLambda(context.Background(), closures[i], []any{100.0}, time.Now(), 0)
		if err != nil {
			t.Fatalf("calling closure %d: %v", i, err)
		}
		if got != want {
			t.Errorf("closure %d returned %v, want %v — it captured another iteration's scope", i, got, want)
		}
	}
}

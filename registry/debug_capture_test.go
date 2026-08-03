package registry

import (
	"context"
	"testing"
)

// The debug sink moved off the context and onto the execution itself, so that
// a run which never calls debug() allocates nothing for it. The risk in that
// change is silent loss: output would simply stop appearing, with every test
// still green. These pin down each path the sink has to reach.

func TestDebugCapturedFromTopLevel(t *testing.T) {
	reg := New(Config{})
	if err := reg.Register("f", `fn f(x: float) -> float => DEBUG("x is", x)`); err != nil {
		t.Fatalf("register: %v", err)
	}

	res, err := reg.Execute(context.Background(), "f", map[string]any{"x": 7.0})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(res.Logs) != 1 {
		t.Fatalf("want 1 log entry, got %d (%#v)", len(res.Logs), res.Logs)
	}
	if res.Logs[0].Label != "x is" {
		t.Errorf("label = %q, want %q", res.Logs[0].Label, "x is")
	}
	// debug returns its last argument so it stays chainable.
	if res.Value != 7.0 {
		t.Errorf("value = %v, want 7", res.Value)
	}
}

// A callee's output belongs to the same run, so the sink must cross the
// user-function boundary — where a fresh evalCtx and scope are built.
func TestDebugCapturedFromNestedFunction(t *testing.T) {
	reg := New(Config{})
	if err := reg.Register("inner", `fn inner(x: float) -> float => DEBUG("inner", x)`); err != nil {
		t.Fatalf("register inner: %v", err)
	}
	if err := reg.Register("outer", `fn outer(x: float) -> float => inner(x) + 1`); err != nil {
		t.Fatalf("register outer: %v", err)
	}

	res, err := reg.Execute(context.Background(), "outer", map[string]any{"x": 2.0})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(res.Logs) != 1 {
		t.Fatalf("want 1 log entry from the callee, got %d (%#v)", len(res.Logs), res.Logs)
	}
	if res.Logs[0].Label != "inner" {
		t.Errorf("label = %q, want %q", res.Logs[0].Label, "inner")
	}
}

// Lambdas capture the sink at closure-creation time, because the public
// CallLambda entry point stdlib uses carries only a context.
func TestDebugCapturedFromLambda(t *testing.T) {
	reg := New(Config{})
	src := `fn f(xs: float[]) -> float[] => map(xs, x => DEBUG("elem", x))`
	if err := reg.Register("f", src); err != nil {
		t.Fatalf("register: %v", err)
	}

	res, err := reg.Execute(context.Background(), "f",
		map[string]any{"xs": []any{1.0, 2.0, 3.0}})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(res.Logs) != 3 {
		t.Fatalf("want 3 log entries, one per element, got %d (%#v)", len(res.Logs), res.Logs)
	}
}

// A run that never emits must leave Logs nil — that is the allocation the
// change was made to avoid, so assert it rather than merely assuming it.
func TestNoDebugLeavesLogsNil(t *testing.T) {
	reg := New(Config{})
	if err := reg.Register("f", `fn f(x: float) -> float => x * 2`); err != nil {
		t.Fatalf("register: %v", err)
	}

	res, err := reg.Execute(context.Background(), "f", map[string]any{"x": 4.0})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.Logs != nil {
		t.Errorf("Logs = %#v, want nil for a run that emitted nothing", res.Logs)
	}
}

// Output emitted before a failure is still worth returning, since it is often
// the only evidence of how execution reached the error.
func TestDebugRetainedOnError(t *testing.T) {
	reg := New(Config{})
	src := `fn f(x: float) -> float:
    let seen = DEBUG("before", x)
    raise "boom"`
	if err := reg.Register("f", src); err != nil {
		t.Fatalf("register: %v", err)
	}

	res, err := reg.Execute(context.Background(), "f", map[string]any{"x": 1.0})
	if err == nil {
		t.Fatal("expected an error from raise")
	}
	if res == nil || len(res.Logs) != 1 {
		t.Fatalf("want the pre-failure entry preserved, got %#v", res)
	}
}

// ExecuteInline shares the executor path and must capture identically.
func TestDebugCapturedFromInline(t *testing.T) {
	reg := New(Config{})
	res, err := reg.ExecuteInline(context.Background(),
		`fn f(x: float) -> float => DEBUG("inline", x)`,
		map[string]any{"x": 5.0})
	if err != nil {
		t.Fatalf("execute inline: %v", err)
	}
	if len(res.Logs) != 1 {
		t.Fatalf("want 1 log entry, got %d (%#v)", len(res.Logs), res.Logs)
	}
}

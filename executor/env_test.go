package executor

import (
	"context"
	"fmt"
	"testing"
)

// env moved from a map to inline storage with a linear scan, so the behaviour
// a map gave for free now has to be implemented: overwrite-on-rebind, spilling
// past the inline capacity, and shadowing along the scope chain.

func TestEnvSetAndGet(t *testing.T) {
	e := newEnv(nil)
	e.set("a", 1)
	e.set("b", "two")

	if v, ok := e.get("a"); !ok || v != 1 {
		t.Errorf("get(a) = %v, %v; want 1, true", v, ok)
	}
	if v, ok := e.get("b"); !ok || v != "two" {
		t.Errorf("get(b) = %v, %v; want two, true", v, ok)
	}
	if _, ok := e.get("missing"); ok {
		t.Error("get(missing) reported found")
	}
}

// Rebinding must overwrite in place. Appending instead would leave the old
// binding earlier in the scan order, so the stale value would win.
func TestEnvRebindOverwrites(t *testing.T) {
	e := newEnv(nil)
	e.set("x", 1)
	e.set("x", 2)

	if v, _ := e.get("x"); v != 2 {
		t.Errorf("get(x) = %v, want 2 — rebind did not overwrite", v)
	}
}

// Past envInline, bindings spill to the slices. Both halves must stay
// readable and writable, and rebinding must work on either side of the split.
func TestEnvSpillsPastInline(t *testing.T) {
	e := newEnv(nil)
	const n = envInline * 3
	for i := 0; i < n; i++ {
		e.set(fmt.Sprintf("v%d", i), i)
	}

	for i := 0; i < n; i++ {
		name := fmt.Sprintf("v%d", i)
		if v, ok := e.get(name); !ok || v != i {
			t.Errorf("get(%s) = %v, %v; want %d, true", name, v, ok, i)
		}
	}

	// Rebind one inline and one spilled binding.
	e.set("v0", "inline-updated")
	e.set(fmt.Sprintf("v%d", n-1), "spilled-updated")
	if v, _ := e.get("v0"); v != "inline-updated" {
		t.Errorf("inline rebind failed: %v", v)
	}
	if v, _ := e.get(fmt.Sprintf("v%d", n-1)); v != "spilled-updated" {
		t.Errorf("spilled rebind failed: %v", v)
	}
}

func TestEnvParentLookup(t *testing.T) {
	parent := newEnv(nil)
	parent.set("outer", "p")
	child := newEnv(parent)
	child.set("inner", "c")

	if v, ok := child.get("outer"); !ok || v != "p" {
		t.Errorf("child.get(outer) = %v, %v; want p, true", v, ok)
	}
	if _, ok := parent.get("inner"); ok {
		t.Error("parent saw a child binding")
	}
}

// A binding in an inner scope hides the outer one without destroying it.
func TestEnvChildShadowsParent(t *testing.T) {
	parent := newEnv(nil)
	parent.set("x", "outer")
	child := newEnv(parent)
	child.set("x", "inner")

	if v, _ := child.get("x"); v != "inner" {
		t.Errorf("child.get(x) = %v, want inner", v)
	}
	if v, _ := parent.get("x"); v != "outer" {
		t.Errorf("parent.get(x) = %v, want outer — child mutated the parent", v)
	}
}

// Shadowing must still work when the outer binding lives in spill storage,
// since the two halves are scanned separately.
func TestEnvShadowsSpilledParentBinding(t *testing.T) {
	parent := newEnv(nil)
	for i := 0; i < envInline+2; i++ {
		parent.set(fmt.Sprintf("v%d", i), "outer")
	}
	spilled := fmt.Sprintf("v%d", envInline+1)

	child := newEnv(parent)
	child.set(spilled, "inner")

	if v, _ := child.get(spilled); v != "inner" {
		t.Errorf("child.get(%s) = %v, want inner", spilled, v)
	}
	if v, _ := parent.get(spilled); v != "outer" {
		t.Errorf("parent.get(%s) = %v, want outer", spilled, v)
	}
}

func TestEnvNilValueIsStillABinding(t *testing.T) {
	e := newEnv(nil)
	e.set("x", nil)
	if v, ok := e.get("x"); !ok || v != nil {
		t.Errorf("get(x) = %v, %v; want nil, true — a nil binding must still exist", v, ok)
	}
}

// NewDebugContext predates the sink and is retained for callers driving the
// executor directly. It still has to collect output.
func TestLegacyDebugContextStillCollects(t *testing.T) {
	ex := New(nil, nil, ExecConfig{})
	ctx, buf := NewDebugContext(context.Background())

	ec := &evalCtx{ctx: ctx, env: newEnv(nil)}
	if _, err := ex.callFunction(ec, "DEBUG", []any{"label", 1}); err != nil {
		t.Fatalf("callFunction: %v", err)
	}

	if len(*buf) != 1 {
		t.Fatalf("want 1 entry in the context buffer, got %d", len(*buf))
	}
	if (*buf)[0].Label != "label" {
		t.Errorf("label = %q, want %q", (*buf)[0].Label, "label")
	}
}

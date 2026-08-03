package registry

import (
	"context"
	"sync"
	"testing"

	"github.com/xraph/dtl/executor"
	"github.com/xraph/dtl/stdlib"
)

// The standard library is now one table shared by every registry in the
// process, which is what makes New cheap. The hazard that buys is leakage: a
// builtin registered by one host reaching another registry, or worse, being
// written into the shared table itself. For a host that builds a registry per
// tenant, that is a data-isolation bug, not a performance regression — so
// these tests are the ones that matter most about this change.

func constBuiltin(name string, val any) *executor.BuiltinFunc {
	return &executor.BuiltinFunc{
		Name: name, MinArgs: 0, MaxArgs: 0,
		Doc: name + "() -> any -- test builtin",
		Fn:  func([]any) (any, error) { return val, nil },
	}
}

// shadowUpper replaces the stdlib's one-argument upper, so it has to accept
// the same arity the real one does.
func shadowUpper() *executor.BuiltinFunc {
	return &executor.BuiltinFunc{
		Name: "upper", MinArgs: 1, MaxArgs: 1,
		Doc: "upper(s) -> string -- test shadow",
		Fn:  func([]any) (any, error) { return "SHADOWED", nil },
	}
}

func TestRegisterBuiltinDoesNotLeakToOtherRegistry(t *testing.T) {
	a := New(Config{})
	b := New(Config{})

	a.RegisterBuiltin("tenant::secret", constBuiltin("tenant::secret", "a-only"))

	if _, ok := a.ResolveFunction("tenant::secret"); !ok {
		t.Fatal("registry a cannot see its own builtin")
	}
	if _, ok := b.ResolveFunction("tenant::secret"); ok {
		t.Fatal("registry b sees a builtin registered on registry a — tables are shared")
	}

	// And a registry created afterwards must not inherit it either.
	c := New(Config{})
	if _, ok := c.ResolveFunction("tenant::secret"); ok {
		t.Fatal("a registry created later inherited another registry's builtin")
	}
}

// Executing is the path that matters; resolution alone could pass while the
// executor still consulted a shared table.
func TestRegisteredBuiltinIsNotExecutableFromAnotherRegistry(t *testing.T) {
	a := New(Config{})
	b := New(Config{})

	a.RegisterBuiltin("tenant::whoami", constBuiltin("tenant::whoami", "tenant-a"))

	if err := a.Register("fa", `fn fa() -> any => tenant::whoami()`); err != nil {
		t.Fatalf("register on a: %v", err)
	}
	res, err := a.Execute(context.Background(), "fa", nil)
	if err != nil {
		t.Fatalf("execute on a: %v", err)
	}
	if res.Value != "tenant-a" {
		t.Errorf("a returned %v, want tenant-a", res.Value)
	}

	// b must not even compile a reference to it.
	if err := b.Register("fb", `fn fb() -> any => tenant::whoami()`); err == nil {
		t.Fatal("registry b compiled a call to registry a's builtin")
	}
}

// Shadowing a standard-library name must affect only the registry doing it.
// If this wrote through to the shared table it would corrupt every registry
// in the process, including ones already running.
func TestShadowingStdlibIsRegistryLocal(t *testing.T) {
	a := New(Config{})
	b := New(Config{})

	a.RegisterBuiltin("upper", shadowUpper())

	if err := a.Register("f", `fn f() -> any => upper("x")`); err != nil {
		t.Fatalf("register on a: %v", err)
	}
	if err := b.Register("f", `fn f() -> any => upper("x")`); err != nil {
		t.Fatalf("register on b: %v", err)
	}

	resA, err := a.Execute(context.Background(), "f", nil)
	if err != nil {
		t.Fatalf("execute a: %v", err)
	}
	if resA.Value != "SHADOWED" {
		t.Errorf("a: upper = %v, want SHADOWED", resA.Value)
	}

	resB, err := b.Execute(context.Background(), "f", nil)
	if err != nil {
		t.Fatalf("execute b: %v", err)
	}
	if resB.Value != "X" {
		t.Errorf("b: upper = %v, want X — the shared stdlib was mutated", resB.Value)
	}
}

// The shared table itself must come out of all of this unchanged.
func TestSharedTableIsNotMutated(t *testing.T) {
	before := len(stdlib.Shared())
	original := stdlib.Shared()["upper"]

	r := New(Config{})
	r.RegisterBuiltin("upper", shadowUpper())
	r.RegisterBuiltin("brand::new", constBuiltin("brand::new", 1))

	if got := len(stdlib.Shared()); got != before {
		t.Errorf("shared table grew from %d to %d entries", before, got)
	}
	if stdlib.Shared()["upper"] != original {
		t.Error("shared table's upper was replaced by a registry-local shadow")
	}
	if _, leaked := stdlib.Shared()["brand::new"]; leaked {
		t.Error("a registry-local builtin was written into the shared table")
	}
}

// GetBuiltins hands out a table callers may treat as theirs, so it must be a
// copy — otherwise a caller writing to it would corrupt every registry.
func TestGetBuiltinsReturnsAnIsolatedCopy(t *testing.T) {
	r := New(Config{})
	sharedLen := len(stdlib.Shared())

	got := r.GetBuiltins()
	got["injected"] = constBuiltin("injected", 1)

	if _, leaked := stdlib.Shared()["injected"]; leaked {
		t.Fatal("writing to GetBuiltins' result reached the shared table")
	}
	if len(stdlib.Shared()) != sharedLen {
		t.Fatal("shared table changed size after a write to GetBuiltins' result")
	}
	if _, ok := r.ResolveFunction("injected"); ok {
		t.Error("writing to GetBuiltins' result changed what the registry resolves")
	}
}

func TestGetBuiltinsIncludesOverrides(t *testing.T) {
	r := New(Config{})
	r.RegisterBuiltin("host::thing", constBuiltin("host::thing", 1))

	all := r.GetBuiltins()
	if _, ok := all["host::thing"]; !ok {
		t.Error("GetBuiltins omitted a host-registered builtin")
	}
	if _, ok := all["upper"]; !ok {
		t.Error("GetBuiltins omitted the standard library")
	}
}

func TestListFunctionNamesHasNoDuplicateWhenShadowing(t *testing.T) {
	r := New(Config{})
	r.RegisterBuiltin("upper", shadowUpper())

	seen := map[string]int{}
	for _, n := range r.ListFunctionNames() {
		seen[n]++
	}
	if seen["upper"] != 1 {
		t.Errorf("upper listed %d times, want 1 — a shadowed name was counted twice", seen["upper"])
	}
}

// Shared is built under a sync.Once; concurrent first use must not race or
// return a partially built table.
func TestSharedIsSafeForConcurrentFirstUse(t *testing.T) {
	var wg sync.WaitGroup
	lens := make([]int, 16)
	for i := range lens {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			lens[i] = len(New(Config{}).GetBuiltins())
		}(i)
	}
	wg.Wait()

	for i, n := range lens {
		if n != lens[0] {
			t.Fatalf("goroutine %d saw %d builtins, goroutine 0 saw %d", i, n, lens[0])
		}
	}
}

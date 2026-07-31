package registry

import (
	"context"
	"testing"
	"time"

	"github.com/xraph/dtl/stdlib"
)

func newTestRegistry() *Registry {
	return New(Config{
		DefaultTimeout: 5 * time.Second,
		MaxCallDepth:   100,
	})
}

func TestNew_LoadsStdlib(t *testing.T) {
	r := newTestRegistry()
	// Should have stdlib builtins loaded
	builtins := r.GetBuiltins()
	if len(builtins) == 0 {
		t.Fatal("expected stdlib builtins to be loaded")
	}
	// Spot-check a few
	for _, name := range []string{"len", "sqrt", "upper", "now"} {
		if _, ok := builtins[name]; !ok {
			t.Errorf("expected builtin %q to be loaded", name)
		}
	}
}

func TestRegister_CachesCompiledFn(t *testing.T) {
	r := newTestRegistry()
	src := "fn add(a: int, b: int) -> int => a + b"
	err := r.Register("shared::add", src)
	if err != nil {
		t.Fatalf("register error: %v", err)
	}

	cf, ok := r.GetCompiled(context.Background(), "shared::add")
	if !ok {
		t.Fatal("expected compiled function to be cached")
	}
	if cf.Name != "shared::add" {
		t.Errorf("name: got %q, want %q", cf.Name, "shared::add")
	}
	if cf.Source != src {
		t.Error("source mismatch")
	}
}

func TestRegister_ParseError(t *testing.T) {
	r := newTestRegistry()
	err := r.Register("shared::bad", "invalid syntax here")
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestExecute_HappyPath(t *testing.T) {
	r := newTestRegistry()
	src := "fn double(x: int) -> int => x * 2"
	if err := r.Register("shared::double", src); err != nil {
		t.Fatalf("register error: %v", err)
	}

	result, err := r.Execute(context.Background(), "shared::double", map[string]any{"x": int64(5)})
	if err != nil {
		t.Fatalf("execute error: %v", err)
	}
	if result.Value != int64(10) {
		t.Errorf("got %v, want 10", result.Value)
	}
}

func TestExecute_UnknownFunction(t *testing.T) {
	r := newTestRegistry()
	_, err := r.Execute(context.Background(), "shared::nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for unknown function")
	}
}

func TestExecuteInline(t *testing.T) {
	r := newTestRegistry()
	src := "fn temp(x: float) -> float => x * 1.8 + 32"
	result, err := r.ExecuteInline(context.Background(), src, map[string]any{"x": 100.0})
	if err != nil {
		t.Fatalf("execute inline error: %v", err)
	}
	f, ok := result.Value.(float64)
	if !ok {
		t.Fatalf("expected float64, got %T", result.Value)
	}
	if f != 212.0 {
		t.Errorf("got %v, want 212.0", f)
	}
}

func TestExecuteInline_ParseError(t *testing.T) {
	r := newTestRegistry()
	_, err := r.ExecuteInline(context.Background(), "invalid", nil)
	if err == nil {
		t.Fatal("expected error for invalid source")
	}
}

func TestValidate_Valid(t *testing.T) {
	r := newTestRegistry()
	result := r.Validate("fn add(a: int, b: int) -> int => a + b")
	if !result.Valid {
		t.Errorf("expected valid, got errors: %v", result.Errors)
	}
	if result.AST == nil {
		t.Error("expected non-nil AST")
	}
}

func TestValidate_Invalid(t *testing.T) {
	r := newTestRegistry()
	result := r.Validate("")
	if result.Valid {
		t.Error("expected invalid for empty source")
	}
}

func TestInvalidate_RemovesFromCache(t *testing.T) {
	r := newTestRegistry()
	src := "fn f(x: int) -> int => x"
	if err := r.Register("shared::f", src); err != nil {
		t.Fatalf("register error: %v", err)
	}

	_, ok := r.GetCompiled(context.Background(), "shared::f")
	if !ok {
		t.Fatal("expected function to be cached before invalidate")
	}

	r.Invalidate("shared::f")

	_, ok = r.GetCompiled(context.Background(), "shared::f")
	if ok {
		t.Error("expected function to be removed after invalidate")
	}
}

func TestResolveFunction_FindsBuiltins(t *testing.T) {
	r := newTestRegistry()
	_, ok := r.ResolveFunction("len")
	if !ok {
		t.Error("expected to resolve builtin 'len'")
	}
}

func TestResolveFunction_FindsCompiled(t *testing.T) {
	r := newTestRegistry()
	if err := r.Register("shared::myFn", "fn myFn(a: int) -> int => a"); err != nil {
		t.Fatalf("register error: %v", err)
	}

	count, ok := r.ResolveFunction("shared::myFn")
	if !ok {
		t.Error("expected to resolve compiled function")
	}
	if count != 1 {
		t.Errorf("param count: got %d, want 1", count)
	}
}

func TestResolveFunction_NotFound(t *testing.T) {
	r := newTestRegistry()
	_, ok := r.ResolveFunction("nonexistent")
	if ok {
		t.Error("expected ResolveFunction to return false for unknown")
	}
}

func TestListCompiled(t *testing.T) {
	r := newTestRegistry()
	if err := r.Register("shared::a", "fn a() -> int => 1"); err != nil {
		t.Fatalf("register a error: %v", err)
	}
	if err := r.Register("shared::b", "fn b() -> int => 2"); err != nil {
		t.Fatalf("register b error: %v", err)
	}

	names := r.ListCompiled()
	if len(names) != 2 {
		t.Errorf("expected 2 compiled functions, got %d", len(names))
	}
}

func TestGetCompiled_NotFound(t *testing.T) {
	r := newTestRegistry()
	_, ok := r.GetCompiled(context.Background(), "nonexistent")
	if ok {
		t.Error("expected false for non-existent function")
	}
}

func TestExecute_GlobalVarsInjection(t *testing.T) {
	r := newTestRegistry()
	err := r.Register("test::global_access", `fn global_access(global: object) -> any => global.env.MY_VAR`)
	if err != nil {
		t.Fatalf("register error: %v", err)
	}

	// Inject global vars via context
	globalVars := map[string]any{
		"env": map[string]any{
			"MY_VAR": "hello_from_env",
		},
		"secrets": map[string]any{},
	}
	ctx := stdlib.WithGlobalVars(context.Background(), globalVars)

	result, execErr := r.Execute(ctx, "test::global_access", nil)
	if execErr != nil {
		t.Fatalf("execute error: %v", execErr)
	}
	if result.Value != "hello_from_env" {
		t.Errorf("got %v, want 'hello_from_env'", result.Value)
	}
}

func TestExecute_GlobalVarsNotOverwritten(t *testing.T) {
	r := newTestRegistry()
	err := r.Register("test::custom_global", `fn custom_global(global: object) -> any => global.custom`)
	if err != nil {
		t.Fatalf("register error: %v", err)
	}

	// Caller provides explicit "global" — should not be overwritten
	args := map[string]any{
		"global": map[string]any{"custom": "explicit_value"},
	}
	ctx := stdlib.WithGlobalVars(context.Background(), map[string]any{
		"env": map[string]any{},
	})

	result, execErr := r.Execute(ctx, "test::custom_global", args)
	if execErr != nil {
		t.Fatalf("execute error: %v", execErr)
	}
	if result.Value != "explicit_value" {
		t.Errorf("got %v, want 'explicit_value'", result.Value)
	}
}

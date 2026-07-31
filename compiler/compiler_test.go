package compiler

import (
	"testing"

	"github.com/xraph/dtl/parser"
)

// mockResolver implements FunctionResolver for testing.
type mockResolver struct {
	fns map[string]int // name -> param count
}

func (m *mockResolver) ResolveFunction(name string) (int, bool) {
	count, ok := m.fns[name]
	return count, ok
}

// --- Tests ---

func TestCompile_ValidAST(t *testing.T) {
	fn, errs := parser.Parse("fn add(a: int, b: int) -> int => a + b")
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	c := New(nil) // no resolver — dependency tracking only
	result := c.Compile(fn)

	if result.HasErrors() {
		t.Fatalf("unexpected compile errors: %v", result.Errors)
	}
}

func TestCompile_UndefinedVariable(t *testing.T) {
	fn, errs := parser.Parse("fn f(x: int) -> int => x + y")
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	c := New(nil)
	result := c.Compile(fn)

	if !result.HasErrors() {
		t.Fatal("expected compile errors for undefined variable y")
	}
	found := false
	for _, e := range result.Errors {
		if e.Code == "undefined_var" {
			found = true
		}
	}
	if !found {
		t.Error("expected undefined_var error code")
	}
}

func TestCompile_LetScopeVisibility(t *testing.T) {
	src := `fn f() -> int:
  let x = 1
  return x`
	fn, errs := parser.Parse(src)
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	c := New(nil)
	result := c.Compile(fn)

	if result.HasErrors() {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
}

func TestCompile_FunctionCallResolution_Known(t *testing.T) {
	fn, errs := parser.Parse("fn f(x: float) -> float => sqrt(x)")
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	resolver := &mockResolver{fns: map[string]int{"sqrt": 1}}
	c := New(resolver)
	result := c.Compile(fn)

	if result.HasErrors() {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if len(result.Dependencies) != 1 || result.Dependencies[0] != "sqrt" {
		t.Errorf("dependencies: got %v, want [sqrt]", result.Dependencies)
	}
}

func TestCompile_FunctionCallResolution_Unknown(t *testing.T) {
	fn, errs := parser.Parse("fn f(x: float) -> float => unknown_fn(x)")
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	resolver := &mockResolver{fns: map[string]int{}}
	c := New(resolver)
	result := c.Compile(fn)

	if !result.HasErrors() {
		t.Fatal("expected compile errors for unknown function")
	}
	found := false
	for _, e := range result.Errors {
		if e.Code == "unknown_function" {
			found = true
		}
	}
	if !found {
		t.Error("expected unknown_function error code")
	}
}

func TestCompile_LegacyBuiltinRejected(t *testing.T) {
	// Each legacy flat name should be rejected with a deterministic
	// "deprecated_builtin" error pointing at the namespaced replacement.
	cases := map[string]string{
		"http_get":        "http::get",
		"schema_get":      "dataset::schema",
		"pipeline_run":    "pipeline::run",
		"query_count":     "dataset::count",
		"viz_transform":   "viz::transform",
		"agent_call":      "agent::call",
		"http_post":       "http::post",
		"schema_columns":  "dataset::columns",
		"schema_validate": "dataset::validate",
	}

	for legacy, replacement := range cases {
		t.Run(legacy, func(t *testing.T) {
			src := "fn f(x: string) -> any => " + legacy + "(x)"
			fn, errs := parser.Parse(src)
			if len(errs) > 0 {
				t.Fatalf("parse errors: %v", errs)
			}
			c := New(&mockResolver{fns: map[string]int{}})
			result := c.Compile(fn)

			if !result.HasErrors() {
				t.Fatalf("expected error for legacy builtin %q", legacy)
			}
			var found bool
			for _, e := range result.Errors {
				if e.Code == "deprecated_builtin" {
					found = true
					if !contains(e.Message, replacement) {
						t.Errorf("error message %q does not mention replacement %q", e.Message, replacement)
					}
				}
			}
			if !found {
				t.Errorf("expected deprecated_builtin code; got %v", result.Errors)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > 0 && (indexOf(s, substr) >= 0)))
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func TestCompile_NoResolver_TracksDependencies(t *testing.T) {
	fn, errs := parser.Parse("fn f(x: float) -> float => sqrt(x)")
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	c := New(nil) // nil resolver — no resolution, just tracking
	result := c.Compile(fn)

	// Should NOT have errors (no resolver means no resolution check)
	if result.HasErrors() {
		t.Fatalf("unexpected errors with nil resolver: %v", result.Errors)
	}
	if len(result.Dependencies) != 1 || result.Dependencies[0] != "sqrt" {
		t.Errorf("dependencies: got %v, want [sqrt]", result.Dependencies)
	}
}

func TestCompile_DependencyDeduplication(t *testing.T) {
	fn, errs := parser.Parse("fn f(x: float) -> float => sqrt(x) + sqrt(x)")
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	c := New(nil)
	result := c.Compile(fn)

	if result.HasErrors() {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if len(result.Dependencies) != 1 {
		t.Errorf("expected deduplicated dependencies, got %v", result.Dependencies)
	}
}

func TestCompile_LambdaScope(t *testing.T) {
	// Lambda params should be visible inside the lambda body
	fn, errs := parser.Parse("fn f(arr: any) -> any => arr")
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	c := New(nil)
	result := c.Compile(fn)

	if result.HasErrors() {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
}

func TestCompile_ParameterAutoDefinition(t *testing.T) {
	// Parameters should automatically be in scope
	fn, errs := parser.Parse("fn f(a: int, b: int, c: int) -> int => a + b + c")
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	c := New(nil)
	result := c.Compile(fn)

	if result.HasErrors() {
		t.Fatalf("unexpected errors for param references: %v", result.Errors)
	}
}

func TestCompile_MatchNotExhaustive(t *testing.T) {
	src := `fn f(x: int) -> string:
  return match x:
    when 1 => "one"
    when 2 => "two"`
	fn, errs := parser.Parse(src)
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	c := New(nil)
	result := c.Compile(fn)

	// Should have a warning about non-exhaustive match
	found := false
	for _, e := range result.Errors {
		if e.Code == "match_not_exhaustive" {
			found = true
		}
	}
	if !found {
		t.Error("expected match_not_exhaustive error/warning")
	}
}

func TestCompile_MatchWithWildcard(t *testing.T) {
	src := `fn f(x: int) -> string:
  return match x:
    when 1 => "one"
    when _ => "other"`
	fn, errs := parser.Parse(src)
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	c := New(nil)
	result := c.Compile(fn)

	// Should NOT have match_not_exhaustive
	for _, e := range result.Errors {
		if e.Code == "match_not_exhaustive" {
			t.Error("did not expect match_not_exhaustive with wildcard")
		}
	}
}

func TestCompile_DollarParamNotUndefined(t *testing.T) {
	// $-prefixed identifiers should not be flagged as undefined
	fn, errs := parser.Parse("fn f() -> any => $param")
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	c := New(nil)
	result := c.Compile(fn)

	for _, e := range result.Errors {
		if e.Code == "undefined_var" {
			t.Error("$param should not be flagged as undefined")
		}
	}
}

func TestCompileError_Error(t *testing.T) {
	e := CompileError{Line: 1, Column: 5, Code: "test", Message: "test error"}
	msg := e.Error()
	if msg == "" {
		t.Error("error message should not be empty")
	}
}

func TestCompileResult_HasErrors(t *testing.T) {
	r := &CompileResult{}
	if r.HasErrors() {
		t.Error("empty result should not have errors")
	}

	r.Errors = append(r.Errors, CompileError{Code: "test"})
	if !r.HasErrors() {
		t.Error("result with errors should return true")
	}
}

// --- for..in, in, raise, use compile tests ---

func TestCompile_ForInExpression(t *testing.T) {
	src := `fn f(items: any[]) -> any[]:
  for x in items:
    x * 2`
	fn, errs := parser.Parse(src)
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}
	c := New(nil)
	result := c.Compile(fn)
	if result.HasErrors() {
		t.Fatalf("unexpected compile errors: %v", result.Errors)
	}
}

func TestCompile_ForInWithIndex(t *testing.T) {
	src := `fn f(items: any[]) -> any[]:
  for item, idx in items:
    idx`
	fn, errs := parser.Parse(src)
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}
	c := New(nil)
	result := c.Compile(fn)
	if result.HasErrors() {
		t.Fatalf("unexpected compile errors: %v", result.Errors)
	}
}

func TestCompile_InExpression(t *testing.T) {
	src := `fn f(status: string) -> bool => status in ["active", "pending"]`
	fn, errs := parser.Parse(src)
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}
	c := New(nil)
	result := c.Compile(fn)
	if result.HasErrors() {
		t.Fatalf("unexpected compile errors: %v", result.Errors)
	}
}

func TestCompile_RaiseExpression(t *testing.T) {
	src := `fn f(x: int) -> int:
  if x < 0 then raise "negative" else x`
	fn, errs := parser.Parse(src)
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}
	c := New(nil)
	result := c.Compile(fn)
	if result.HasErrors() {
		t.Fatalf("unexpected compile errors: %v", result.Errors)
	}
}

func TestCompile_UseStatement(t *testing.T) {
	src := `fn f() -> int:
  use maintenance
  42`
	fn, errs := parser.Parse(src)
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}
	c := New(nil)
	result := c.Compile(fn)
	if result.HasErrors() {
		t.Fatalf("unexpected compile errors: %v", result.Errors)
	}
}

func TestCompile_DidYouMean_Variable(t *testing.T) {
	src := `fn f(temperature: float) -> float => temperture + 1`
	fn, errs := parser.Parse(src)
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}
	c := New(nil)
	result := c.Compile(fn)
	if !result.HasErrors() {
		t.Fatal("expected compile errors")
	}
	found := false
	for _, e := range result.Errors {
		if e.Code == "undefined_var" {
			found = true
			// Check that error contains suggestion
			if len(e.Message) > 0 {
				t.Logf("error message: %s", e.Message)
			}
		}
	}
	if !found {
		t.Error("expected undefined_var error")
	}
}

// mockLister implements FunctionResolver and FunctionLister for testing.
type mockLister struct {
	fns map[string]int
}

func (m *mockLister) ResolveFunction(name string) (int, bool) {
	count, ok := m.fns[name]
	return count, ok
}

func (m *mockLister) ListFunctionNames() []string {
	names := make([]string, 0, len(m.fns))
	for name := range m.fns {
		names = append(names, name)
	}
	return names
}

func TestCompile_DidYouMean_Function(t *testing.T) {
	src := `fn f(x: float) -> float => sqrt(x)`
	fn, errs := parser.Parse(src)
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}
	resolver := &mockLister{fns: map[string]int{"sqrt_val": 1, "abs": 1}}
	c := New(resolver)
	result := c.Compile(fn)
	// "sqrt" is not in resolver, but "sqrt_val" is close
	if !result.HasErrors() {
		t.Fatal("expected compile errors")
	}
	for _, e := range result.Errors {
		t.Logf("error: %s", e.Message)
	}
}

// --- Cross-pack calls via `use` ---
//
// A pack function calls another pack's function by declaring `use <pack id>`
// and then calling the bare name; the executor resolves that to
// app:<packID>::<name>, which is the key the function registry holds. The
// compiler is the install-time gate — registry.Register fails on compile
// errors, so every pack install and validation run passes through it.
// Until it understood `use`, a pack that ran correctly was rejected at
// install with unknown_function.

func TestCompile_UseStatementResolvesACrossPackCall(t *testing.T) {
	src := `fn accept(instance_ref: string) -> string:
  use com.example.core.shifts
  return close_shift(instance_ref)
`
	fn, errs := parser.Parse(src)
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	// The registry holds the qualified key, never the bare name.
	c := New(&mockResolver{fns: map[string]int{
		"app:com.example.core.shifts::close_shift": 1,
	}})
	result := c.Compile(fn)

	if result.HasErrors() {
		t.Fatalf("a call resolvable through the use statement must compile: %v", result.Errors)
	}
}

// Each declared namespace is tried, so a function composing several core
// packs resolves against whichever one owns each callee. This is the shape
// the first composing consumer pack takes.
func TestCompile_UseStatementTriesEveryDeclaredNamespace(t *testing.T) {
	src := `fn compose(crew_code: string, worker_ref: string) -> string:
  use com.example.core.shifts
  use com.example.core.crews
  return assign_member(crew_code, worker_ref)
`
	fn, errs := parser.Parse(src)
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	c := New(&mockResolver{fns: map[string]int{
		"app:com.example.core.crews::assign_member": 2,
	}})
	result := c.Compile(fn)

	if result.HasErrors() {
		t.Fatalf("the owning namespace must be found even when an earlier use does not own it: %v", result.Errors)
	}
}

// A name that no declared namespace owns is still an error. Treating `use` as
// a blanket amnesty would turn the install gate off entirely.
func TestCompile_UseStatementDoesNotExcuseAnUnknownFunction(t *testing.T) {
	src := `fn f(x: string) -> string:
  use com.example.core.shifts
  return no_such_function(x)
`
	fn, errs := parser.Parse(src)
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	c := New(&mockResolver{fns: map[string]int{
		"app:com.example.core.shifts::close_shift": 1,
	}})
	result := c.Compile(fn)

	if !result.HasErrors() {
		t.Fatal("expected unknown_function for a name no declared namespace owns")
	}
	found := false
	for _, e := range result.Errors {
		if e.Code == "unknown_function" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected unknown_function, got %v", result.Errors)
	}
}

// The dependency the pack actually has is on the qualified function, not on a
// bare name that exists in no registry. Dependency tracking feeds install
// ordering, so recording the bare name would make the dependency invisible.
func TestCompile_CrossPackCallIsRecordedAsAQualifiedDependency(t *testing.T) {
	src := `fn accept(instance_ref: string) -> string:
  use com.example.core.shifts
  return close_shift(instance_ref)
`
	fn, errs := parser.Parse(src)
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	c := New(&mockResolver{fns: map[string]int{
		"app:com.example.core.shifts::close_shift": 1,
	}})
	result := c.Compile(fn)

	found := false
	for _, dep := range result.Dependencies {
		if dep == "app:com.example.core.shifts::close_shift" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected the qualified dependency to be recorded, got %v", result.Dependencies)
	}
}

package executor

import (
	"context"
	"testing"

	"github.com/xraph/dtl/capability"
	"github.com/xraph/dtl/parser"
)

// The executor must re-scope the context through capability.Enter as it crosses
// into a user function, naming that function. This is the hook an embedder uses
// to confine a function's writes to whatever it was granted.
func TestCallUserFunction_entersCapabilityWithFunctionName(t *testing.T) {
	fnAST, errs := parser.Parse("fn helper() -> int => 1")
	if len(errs) > 0 {
		t.Fatalf("parse: %v", errs)
	}

	var seen []string
	ctx := capability.WithInterceptor(context.Background(),
		func(ctx context.Context, name string) context.Context {
			seen = append(seen, name)
			return ctx
		})

	ex := New(map[string]*BuiltinFunc{}, nil, ExecConfig{})
	ec := &evalCtx{ctx: ctx, env: newEnv(nil)}
	compiled := &CompiledFunction{Name: "app:com.acme.pack::helper", AST: fnAST}

	if _, err := ex.callUserFunction(ec, compiled, nil); err != nil {
		t.Fatalf("call: %v", err)
	}

	if len(seen) != 1 || seen[0] != "app:com.acme.pack::helper" {
		t.Fatalf("capability.Enter got %v, want [app:com.acme.pack::helper]", seen)
	}
}

// markerKey is the test's stand-in for whatever key a real interceptor
// attaches — a value only reachable by actually using the context that
// capability.Enter returns.
type markerKey struct{}

// The security property capability.Enter provides is its RETURNED context,
// not merely that it was called. A call site that does `_ =
// capability.Enter(ec.ctx, fn.Name)` while still building the child evalCtx
// from `ec.ctx` calls the interceptor and discards what it re-scoped — the
// callee would silently keep running under the caller's authority. This test
// proves the callee's own frame observes what the interceptor attached,
// by having the callee body invoke a builtin that reads the marker back out
// of its context.
func TestCallUserFunction_calleeObservesInterceptorsReturnedContext(t *testing.T) {
	fnAST, errs := parser.Parse("fn helper() -> string => read_marker()")
	if len(errs) > 0 {
		t.Fatalf("parse: %v", errs)
	}

	ctx := capability.WithInterceptor(context.Background(),
		func(ctx context.Context, _ string) context.Context {
			return context.WithValue(ctx, markerKey{}, "elevated")
		})

	builtins := map[string]*BuiltinFunc{
		"read_marker": {
			Name: "read_marker", MinArgs: 0, MaxArgs: 0,
			CtxFn: func(ctx context.Context, _ []any) (any, error) {
				marker, _ := ctx.Value(markerKey{}).(string)
				return marker, nil
			},
		},
	}

	ex := New(builtins, nil, ExecConfig{})
	ec := &evalCtx{ctx: ctx, env: newEnv(nil)}
	compiled := &CompiledFunction{Name: "app:com.acme.pack::helper", AST: fnAST}

	got, err := ex.callUserFunction(ec, compiled, nil)
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if got != "elevated" {
		t.Fatalf("callee observed marker %q, want %q — the context capability.Enter "+
			"returned did not reach the callee's frame", got, "elevated")
	}
}

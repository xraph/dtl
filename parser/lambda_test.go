package parser

import (
	"testing"

	"github.com/xraph/dtl/ast"
)

func parseFn(t *testing.T, src string) *ast.FnAST {
	t.Helper()
	fn, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("parse %q: %v", src, errs)
	}
	return fn
}

// A single parameter may be written without parentheses, which is the form the
// collection operations are documented with.
func TestBareLambdaParses(t *testing.T) {
	tests := []struct {
		name, src  string
		wantParams []string
	}{
		{"in an argument list", `fn f(a: any) -> any => map(a, x => x * 2)`, []string{"x"}},
		{"in a pipe", `fn f(a: any) -> any => a | map(x => x * 2)`, []string{"x"}},
		{"body is a call", `fn f(a: any) -> any => map(a, x => upper(x))`, []string{"x"}},
		{"body reads a field", `fn f(a: any) -> any => map(a, r => r.name)`, []string{"r"}},
		{"underscore-prefixed name", `fn f(a: any) -> any => map(a, _x => _x)`, []string{"_x"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := parseFn(t, tt.src)
			lambda := findLambda(t, fn)
			if len(lambda.Params) != len(tt.wantParams) || lambda.Params[0] != tt.wantParams[0] {
				t.Errorf("params = %v, want %v", lambda.Params, tt.wantParams)
			}
		})
	}
}

// The parenthesised and fn forms keep working and produce the same node, so
// existing sources are unaffected.
func TestParenthesisedLambdaFormsStillParse(t *testing.T) {
	for _, src := range []string{
		`fn f(a: any) -> any => map(a, (x) => x * 2)`,
		`fn f(a: any) -> any => map(a, fn (x) => x * 2)`,
		`fn f(a: any) -> any => reduce(a, 0, (acc, x) => acc + x)`,
		`fn f(a: any) -> any => map(a, () => 1)`,
	} {
		t.Run(src, func(t *testing.T) {
			fn := parseFn(t, src)
			findLambda(t, fn)
		})
	}
}

// Only an arrow directly after a plain identifier makes a lambda. An ordinary
// identifier, a call and a namespaced name must be untouched.
func TestIdentifiersAreNotMistakenForLambdas(t *testing.T) {
	for _, src := range []string{
		`fn f(x: any) -> any => x`,
		`fn f(x: any) -> any => upper(x)`,
		`fn f(x: any) -> any => path::get(x, "a")`,
		`fn f(x: any) -> any => x.field`,
		`fn f(x: any) -> any => x + 1`,
		`fn f(x: any) -> any => if x then 1 else 2`,
	} {
		t.Run(src, func(t *testing.T) {
			fn := parseFn(t, src)
			if lambdaIn(fn.Body) != nil {
				t.Errorf("%q was parsed as containing a lambda", src)
			}
		})
	}
}

// A match arm is `when <pattern> => <result>`. Patterns reach parsePrimary, so
// an identifier pattern is followed by an arrow exactly like a bare lambda —
// the arm must still win, or the arrow would swallow the arm's result.
func TestMatchArmsAreNotParsedAsLambdas(t *testing.T) {
	src := `fn f(v: any, limit: any) -> any:
    match v:
        when limit => "at the limit"
        when < 0 => "negative"
        when _ => "other"
`
	fn := parseFn(t, src)
	if lambdaIn(fn.Body) != nil {
		t.Error("a match arm was parsed as a lambda")
	}
}

// findLambda asserts the function contains a lambda and returns the first one.
func findLambda(t *testing.T, fn *ast.FnAST) *ast.LambdaExpr {
	t.Helper()
	if l := lambdaIn(fn.Body); l != nil {
		return l
	}
	t.Fatal("expected a lambda in the parsed function")
	return nil
}

func lambdaIn(stmts []ast.StmtNode) *ast.LambdaExpr {
	for _, s := range stmts {
		var e ast.ExprNode
		switch v := s.(type) {
		case *ast.ReturnStmt:
			e = v.Value
		case *ast.ExprStmt:
			e = v.Expr
		case *ast.LetStmt:
			e = v.Value
		}
		if l := lambdaInExpr(e); l != nil {
			return l
		}
	}
	return nil
}

func lambdaInExpr(e ast.ExprNode) *ast.LambdaExpr {
	switch v := e.(type) {
	case nil:
		return nil
	case *ast.LambdaExpr:
		return v
	case *ast.FnCallExpr:
		for _, a := range v.Args {
			if l := lambdaInExpr(a); l != nil {
				return l
			}
		}
	case *ast.BinaryExpr:
		if l := lambdaInExpr(v.Left); l != nil {
			return l
		}
		return lambdaInExpr(v.Right)
	case *ast.PipeExpr:
		if l := lambdaInExpr(v.Input); l != nil {
			return l
		}
		for _, a := range v.Args {
			if l := lambdaInExpr(a); l != nil {
				return l
			}
		}
	}
	return nil
}

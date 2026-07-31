package parser

import (
	"testing"

	"github.com/xraph/dtl/ast"
)

// --- Assertion helpers ---

func assertNoErrors(t testing.TB, errs []ParseError) {
	t.Helper()
	if len(errs) > 0 {
		t.Fatalf("unexpected parse errors: %v", errs)
	}
}

func assertHasErrors(t testing.TB, errs []ParseError) {
	t.Helper()
	if len(errs) == 0 {
		t.Fatal("expected parse errors, got none")
	}
}

// --- Parse() function definition tests ---

func TestParse_SimpleFn(t *testing.T) {
	src := "fn add(a: int, b: int) -> int => a + b"
	fn, errs := Parse(src)
	assertNoErrors(t, errs)

	if fn == nil {
		t.Fatal("expected non-nil FnAST")
	}
	if fn.Name != "add" {
		t.Errorf("name: got %q, want %q", fn.Name, "add")
	}
	if len(fn.Params) != 2 {
		t.Fatalf("params: got %d, want 2", len(fn.Params))
	}
	if fn.Params[0].Name != "a" || fn.Params[0].Type.Name != "int" {
		t.Errorf("param 0: got %q:%q, want a:int", fn.Params[0].Name, fn.Params[0].Type.Name)
	}
	if fn.Params[1].Name != "b" || fn.Params[1].Type.Name != "int" {
		t.Errorf("param 1: got %q:%q, want b:int", fn.Params[1].Name, fn.Params[1].Type.Name)
	}
	if fn.ReturnType.Name != "int" {
		t.Errorf("return type: got %q, want %q", fn.ReturnType.Name, "int")
	}
	if !fn.IsOneLiner {
		t.Error("expected IsOneLiner to be true")
	}
	if len(fn.Body) != 1 {
		t.Fatalf("body: got %d stmts, want 1", len(fn.Body))
	}
}

func TestParse_BlockBody(t *testing.T) {
	src := `fn greet(name: string) -> string:
  let msg = "hello"
  return msg`
	fn, errs := Parse(src)
	assertNoErrors(t, errs)

	if fn.Name != "greet" {
		t.Errorf("name: got %q, want %q", fn.Name, "greet")
	}
	if fn.IsOneLiner {
		t.Error("expected IsOneLiner to be false for block body")
	}
	if len(fn.Body) != 2 {
		t.Fatalf("body: got %d stmts, want 2", len(fn.Body))
	}

	// First should be LetStmt
	if _, ok := fn.Body[0].(*ast.LetStmt); !ok {
		t.Errorf("body[0]: expected LetStmt, got %T", fn.Body[0])
	}
	// Second should be ReturnStmt
	if _, ok := fn.Body[1].(*ast.ReturnStmt); !ok {
		t.Errorf("body[1]: expected ReturnStmt, got %T", fn.Body[1])
	}
}

func TestParse_ArrowBlockBody(t *testing.T) {
	// => followed by indented block should be treated as block body (not one-liner)
	src := `fn transform(data: any) -> any =>
  let result = data
  result`
	fn, errs := Parse(src)
	assertNoErrors(t, errs)

	if fn.Name != "transform" {
		t.Errorf("name: got %q, want %q", fn.Name, "transform")
	}
	if fn.IsOneLiner {
		t.Error("expected IsOneLiner to be false for => with block body")
	}
	if len(fn.Body) != 2 {
		t.Fatalf("body: got %d stmts, want 2", len(fn.Body))
	}

	// First should be LetStmt
	if _, ok := fn.Body[0].(*ast.LetStmt); !ok {
		t.Errorf("body[0]: expected LetStmt, got %T", fn.Body[0])
	}
	// Second should be ExprStmt (bare expression)
	if _, ok := fn.Body[1].(*ast.ExprStmt); !ok {
		t.Errorf("body[1]: expected ExprStmt, got %T", fn.Body[1])
	}
}

func TestParse_DefaultParam(t *testing.T) {
	src := "fn scale(x: float, factor: float = 1.0) -> float => x * factor"
	fn, errs := Parse(src)
	assertNoErrors(t, errs)

	if len(fn.Params) != 2 {
		t.Fatalf("params: got %d, want 2", len(fn.Params))
	}
	if fn.Params[0].Default != nil {
		t.Error("param 0 should have no default")
	}
	if fn.Params[1].Default == nil {
		t.Fatal("param 1 should have a default value")
	}
	lit, ok := fn.Params[1].Default.(*ast.LiteralExpr)
	if !ok {
		t.Fatalf("default: expected LiteralExpr, got %T", fn.Params[1].Default)
	}
	if lit.Value != 1.0 {
		t.Errorf("default value: got %v, want 1.0", lit.Value)
	}
}

func TestParse_VariadicParam(t *testing.T) {
	src := "fn sum(...vals: float) -> float => vals"
	fn, errs := Parse(src)
	assertNoErrors(t, errs)

	if len(fn.Params) != 1 {
		t.Fatalf("params: got %d, want 1", len(fn.Params))
	}
	if !fn.Params[0].Variadic {
		t.Error("expected variadic param")
	}
	if fn.Params[0].Name != "vals" {
		t.Errorf("param name: got %q, want %q", fn.Params[0].Name, "vals")
	}
}

func TestParse_ArrayReturnType(t *testing.T) {
	src := "fn getList() -> string[] => null"
	fn, errs := Parse(src)
	assertNoErrors(t, errs)

	if !fn.ReturnType.IsArray {
		t.Error("expected array return type")
	}
	if fn.ReturnType.Name != "string" {
		t.Errorf("return type: got %q, want %q", fn.ReturnType.Name, "string")
	}
}

func TestParse_NoParams(t *testing.T) {
	src := "fn now() -> float => 0"
	fn, errs := Parse(src)
	assertNoErrors(t, errs)

	if len(fn.Params) != 0 {
		t.Errorf("params: got %d, want 0", len(fn.Params))
	}
}

func TestParse_EmptySource(t *testing.T) {
	_, errs := Parse("")
	assertHasErrors(t, errs)
}

func TestParse_MissingFnKeyword(t *testing.T) {
	_, errs := Parse("add(a: int) -> int => a")
	assertHasErrors(t, errs)
}

func TestParse_MissingParens(t *testing.T) {
	_, errs := Parse("fn add a: int -> int => a")
	assertHasErrors(t, errs)
}

// --- ParseExpression tests ---

func TestParseExpression_IntLiteral(t *testing.T) {
	expr, errs := ParseExpression("42")
	assertNoErrors(t, errs)

	lit, ok := expr.(*ast.LiteralExpr)
	if !ok {
		t.Fatalf("expected LiteralExpr, got %T", expr)
	}
	if lit.Value != int64(42) {
		t.Errorf("got %v, want 42", lit.Value)
	}
	if lit.Type != "int" {
		t.Errorf("type: got %q, want %q", lit.Type, "int")
	}
}

func TestParseExpression_FloatLiteral(t *testing.T) {
	expr, errs := ParseExpression("3.14")
	assertNoErrors(t, errs)

	lit, ok := expr.(*ast.LiteralExpr)
	if !ok {
		t.Fatalf("expected LiteralExpr, got %T", expr)
	}
	if lit.Value != 3.14 {
		t.Errorf("got %v, want 3.14", lit.Value)
	}
}

func TestParseExpression_StringLiteral(t *testing.T) {
	expr, errs := ParseExpression(`"hello"`)
	assertNoErrors(t, errs)

	lit, ok := expr.(*ast.LiteralExpr)
	if !ok {
		t.Fatalf("expected LiteralExpr, got %T", expr)
	}
	if lit.Value != "hello" {
		t.Errorf("got %v, want hello", lit.Value)
	}
}

func TestParseExpression_BoolLiterals(t *testing.T) {
	tests := []struct {
		src  string
		want bool
	}{
		{"true", true},
		{"false", false},
	}
	for _, tc := range tests {
		t.Run(tc.src, func(t *testing.T) {
			expr, errs := ParseExpression(tc.src)
			assertNoErrors(t, errs)
			lit := expr.(*ast.LiteralExpr)
			if lit.Value != tc.want {
				t.Errorf("got %v, want %v", lit.Value, tc.want)
			}
		})
	}
}

func TestParseExpression_NullLiteral(t *testing.T) {
	expr, errs := ParseExpression("null")
	assertNoErrors(t, errs)

	lit := expr.(*ast.LiteralExpr)
	if lit.Value != nil {
		t.Errorf("got %v, want nil", lit.Value)
	}
}

func TestParseExpression_BinaryPrecedence(t *testing.T) {
	// 1 + 2 * 3 should parse as 1 + (2 * 3)
	expr, errs := ParseExpression("1 + 2 * 3")
	assertNoErrors(t, errs)

	add, ok := expr.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr(+), got %T", expr)
	}
	if add.Op != "+" {
		t.Errorf("top op: got %q, want %q", add.Op, "+")
	}

	// Right should be 2 * 3
	mul, ok := add.Right.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr(*) on right, got %T", add.Right)
	}
	if mul.Op != "*" {
		t.Errorf("right op: got %q, want %q", mul.Op, "*")
	}
}

func TestParseExpression_UnaryMinus(t *testing.T) {
	expr, errs := ParseExpression("-x")
	assertNoErrors(t, errs)

	unary, ok := expr.(*ast.UnaryExpr)
	if !ok {
		t.Fatalf("expected UnaryExpr, got %T", expr)
	}
	if unary.Op != "-" {
		t.Errorf("op: got %q, want %q", unary.Op, "-")
	}
	ident, ok := unary.Operand.(*ast.IdentExpr)
	if !ok {
		t.Fatalf("expected IdentExpr, got %T", unary.Operand)
	}
	if ident.Name != "x" {
		t.Errorf("name: got %q, want %q", ident.Name, "x")
	}
}

func TestParseExpression_UnaryNot(t *testing.T) {
	expr, errs := ParseExpression("!flag")
	assertNoErrors(t, errs)

	unary, ok := expr.(*ast.UnaryExpr)
	if !ok {
		t.Fatalf("expected UnaryExpr, got %T", expr)
	}
	if unary.Op != "!" {
		t.Errorf("op: got %q, want %q", unary.Op, "!")
	}
}

func TestParseExpression_NotKeyword(t *testing.T) {
	expr, errs := ParseExpression("not flag")
	assertNoErrors(t, errs)

	unary, ok := expr.(*ast.UnaryExpr)
	if !ok {
		t.Fatalf("expected UnaryExpr, got %T", expr)
	}
	if unary.Op != "!" {
		t.Errorf("op: got %q, want %q (not keyword normalizes to !)", unary.Op, "!")
	}
}

func TestParseExpression_IfThenElse(t *testing.T) {
	expr, errs := ParseExpression("if x then 1 else 2")
	assertNoErrors(t, errs)

	ifExpr, ok := expr.(*ast.IfExpr)
	if !ok {
		t.Fatalf("expected IfExpr, got %T", expr)
	}
	if ifExpr.Else == nil {
		t.Error("expected else branch")
	}
}

func TestParseExpression_IfWithoutElse(t *testing.T) {
	expr, errs := ParseExpression("if x then 1")
	assertNoErrors(t, errs)

	ifExpr, ok := expr.(*ast.IfExpr)
	if !ok {
		t.Fatalf("expected IfExpr, got %T", expr)
	}
	if ifExpr.Else != nil {
		t.Error("expected no else branch")
	}
}

func TestParseExpression_MatchExpr(t *testing.T) {
	src := `match x:
  when 1 => "one"
  when 2 => "two"
  when _ => "other"`
	expr, errs := ParseExpression(src)
	assertNoErrors(t, errs)

	matchExpr, ok := expr.(*ast.MatchExpr)
	if !ok {
		t.Fatalf("expected MatchExpr, got %T", expr)
	}
	if len(matchExpr.Arms) != 3 {
		t.Fatalf("arms: got %d, want 3", len(matchExpr.Arms))
	}
	// Last arm should be wildcard
	if _, ok := matchExpr.Arms[2].Pattern.(*ast.WildcardPattern); !ok {
		t.Errorf("arm 2 pattern: expected WildcardPattern, got %T", matchExpr.Arms[2].Pattern)
	}
}

func TestParseExpression_TryCatch(t *testing.T) {
	expr, errs := ParseExpression("try risky() catch null")
	assertNoErrors(t, errs)

	tryExpr, ok := expr.(*ast.TryExpr)
	if !ok {
		t.Fatalf("expected TryExpr, got %T", expr)
	}
	if tryExpr.Default == nil {
		t.Error("expected default/catch expression")
	}
}

func TestParseExpression_TryWithoutCatch(t *testing.T) {
	expr, errs := ParseExpression("try risky()")
	assertNoErrors(t, errs)

	tryExpr, ok := expr.(*ast.TryExpr)
	if !ok {
		t.Fatalf("expected TryExpr, got %T", expr)
	}
	if tryExpr.Default != nil {
		t.Error("expected no default when no catch clause")
	}
}

func TestParseExpression_Lambda(t *testing.T) {
	expr, errs := ParseExpression("(x) => x + 1")
	assertNoErrors(t, errs)

	lambda, ok := expr.(*ast.LambdaExpr)
	if !ok {
		t.Fatalf("expected LambdaExpr, got %T", expr)
	}
	if len(lambda.Params) != 1 || lambda.Params[0] != "x" {
		t.Errorf("params: got %v, want [x]", lambda.Params)
	}
}

func TestParseExpression_LambdaMultiParam(t *testing.T) {
	expr, errs := ParseExpression("(a, b) => a + b")
	assertNoErrors(t, errs)

	lambda, ok := expr.(*ast.LambdaExpr)
	if !ok {
		t.Fatalf("expected LambdaExpr, got %T", expr)
	}
	if len(lambda.Params) != 2 {
		t.Fatalf("params: got %d, want 2", len(lambda.Params))
	}
}

func TestParseExpression_LambdaNoParams(t *testing.T) {
	expr, errs := ParseExpression("() => 42")
	assertNoErrors(t, errs)

	lambda, ok := expr.(*ast.LambdaExpr)
	if !ok {
		t.Fatalf("expected LambdaExpr, got %T", expr)
	}
	if len(lambda.Params) != 0 {
		t.Errorf("params: got %d, want 0", len(lambda.Params))
	}
}

func TestParseExpression_Pipe(t *testing.T) {
	expr, errs := ParseExpression("data | filter()")
	assertNoErrors(t, errs)

	pipe, ok := expr.(*ast.PipeExpr)
	if !ok {
		t.Fatalf("expected PipeExpr, got %T", expr)
	}
	if pipe.Function != "filter" {
		t.Errorf("function: got %q, want %q", pipe.Function, "filter")
	}
}

func TestParseExpression_PipeChain(t *testing.T) {
	expr, errs := ParseExpression("data | filter() | sort()")
	assertNoErrors(t, errs)

	// Outermost should be the last pipe (left-associative)
	pipe2, ok := expr.(*ast.PipeExpr)
	if !ok {
		t.Fatalf("expected PipeExpr, got %T", expr)
	}
	if pipe2.Function != "sort" {
		t.Errorf("outer function: got %q, want %q", pipe2.Function, "sort")
	}

	pipe1, ok := pipe2.Input.(*ast.PipeExpr)
	if !ok {
		t.Fatalf("expected PipeExpr for inner, got %T", pipe2.Input)
	}
	if pipe1.Function != "filter" {
		t.Errorf("inner function: got %q, want %q", pipe1.Function, "filter")
	}
}

func TestParseExpression_FunctionCall(t *testing.T) {
	expr, errs := ParseExpression("sqrt(16)")
	assertNoErrors(t, errs)

	call, ok := expr.(*ast.FnCallExpr)
	if !ok {
		t.Fatalf("expected FnCallExpr, got %T", expr)
	}
	if call.Name != "sqrt" {
		t.Errorf("name: got %q, want %q", call.Name, "sqrt")
	}
	if len(call.Args) != 1 {
		t.Fatalf("args: got %d, want 1", len(call.Args))
	}
}

func TestParseExpression_NamespacedCall(t *testing.T) {
	expr, errs := ParseExpression("math::sqrt(16)")
	assertNoErrors(t, errs)

	call, ok := expr.(*ast.FnCallExpr)
	if !ok {
		t.Fatalf("expected FnCallExpr, got %T", expr)
	}
	if call.Name != "math::sqrt" {
		t.Errorf("name: got %q, want %q", call.Name, "math::sqrt")
	}
}

func TestParseExpression_NamespacedCall_PrefixColon(t *testing.T) {
	tests := []struct {
		input    string
		wantName string
	}{
		{"team:analytics::score(x)", "team:analytics::score"},
		{"user:john::helper(x, y)", "user:john::helper"},
		{"app:myapp::transform(data)", "app:myapp::transform"},
		{"shared::simple(x)", "shared::simple"},
		{"system::collections::top_n(arr, 5)", "system::collections::top_n"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			expr, errs := ParseExpression(tt.input)
			assertNoErrors(t, errs)

			call, ok := expr.(*ast.FnCallExpr)
			if !ok {
				t.Fatalf("expected FnCallExpr, got %T", expr)
			}
			if call.Name != tt.wantName {
				t.Errorf("name: got %q, want %q", call.Name, tt.wantName)
			}
		})
	}
}

func TestParseExpression_FieldAccess(t *testing.T) {
	expr, errs := ParseExpression("obj.field")
	assertNoErrors(t, errs)

	fa, ok := expr.(*ast.FieldAccessExpr)
	if !ok {
		t.Fatalf("expected FieldAccessExpr, got %T", expr)
	}
	if fa.Field != "field" {
		t.Errorf("field: got %q, want %q", fa.Field, "field")
	}
	if fa.Optional {
		t.Error("expected non-optional field access")
	}
}

func TestParseExpression_OptionalChaining(t *testing.T) {
	expr, errs := ParseExpression("obj?.field")
	assertNoErrors(t, errs)

	fa, ok := expr.(*ast.FieldAccessExpr)
	if !ok {
		t.Fatalf("expected FieldAccessExpr, got %T", expr)
	}
	if !fa.Optional {
		t.Error("expected optional field access")
	}
}

func TestParseExpression_IndexAccess(t *testing.T) {
	expr, errs := ParseExpression("arr[0]")
	assertNoErrors(t, errs)

	idx, ok := expr.(*ast.IndexExpr)
	if !ok {
		t.Fatalf("expected IndexExpr, got %T", expr)
	}
	lit, ok := idx.Index.(*ast.LiteralExpr)
	if !ok {
		t.Fatalf("expected LiteralExpr index, got %T", idx.Index)
	}
	if lit.Value != int64(0) {
		t.Errorf("index: got %v, want 0", lit.Value)
	}
}

func TestParseExpression_ArrayLiteral(t *testing.T) {
	expr, errs := ParseExpression("[1, 2, 3]")
	assertNoErrors(t, errs)

	arr, ok := expr.(*ast.ArrayExpr)
	if !ok {
		t.Fatalf("expected ArrayExpr, got %T", expr)
	}
	if len(arr.Elements) != 3 {
		t.Errorf("elements: got %d, want 3", len(arr.Elements))
	}
}

func TestParseExpression_ObjectLiteral(t *testing.T) {
	expr, errs := ParseExpression(`{x: 1, y: 2}`)
	assertNoErrors(t, errs)

	obj, ok := expr.(*ast.ObjectExpr)
	if !ok {
		t.Fatalf("expected ObjectExpr, got %T", expr)
	}
	if len(obj.Fields) != 2 {
		t.Fatalf("fields: got %d, want 2", len(obj.Fields))
	}
	if obj.Fields[0].Key != "x" || obj.Fields[1].Key != "y" {
		t.Errorf("keys: got %q,%q, want x,y", obj.Fields[0].Key, obj.Fields[1].Key)
	}
}

func TestParseExpression_NullCoalescing(t *testing.T) {
	expr, errs := ParseExpression("a ?? b")
	assertNoErrors(t, errs)

	coalesce, ok := expr.(*ast.CoalesceExpr)
	if !ok {
		t.Fatalf("expected CoalesceExpr, got %T", expr)
	}
	_ = coalesce
}

func TestParseExpression_GroupedExpression(t *testing.T) {
	// (1 + 2) * 3 — parens should group
	expr, errs := ParseExpression("(1 + 2) * 3")
	assertNoErrors(t, errs)

	mul, ok := expr.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr(*), got %T", expr)
	}
	if mul.Op != "*" {
		t.Errorf("top op: got %q, want %q", mul.Op, "*")
	}
	// Left should be the grouped addition
	add, ok := mul.Left.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr(+) on left, got %T", mul.Left)
	}
	if add.Op != "+" {
		t.Errorf("left op: got %q, want %q", add.Op, "+")
	}
}

func TestParseExpression_EmptySource(t *testing.T) {
	_, errs := ParseExpression("")
	assertHasErrors(t, errs)
}

func TestParseExpression_QueryExpr(t *testing.T) {
	expr, errs := ParseExpression(`query("sensors")`)
	assertNoErrors(t, errs)

	q, ok := expr.(*ast.QueryExpr)
	if !ok {
		t.Fatalf("expected QueryExpr, got %T", expr)
	}
	lit, ok := q.Dataset.(*ast.LiteralExpr)
	if !ok {
		t.Fatalf("expected LiteralExpr dataset, got %T", q.Dataset)
	}
	if lit.Value != "sensors" {
		t.Errorf("dataset: got %v, want sensors", lit.Value)
	}
}

// TestParseExpression_DatasetQuerySugar verifies that `dataset::query(...)` is
// grammar-level sugar for the `query` keyword and produces the same QueryExpr.
func TestParseExpression_DatasetQuerySugar(t *testing.T) {
	expr, errs := ParseExpression(`dataset::query("sensors")`)
	assertNoErrors(t, errs)

	q, ok := expr.(*ast.QueryExpr)
	if !ok {
		t.Fatalf("expected QueryExpr, got %T", expr)
	}
	lit, ok := q.Dataset.(*ast.LiteralExpr)
	if !ok {
		t.Fatalf("expected LiteralExpr dataset, got %T", q.Dataset)
	}
	if lit.Value != "sensors" {
		t.Errorf("dataset: got %v, want sensors", lit.Value)
	}
}

func TestParseExpression_InterpolatedString(t *testing.T) {
	expr, errs := ParseExpression(`"hello {name}"`)
	assertNoErrors(t, errs)

	interp, ok := expr.(*ast.InterpolatedStringExpr)
	if !ok {
		t.Fatalf("expected InterpolatedStringExpr, got %T", expr)
	}
	if len(interp.Parts) < 2 {
		t.Fatalf("parts: got %d, want at least 2", len(interp.Parts))
	}
}

func TestParseExpression_LogicalAnd(t *testing.T) {
	expr, errs := ParseExpression("a && b")
	assertNoErrors(t, errs)

	bin, ok := expr.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", expr)
	}
	if bin.Op != "&&" {
		t.Errorf("op: got %q, want %q", bin.Op, "&&")
	}
}

func TestParseExpression_LogicalOr(t *testing.T) {
	expr, errs := ParseExpression("a || b")
	assertNoErrors(t, errs)

	bin, ok := expr.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", expr)
	}
	if bin.Op != "||" {
		t.Errorf("op: got %q, want %q", bin.Op, "||")
	}
}

func TestParseExpression_MatchWithRange(t *testing.T) {
	src := `match score:
  when 1..50 => "low"
  when 51..100 => "high"`
	expr, errs := ParseExpression(src)
	assertNoErrors(t, errs)

	matchExpr, ok := expr.(*ast.MatchExpr)
	if !ok {
		t.Fatalf("expected MatchExpr, got %T", expr)
	}
	if len(matchExpr.Arms) != 2 {
		t.Fatalf("arms: got %d, want 2", len(matchExpr.Arms))
	}
	// First arm should have a range pattern
	if _, ok := matchExpr.Arms[0].Pattern.(*ast.RangePattern); !ok {
		t.Errorf("arm 0 pattern: expected RangePattern, got %T", matchExpr.Arms[0].Pattern)
	}
}

func TestParseExpression_MatchWithComparison(t *testing.T) {
	src := `match x:
  when < 0 => "negative"
  when >= 0 => "non-negative"`
	expr, errs := ParseExpression(src)
	assertNoErrors(t, errs)

	matchExpr, ok := expr.(*ast.MatchExpr)
	if !ok {
		t.Fatalf("expected MatchExpr, got %T", expr)
	}
	if len(matchExpr.Arms) != 2 {
		t.Fatalf("arms: got %d, want 2", len(matchExpr.Arms))
	}
	cp, ok := matchExpr.Arms[0].Pattern.(*ast.ComparisonPattern)
	if !ok {
		t.Fatalf("arm 0: expected ComparisonPattern, got %T", matchExpr.Arms[0].Pattern)
	}
	if cp.Op != "<" {
		t.Errorf("op: got %q, want %q", cp.Op, "<")
	}
}

func TestParseError_Error(t *testing.T) {
	_, errs := Parse("")
	if len(errs) == 0 {
		t.Fatal("expected errors")
	}
	msg := errs[0].Error()
	if msg == "" {
		t.Error("error message should not be empty")
	}
}

// --- for..in, in, raise, use expression/statement tests ---

func TestParseExpression_ForIn(t *testing.T) {
	expr, errs := ParseExpression("for x in items: x * 2")
	assertNoErrors(t, errs)
	forExpr, ok := expr.(*ast.ForExpr)
	if !ok {
		t.Fatalf("expected ForExpr, got %T", expr)
	}
	if forExpr.Variable != "x" {
		t.Errorf("variable: got %q, want %q", forExpr.Variable, "x")
	}
	if forExpr.Index != "" {
		t.Errorf("index: got %q, want empty", forExpr.Index)
	}
}

func TestParseExpression_ForInWithIndex(t *testing.T) {
	expr, errs := ParseExpression("for item, idx in list: idx")
	assertNoErrors(t, errs)
	forExpr, ok := expr.(*ast.ForExpr)
	if !ok {
		t.Fatalf("expected ForExpr, got %T", expr)
	}
	if forExpr.Variable != "item" {
		t.Errorf("variable: got %q, want %q", forExpr.Variable, "item")
	}
	if forExpr.Index != "idx" {
		t.Errorf("index: got %q, want %q", forExpr.Index, "idx")
	}
}

func TestParseExpression_InOperator(t *testing.T) {
	expr, errs := ParseExpression(`status in ["active", "pending"]`)
	assertNoErrors(t, errs)
	inExpr, ok := expr.(*ast.InExpr)
	if !ok {
		t.Fatalf("expected InExpr, got %T", expr)
	}
	// Value should be an IdentExpr
	ident, ok := inExpr.Value.(*ast.IdentExpr)
	if !ok {
		t.Fatalf("expected IdentExpr for value, got %T", inExpr.Value)
	}
	if ident.Name != "status" {
		t.Errorf("value name: got %q, want %q", ident.Name, "status")
	}
	// Collection should be ArrayExpr
	arr, ok := inExpr.Collection.(*ast.ArrayExpr)
	if !ok {
		t.Fatalf("expected ArrayExpr for collection, got %T", inExpr.Collection)
	}
	if len(arr.Elements) != 2 {
		t.Errorf("array elements: got %d, want 2", len(arr.Elements))
	}
}

func TestParseExpression_Raise(t *testing.T) {
	expr, errs := ParseExpression(`raise "something went wrong"`)
	assertNoErrors(t, errs)
	raiseExpr, ok := expr.(*ast.RaiseExpr)
	if !ok {
		t.Fatalf("expected RaiseExpr, got %T", expr)
	}
	if raiseExpr.Message == nil {
		t.Fatal("expected non-nil message")
	}
}

func TestParse_UseStatement(t *testing.T) {
	src := `fn f() -> int:
  use maintenance
  assign_work_order("WO-1")`
	fn, errs := Parse(src)
	assertNoErrors(t, errs)
	if len(fn.Body) != 2 {
		t.Fatalf("body: got %d stmts, want 2", len(fn.Body))
	}
	useStmt, ok := fn.Body[0].(*ast.UseStmt)
	if !ok {
		t.Fatalf("expected UseStmt, got %T", fn.Body[0])
	}
	if useStmt.Namespace != "maintenance" {
		t.Errorf("namespace: got %q, want %q", useStmt.Namespace, "maintenance")
	}
}

// Pack ids are dotted (com.example.core.shifts) and '.' is not an identifier
// character, so a namespace arrives as IDENT (DOT IDENT)*. Before this was
// handled, `use com.example.core.shifts` bound the namespace to "com" and every
// cross-pack call resolved against app:com::<name>, which matches nothing.
func TestParseUseStmt_DottedNamespace(t *testing.T) {
	src := "fn f() -> string:\n  use com.example.core.shifts\n  return \"ok\"\n"
	fn, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}
	var got string
	for _, stmt := range fn.Body {
		if u, ok := stmt.(*ast.UseStmt); ok {
			got = u.Namespace
		}
	}
	if got != "com.example.core.shifts" {
		t.Errorf("expected the whole dotted namespace, got %q", got)
	}
}

// The single-segment form is what every existing caller would use; it must not
// regress.
func TestParseUseStmt_SingleSegmentStillParses(t *testing.T) {
	src := "fn f() -> string:\n  use shared\n  return \"ok\"\n"
	fn, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}
	for _, stmt := range fn.Body {
		if u, ok := stmt.(*ast.UseStmt); ok && u.Namespace != "shared" {
			t.Errorf("expected \"shared\", got %q", u.Namespace)
		}
	}
}

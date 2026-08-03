package registry_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/xraph/dtl/registry"
)

func run(t *testing.T, src string, args map[string]any) any {
	t.Helper()
	reg := registry.New(registry.Config{})
	if err := reg.Register("f", src); err != nil {
		t.Fatalf("register %q: %v", src, err)
	}
	res, err := reg.Execute(context.Background(), "f", args)
	if err != nil {
		t.Fatalf("execute %q: %v", src, err)
	}
	return res.Value
}

// The collection examples in SPEC.md, executed. They are the language's
// headline ergonomics, and every one of them failed to parse before the bare
// lambda and the implicit-argument shorthand existed — this is what keeps the
// documentation and the parser from drifting apart again.
func TestSpecCollectionExamplesExecute(t *testing.T) {
	values := []any{1.0, 2.0, 200.0}

	tests := []struct {
		specLine, expr string
		want           any
	}{
		{"154", `round(avg(filter(values, x => x > 0)), 2)`, 67.67},
		{"156", `values | filter(> 0) | avg() | round(2)`, 67.67},
		{"173", `values | map(x => x * 2)`, []any{2.0, 4.0, 400.0}},
		{"174", `values | filter(x => x > 1)`, []any{2.0, 200.0}},
		{"175", `values | filter(> 0)`, []any{1.0, 2.0, 200.0}},
		{"176", `values | reduce(0, (acc, x) => acc + x)`, 203.0},
		{"200", `values | count_where(> 100)`, int64(1)},
		{"201", `values | sum_where(> 0)`, 203.0},
	}

	for _, tt := range tests {
		t.Run("SPEC:"+tt.specLine, func(t *testing.T) {
			got := run(t, "fn f(values: any) -> any => "+tt.expr, map[string]any{"values": values})
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("%s\n got  %#v\n want %#v", tt.expr, got, tt.want)
			}
		})
	}
}

// Every comparison operator is accepted in the shorthand, not just the ones the
// documentation happens to show.
func TestImplicitArgumentShorthandOperators(t *testing.T) {
	values := []any{1.0, 2.0, 3.0}

	tests := []struct {
		expr string
		want any
	}{
		{`filter(values, > 2)`, []any{3.0}},
		{`filter(values, < 2)`, []any{1.0}},
		{`filter(values, >= 2)`, []any{2.0, 3.0}},
		{`filter(values, <= 2)`, []any{1.0, 2.0}},
		{`filter(values, == 2)`, []any{2.0}},
		{`filter(values, != 2)`, []any{1.0, 3.0}},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got := run(t, "fn f(values: any) -> any => "+tt.expr, map[string]any{"values": values})
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

// The shorthand's parameter must not be a name an author could write, or the
// body would capture it. With a parameter called `it`, `filter(> it)` beside an
// outer `it` would compile to `it > it` and compare every element with itself.
func TestShorthandDoesNotCaptureOuterVariables(t *testing.T) {
	for _, name := range []string{"it", "x", "_", "v"} {
		t.Run(name, func(t *testing.T) {
			src := "fn f(values: any, " + name + ": any) -> any => filter(values, > " + name + ")"
			got := run(t, src, map[string]any{
				"values": []any{1.0, 2.0, 3.0},
				name:     2.0,
			})
			want := []any{3.0}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("outer %q was captured: got %#v, want %#v", name, got, want)
			}
		})
	}
}

// The shorthand's right-hand side is a full expression, not just a literal.
func TestShorthandAcceptsExpressionsOnTheRight(t *testing.T) {
	got := run(t,
		`fn f(values: any, n: any) -> any => filter(values, > n * 2)`,
		map[string]any{"values": []any{1.0, 5.0, 9.0}, "n": 2.0})
	if want := []any{5.0, 9.0}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

// A match arm ends in the same arrow a bare lambda uses. An identifier pattern
// must still be read as a pattern.
func TestMatchArmsStillWork(t *testing.T) {
	src := `fn f(v: any, limit: any) -> any:
    match v:
        when limit => "at the limit"
        when < 0 => "negative"
        when _ => "other"
`
	tests := []struct {
		v, limit float64
		want     any
	}{
		{5, 5, "at the limit"},
		{-1, 5, "negative"},
		{3, 5, "other"},
	}

	for _, tt := range tests {
		got := run(t, src, map[string]any{"v": tt.v, "limit": tt.limit})
		if got != tt.want {
			t.Errorf("v=%v limit=%v: got %#v, want %#v", tt.v, tt.limit, got, tt.want)
		}
	}
}

// Bare and parenthesised forms are the same lambda, so a source written either
// way behaves identically.
func TestBareAndParenthesisedLambdasAgree(t *testing.T) {
	values := map[string]any{"values": []any{1.0, 2.0, 3.0}}

	pairs := [][2]string{
		{`map(values, x => x * 2)`, `map(values, (x) => x * 2)`},
		{`filter(values, x => x > 1)`, `filter(values, (x) => x > 1)`},
		{`filter(values, > 1)`, `filter(values, (x) => x > 1)`},
		{`values | map(x => x + 1)`, `values | map((x) => x + 1)`},
	}

	for _, p := range pairs {
		t.Run(p[0], func(t *testing.T) {
			bare := run(t, "fn f(values: any) -> any => "+p[0], values)
			paren := run(t, "fn f(values: any) -> any => "+p[1], values)
			if !reflect.DeepEqual(bare, paren) {
				t.Errorf("%q gave %#v but %q gave %#v", p[0], bare, p[1], paren)
			}
		})
	}
}

// Lambdas nest, and an inner bare lambda must bind its own parameter rather
// than the outer one.
func TestNestedBareLambdas(t *testing.T) {
	got := run(t,
		`fn f(rows: any) -> any => map(rows, r => sum(map(r, n => n * 2)))`,
		map[string]any{"rows": []any{
			[]any{1.0, 2.0},
			[]any{3.0},
		}})
	if want := []any{6.0, 6.0}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

// group_by and distinct_by take a lambda; sort_by and top_n take a string key.
// The library is genuinely inconsistent here, so each is pinned rather than
// assumed — the documentation for two of them said the opposite until the
// bare-lambda work made the documented examples runnable and exposed it.
func TestKeySelectorConventions(t *testing.T) {
	rows := []any{
		map[string]any{"name": "a", "cat": "x", "n": 3.0},
		map[string]any{"name": "b", "cat": "y", "n": 1.0},
		map[string]any{"name": "c", "cat": "x", "n": 2.0},
	}
	args := map[string]any{"rows": rows}

	t.Run("group_by takes a lambda", func(t *testing.T) {
		got := run(t, `fn f(rows: any) -> any => len(path::get(group_by(rows, r => r.cat), "x"))`, args)
		if got != int64(2) {
			t.Errorf("got %#v, want 2", got)
		}
	})

	t.Run("distinct_by takes a lambda", func(t *testing.T) {
		got := run(t, `fn f(rows: any) -> any => len(distinct_by(rows, r => r.cat))`, args)
		if got != int64(2) {
			t.Errorf("got %#v, want 2", got)
		}
	})

	t.Run("sort_by takes a string key", func(t *testing.T) {
		got := run(t, `fn f(rows: any) -> any => path::get(first(sort_by(rows, "n")), "name")`, args)
		if got != "b" {
			t.Errorf("got %#v, want b", got)
		}
	})

	t.Run("top_n takes a string key", func(t *testing.T) {
		got := run(t, `fn f(rows: any) -> any => path::get(first(top_n(rows, 1, "n")), "name")`, args)
		if got != "a" {
			t.Errorf("got %#v, want a", got)
		}
	})
}

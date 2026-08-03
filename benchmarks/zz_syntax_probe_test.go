package benchmarks

import (
	"context"
	"testing"

	"github.com/xraph/dtl/registry"
)

// Probe which DTL spellings actually compile+run, so the benchmark workloads
// are built on verified syntax rather than guesses.
func TestProbeSyntax(t *testing.T) {
	cases := []struct {
		name string
		src  string
		args map[string]any
	}{
		{"arith", `fn f(a: float, b: float) -> float => a + b * 2 - 1`,
			map[string]any{"a": 10.0, "b": 5.0}},

		{"cond_if", `fn f(x: float) -> string => if x > 100 then "high" else if x > 10 then "mid" else "low"`,
			map[string]any{"x": 42.0}},

		{"cond_match", `fn f(x: float) -> string:
    match x:
        when > 100 => "high"
        when > 10  => "mid"
        when _     => "low"`,
			map[string]any{"x": 42.0}},

		{"string_concat", `fn f(a: string, b: string) -> string => upper(a) + " " + b`,
			map[string]any{"a": "ada", "b": "lovelace"}},

		{"string_interp", `fn f(a: string, b: string) -> string => "{upper(a)} {b}"`,
			map[string]any{"a": "ada", "b": "lovelace"}},

		{"field", `fn f(o: object) -> any => o.user.address.city`,
			map[string]any{"o": map[string]any{"user": map[string]any{"address": map[string]any{"city": "London"}}}}},

		{"coll_lambda", `fn f(xs: float[]) -> float => sum(filter(xs, (x) => x > 10.0))`,
			map[string]any{"xs": []any{5.0, 15.0, 25.0}}},

		{"coll_pipe", `fn f(xs: float[]) -> float => xs | filter((x) => x > 10.0) | sum`,
			map[string]any{"xs": []any{5.0, 15.0, 25.0}}},

		{"coll_reduce", `fn f(xs: float[]) -> float => xs | filter((x) => x > 10.0) | reduce(0, (acc, x) => acc + x)`,
			map[string]any{"xs": []any{5.0, 15.0, 25.0}}},
	}

	reg := registry.New(registry.Config{})
	for _, c := range cases {
		if err := reg.Register(c.name, c.src); err != nil {
			t.Logf("%-14s REGISTER FAIL: %v", c.name, err)
			continue
		}
		res, err := reg.Execute(context.Background(), c.name, c.args)
		if err != nil {
			t.Logf("%-14s EXEC FAIL: %v", c.name, err)
			continue
		}
		t.Logf("%-14s OK -> %#v", c.name, res.Value)
	}
}

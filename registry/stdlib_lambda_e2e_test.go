package registry_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/xraph/dtl/registry"
)

// The higher-order builtins take a DTL lambda, which only the executor can
// construct. They are therefore exercised here, through a real compile and
// execute, rather than by calling the Go function directly — a package-level
// unit test cannot build the argument they need.
func TestHigherOrderBuiltins(t *testing.T) {
	rows := []any{
		map[string]any{"name": "a", "team": "x", "score": 10.0},
		map[string]any{"name": "b", "team": "y", "score": 30.0},
		map[string]any{"name": "c", "team": "x", "score": 20.0},
	}

	cases := []struct {
		name, src string
		args      map[string]any
		want      any
	}{
		{
			"flat_map concatenates the returned arrays",
			`fn f(a: any) -> any => flat_map(a, (x) => [x, x])`,
			map[string]any{"a": []any{int64(1), int64(2)}},
			[]any{int64(1), int64(1), int64(2), int64(2)},
		},
		{
			"flat_map keeps a bare result as one element",
			`fn f(a: any) -> any => flat_map(a, (x) => x)`,
			map[string]any{"a": []any{int64(1), int64(2)}},
			[]any{int64(1), int64(2)},
		},
		{
			"partition splits into matching and rest",
			`fn f(a: any) -> any => partition(a, (x) => x > 1)`,
			map[string]any{"a": []any{int64(1), int64(2), int64(3)}},
			[]any{[]any{int64(2), int64(3)}, []any{int64(1)}},
		},
		{
			"index_by keys each element",
			`fn f(a: any) -> any => path::get(index_by(a, (r) => r.name), "b.score")`,
			map[string]any{"a": rows},
			30.0,
		},
		{
			"count_by counts per key",
			`fn f(a: any) -> any => path::get(count_by(a, (r) => r.team), "x")`,
			map[string]any{"a": rows},
			int64(2),
		},
		{
			"sum_by sums a projection",
			`fn f(a: any) -> any => sum_by(a, (r) => r.score)`,
			map[string]any{"a": rows},
			60.0,
		},
		{
			"avg_by averages a projection",
			`fn f(a: any) -> any => avg_by(a, (r) => r.score)`,
			map[string]any{"a": rows},
			20.0,
		},
		{
			"avg_by of an empty array is zero",
			`fn f(a: any) -> any => avg_by(a, (r) => r.score)`,
			map[string]any{"a": []any{}},
			0.0,
		},
		// min_by and max_by return the element, not the projected value —
		// which is the whole reason they exist alongside min(map(...)).
		{
			"min_by returns the element",
			`fn f(a: any) -> any => path::get(min_by(a, (r) => r.score), "name")`,
			map[string]any{"a": rows},
			"a",
		},
		{
			"max_by returns the element",
			`fn f(a: any) -> any => path::get(max_by(a, (r) => r.score), "name")`,
			map[string]any{"a": rows},
			"b", // scores are a=10, b=30, c=20
		},
		{
			"map_values transforms values, keeping keys",
			`fn f(o: object) -> any => path::get(map_values(o, (v) => v * 2), "a")`,
			map[string]any{"o": map[string]any{"a": 5.0}},
			10.0,
		},
		{
			"map_keys transforms keys, keeping values",
			`fn f(o: object) -> any => path::get(map_keys(o, (k) => upper(k)), "A")`,
			map[string]any{"o": map[string]any{"a": int64(1)}},
			int64(1),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reg := registry.New(registry.Config{})
			if err := reg.Register("f", c.src); err != nil {
				t.Fatalf("register: %v", err)
			}
			res, err := reg.Execute(context.Background(), "f", c.args)
			if err != nil {
				t.Fatalf("execute: %v", err)
			}
			if !reflect.DeepEqual(res.Value, c.want) {
				t.Errorf("got %#v (%T), want %#v (%T)", res.Value, res.Value, c.want, c.want)
			}
		})
	}
}

// min_by and max_by return null on an empty array rather than a zero element,
// so a caller can tell "no rows" from "a row scoring zero".
func TestMinByMaxByOnEmptyInput(t *testing.T) {
	for _, fn := range []string{"min_by", "max_by"} {
		t.Run(fn, func(t *testing.T) {
			reg := registry.New(registry.Config{})
			src := `fn f(a: any) -> any => ` + fn + `(a, (r) => r.score)`
			if err := reg.Register("f", src); err != nil {
				t.Fatalf("register: %v", err)
			}
			res, err := reg.Execute(context.Background(), "f", map[string]any{"a": []any{}})
			if err != nil {
				t.Fatalf("execute: %v", err)
			}
			if res.Value != nil {
				t.Errorf("got %#v, want nil", res.Value)
			}
		})
	}
}

// partition reads predicate results the same way filter does, so the two agree
// about which elements match. If they diverged, partition's two halves would
// not reconstruct what filter selects.
func TestPartitionAgreesWithFilter(t *testing.T) {
	reg := registry.New(registry.Config{})
	srcs := map[string]string{
		"part": `fn part(a: any) -> any => first(partition(a, (x) => x > 2))`,
		"filt": `fn filt(a: any) -> any => filter(a, (x) => x > 2)`,
	}
	for name, src := range srcs {
		if err := reg.Register(name, src); err != nil {
			t.Fatal(err)
		}
	}

	args := map[string]any{"a": []any{int64(1), int64(2), int64(3), int64(4)}}
	part, err := reg.Execute(context.Background(), "part", args)
	if err != nil {
		t.Fatal(err)
	}
	filt, err := reg.Execute(context.Background(), "filt", args)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(part.Value, filt.Value) {
		t.Errorf("partition's matching half = %#v but filter = %#v", part.Value, filt.Value)
	}
}

// An error raised inside a callback must surface, naming the builtin that was
// running, rather than being swallowed into a zero value.
func TestErrorsInsideCallbacksSurface(t *testing.T) {
	reg := registry.New(registry.Config{})
	// sqrt errors on negative input, so the callback fails partway through.
	src := `fn f(a: any) -> any => sum_by(a, (x) => sqrt(x))`
	if err := reg.Register("f", src); err != nil {
		t.Fatal(err)
	}

	_, err := reg.Execute(context.Background(), "f", map[string]any{"a": []any{-1.0}})
	if err == nil {
		t.Fatal("expected the callback's error to surface")
	}
}

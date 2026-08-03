package registry_test

import (
	"context"
	"testing"

	"github.com/xraph/dtl/registry"
)

func TestPhase1ThroughTheLanguage(t *testing.T) {
	cases := []struct {
		name, src string
		args      map[string]any
		want      any
	}{
		{"coalesce", `fn f(a: any) -> any => coalesce(a, "fallback")`, map[string]any{"a": nil}, "fallback"},
		{"default on blank", `fn f(a: any) -> any => default(a, "fb")`, map[string]any{"a": ""}, "fb"},
		{"path get", `fn f(o: object) -> any => path::get(o, "user.name")`,
			map[string]any{"o": map[string]any{"user": map[string]any{"name": "ada"}}}, "ada"},
		{"path get default", `fn f(o: object) -> any => path::get(o, "a.b.c", "none")`,
			map[string]any{"o": map[string]any{}}, "none"},
		{"path set then get", `fn f(o: object) -> any => path::get(path::set(o, "a.b", 7), "a.b")`,
			map[string]any{"o": map[string]any{}}, int64(7)},
		{"deep_merge", `fn f(a: object, b: object) -> any => path::get(deep_merge(a, b), "x.keep")`,
			map[string]any{
				"a": map[string]any{"x": map[string]any{"keep": "yes"}},
				"b": map[string]any{"x": map[string]any{"other": 1}},
			}, "yes"},
		{"invert", `fn f(o: object) -> any => path::get(invert(o), "1")`,
			map[string]any{"o": map[string]any{"a": "1"}}, "a"},
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
			if res.Value != c.want {
				t.Errorf("got %#v (%T), want %#v (%T)", res.Value, res.Value, c.want, c.want)
			}
		})
	}
}

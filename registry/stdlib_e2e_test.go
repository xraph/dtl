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

		{"snake_case", `fn f(s: string) -> string => snake_case(s)`,
			map[string]any{"s": "helloWorld"}, "hello_world"},
		{"index_of composes with substr", `fn f(s: string) -> string => substr(s, index_of(s, "world"))`,
			map[string]any{"s": "café world"}, "world"},
		{"mask", `fn f(s: string) -> string => mask(s)`,
			map[string]any{"s": "4111111111111111"}, "************1111"},

		{"regex test", `fn f(s: string) -> bool => regex::test(s, "^a\\d+$")`,
			map[string]any{"s": "a42"}, true},
		{"regex replace", `fn f(s: string) -> string => regex::replace(s, "(\\w+) (\\w+)", "$2 $1")`,
			map[string]any{"s": "hello world"}, "world hello"},
		{"regex groups", `fn f(s: string) -> any => path::get(regex::groups(s, "(?P<n>\\d+)"), "n")`,
			map[string]any{"s": "id-77"}, "77"},

		{"json parse keeps ints", `fn f(s: string) -> any => path::get(json::parse(s), "id")`,
			map[string]any{"s": `{"id": 12345}`}, int64(12345)},
		{"json round trip", `fn f(s: string) -> string => json::stringify(json::parse(s))`,
			map[string]any{"s": `{"a":1}`}, `{"a":1}`},
		{"json is_valid", `fn f(s: string) -> bool => json::is_valid(s)`,
			map[string]any{"s": "{oops"}, false},

		{"base64 round trip", `fn f(s: string) -> string => encoding::base64_decode(encoding::base64_encode(s))`,
			map[string]any{"s": "café"}, "café"},
		{"url encode", `fn f(s: string) -> string => encoding::url_encode(s)`,
			map[string]any{"s": "a b"}, "a+b"},

		{"sha256", `fn f(s: string) -> string => hash::sha256(s)`,
			map[string]any{"s": "abc"},
			"ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
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

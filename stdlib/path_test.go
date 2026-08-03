package stdlib

import (
	"reflect"
	"testing"
)

func nested() map[string]any {
	return map[string]any{
		"user": map[string]any{
			"name":  "ada",
			"email": nil,
			"tags":  []any{"a", "b"},
		},
		"items": []any{
			map[string]any{"name": "first"},
			map[string]any{"name": "second"},
		},
		"weird.key": 1,
	}
}

func TestPathGet(t *testing.T) {
	tests := []struct {
		name string
		args []any
		want any
	}{
		{"top level", []any{nested(), "user"}, nested()["user"]},
		{"nested", []any{nested(), "user.name"}, "ada"},
		{"array index", []any{nested(), "items.0.name"}, "first"},
		{"array of scalars", []any{nested(), "user.tags.1"}, "b"},
		{"missing yields null", []any{nested(), "user.missing"}, nil},
		{"missing yields default", []any{nested(), "user.missing", "fallback"}, "fallback"},
		{"stored null is not missing", []any{nested(), "user.email", "fallback"}, nil},
		{"index past end", []any{nested(), "items.9.name", "fallback"}, "fallback"},
		{"negative index", []any{nested(), "items.-1", "fallback"}, "fallback"},
		{"descend through scalar", []any{nested(), "user.name.nope", "fallback"}, "fallback"},
		{"empty path returns root", []any{"scalar", ""}, "scalar"},
		{"dotted key unreachable", []any{nested(), "weird.key", "fallback"}, "fallback"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fnPathGet(tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

// A stored null and an absent key are different questions, and path::get's
// default answers only the second. path::has is how the first is asked.
func TestPathHasDistinguishesNullFromAbsent(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"user.name", true},
		{"user.email", true}, // present, stored as null
		{"user.missing", false},
		{"items.0.name", true},
		{"items.5", false},
		{"weird.key", false}, // dotted key: documented as unreachable
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got, err := fnPathHas([]any{nested(), tt.path})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("path::has(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestPathSet(t *testing.T) {
	t.Run("existing key", func(t *testing.T) {
		got, _ := fnPathSet([]any{nested(), "user.name", "grace"})
		if v, _ := fnPathGet([]any{got, "user.name"}); v != "grace" {
			t.Errorf("got %#v, want grace", v)
		}
	})

	t.Run("creates intermediate objects", func(t *testing.T) {
		got, _ := fnPathSet([]any{map[string]any{}, "a.b.c", 1})
		if v, _ := fnPathGet([]any{got, "a.b.c"}); v != 1 {
			t.Errorf("got %#v, want 1", v)
		}
	})

	t.Run("replaces a scalar standing where an object is needed", func(t *testing.T) {
		got, _ := fnPathSet([]any{map[string]any{"a": 5}, "a.b", 1})
		if v, _ := fnPathGet([]any{got, "a.b"}); v != 1 {
			t.Errorf("got %#v, want 1", v)
		}
	})

	t.Run("writes into an array element", func(t *testing.T) {
		got, _ := fnPathSet([]any{nested(), "items.1.name", "renamed"})
		if v, _ := fnPathGet([]any{got, "items.1.name"}); v != "renamed" {
			t.Errorf("got %#v, want renamed", v)
		}
	})

	// The input must survive untouched: DTL values are shared freely between
	// expressions, so a mutating set would corrupt a caller's other references.
	t.Run("does not mutate its input", func(t *testing.T) {
		original := nested()
		if _, err := fnPathSet([]any{original, "user.name", "grace"}); err != nil {
			t.Fatal(err)
		}
		if v, _ := fnPathGet([]any{original, "user.name"}); v != "ada" {
			t.Errorf("input was mutated: user.name is now %#v", v)
		}

		if _, err := fnPathSet([]any{original, "items.0.name", "changed"}); err != nil {
			t.Fatal(err)
		}
		if v, _ := fnPathGet([]any{original, "items.0.name"}); v != "first" {
			t.Errorf("input array was mutated: items.0.name is now %#v", v)
		}
	})
}

func TestPathDelete(t *testing.T) {
	t.Run("removes a nested key", func(t *testing.T) {
		got, _ := fnPathDelete([]any{nested(), "user.name"})
		if has, _ := fnPathHas([]any{got, "user.name"}); has != false {
			t.Error("key survived deletion")
		}
		if has, _ := fnPathHas([]any{got, "user.tags"}); has != true {
			t.Error("sibling key was removed too")
		}
	})

	t.Run("removing an array element shifts the rest", func(t *testing.T) {
		got, _ := fnPathDelete([]any{nested(), "items.0"})
		if v, _ := fnPathGet([]any{got, "items.0.name"}); v != "second" {
			t.Errorf("got %#v, want second", v)
		}
	})

	t.Run("missing path is a no-op", func(t *testing.T) {
		got, _ := fnPathDelete([]any{nested(), "user.nope"})
		if !reflect.DeepEqual(got, nested()) {
			t.Errorf("got %#v, want the input unchanged", got)
		}
	})

	t.Run("does not mutate its input", func(t *testing.T) {
		original := nested()
		if _, err := fnPathDelete([]any{original, "user.name"}); err != nil {
			t.Fatal(err)
		}
		if has, _ := fnPathHas([]any{original, "user.name"}); has != true {
			t.Error("input was mutated")
		}
	})
}

func TestPathFlatten(t *testing.T) {
	got, err := fnPathFlatten([]any{map[string]any{
		"a": map[string]any{
			"b": map[string]any{"c": 1},
			"d": 2,
		},
		"e":     3,
		"empty": map[string]any{},
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := map[string]any{
		"a.b.c": 1,
		"a.d":   2,
		"e":     3,
		"empty": map[string]any{}, // an empty object is a leaf, not a branch
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

// Every path written by flatten must be readable by get, or the two functions
// describe different structures.
func TestPathFlattenRoundTripsThroughGet(t *testing.T) {
	source := map[string]any{
		"user": map[string]any{"name": "ada", "email": nil},
		"n":    1,
	}
	flatRaw, err := fnPathFlatten([]any{source})
	if err != nil {
		t.Fatal(err)
	}
	flat, _ := flatRaw.(map[string]any)

	for path, want := range flat {
		got, err := fnPathGet([]any{source, path})
		if err != nil {
			t.Fatalf("get(%q): %v", path, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("get(%q) = %#v, but flatten recorded %#v", path, got, want)
		}
	}
}

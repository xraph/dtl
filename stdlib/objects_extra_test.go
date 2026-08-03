package stdlib

import (
	"reflect"
	"testing"
)

// coalesce and default divide the null-handling job between them, and the
// division is the whole point: coalesce tests nullness, default tests
// blankness. These cases are where the two disagree.
func TestCoalesceAndDefaultDivideTheWork(t *testing.T) {
	tests := []struct {
		name             string
		value            any
		wantFromCoalesce any
		wantFromDefault  any
	}{
		{"null", nil, "fb", "fb"},
		{"empty string", "", "", "fb"},
		{"whitespace string", "   ", "   ", "fb"},
		{"empty array", []any{}, []any{}, "fb"},
		{"empty object", map[string]any{}, map[string]any{}, "fb"},
		{"zero", int64(0), int64(0), int64(0)},
		{"false", false, false, false},
		{"real value", "x", "x", "x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fnCoalesce([]any{tt.value, "fb"})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.wantFromCoalesce) {
				t.Errorf("coalesce = %#v, want %#v", got, tt.wantFromCoalesce)
			}

			got, err = fnDefault([]any{tt.value, "fb"})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.wantFromDefault) {
				t.Errorf("default = %#v, want %#v", got, tt.wantFromDefault)
			}
		})
	}
}

func TestCoalesceIsVariadic(t *testing.T) {
	tests := []struct {
		name string
		args []any
		want any
	}{
		{"first wins", []any{"a", "b"}, "a"},
		{"skips leading nulls", []any{nil, nil, "c"}, "c"},
		{"all null", []any{nil, nil}, nil},
		{"single non-null", []any{"only"}, "only"},
		{"single null", []any{nil}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fnCoalesce(tt.args)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestDeepMerge(t *testing.T) {
	tests := []struct {
		name string
		args []any
		want any
	}{
		{
			"nested objects merge rather than replace",
			[]any{
				map[string]any{"a": map[string]any{"x": 1, "y": 2}},
				map[string]any{"a": map[string]any{"y": 3, "z": 4}},
			},
			map[string]any{"a": map[string]any{"x": 1, "y": 3, "z": 4}},
		},
		{
			"arrays are replaced, not concatenated",
			[]any{
				map[string]any{"a": []any{1, 2}},
				map[string]any{"a": []any{3}},
			},
			map[string]any{"a": []any{3}},
		},
		{
			"a scalar overwrites an object",
			[]any{
				map[string]any{"a": map[string]any{"x": 1}},
				map[string]any{"a": "scalar"},
			},
			map[string]any{"a": "scalar"},
		},
		{
			"an object overwrites a scalar",
			[]any{
				map[string]any{"a": "scalar"},
				map[string]any{"a": map[string]any{"x": 1}},
			},
			map[string]any{"a": map[string]any{"x": 1}},
		},
		{
			"three layers, later wins",
			[]any{
				map[string]any{"a": 1},
				map[string]any{"a": 2, "b": 2},
				map[string]any{"b": 3},
			},
			map[string]any{"a": 2, "b": 3},
		},
		{
			"a null value overwrites",
			[]any{
				map[string]any{"a": 1},
				map[string]any{"a": nil},
			},
			map[string]any{"a": nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fnDeepMerge(tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestDeepMergeDoesNotMutateItsInputs(t *testing.T) {
	a := map[string]any{"nested": map[string]any{"keep": 1}}
	b := map[string]any{"nested": map[string]any{"add": 2}}

	if _, err := fnDeepMerge([]any{a, b}); err != nil {
		t.Fatal(err)
	}

	inner, _ := a["nested"].(map[string]any)
	if len(inner) != 1 {
		t.Errorf("first input was mutated: nested is now %#v", inner)
	}
}

func TestDeepMergeRejectsNonObjects(t *testing.T) {
	if _, err := fnDeepMerge([]any{map[string]any{}, "not an object"}); err == nil {
		t.Error("expected an error when an argument is not an object")
	}
}

func TestFromEntriesInvertsEntries(t *testing.T) {
	source := map[string]any{"a": 1, "b": "two", "c": nil}

	entries, err := fnEntries([]any{source})
	if err != nil {
		t.Fatal(err)
	}
	got, err := fnFromEntries([]any{entries})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, source) {
		t.Errorf("round trip produced %#v, want %#v", got, source)
	}
}

func TestFromEntriesSkipsMalformedEntries(t *testing.T) {
	got, err := fnFromEntries([]any{[]any{
		map[string]any{"key": "a", "value": 1},
		map[string]any{"value": 2}, // no key
		"not an object",
		map[string]any{"key": "b"}, // no value
	}})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"a": 1, "b": nil}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestInvert(t *testing.T) {
	got, err := fnInvert([]any{map[string]any{"a": "1", "b": "2"}})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"1": "a", "2": "b"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

// When two keys share a value one of them must win, and which one has to be
// the same on every run rather than a function of Go's map iteration order.
func TestInvertResolvesCollisionsDeterministically(t *testing.T) {
	source := map[string]any{"a": "same", "b": "same", "c": "same"}

	first, err := fnInvert([]any{source})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		again, err := fnInvert([]any{source})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(first, again) {
			t.Fatalf("invert is not deterministic: got %#v then %#v", first, again)
		}
	}
}

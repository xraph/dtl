package stdlib

import (
	"reflect"
	"strings"
	"testing"
)

// The reason json::parse uses UseNumber: plain encoding/json makes every number
// a float64, so an id of 12345 would come back as a float and show up as one
// through type_of and through arithmetic. DTL has distinct int and float, and
// parsing must not collapse them.
func TestJSONParsePreservesIntegers(t *testing.T) {
	tests := []struct {
		name, input string
		want        any
	}{
		{"small int", `1`, int64(1)},
		{"negative int", `-42`, int64(-42)},
		{"zero", `0`, int64(0)},
		{"large int", `9007199254740993`, int64(9007199254740993)},
		{"max int64", `9223372036854775807`, int64(9223372036854775807)},
		{"float", `1.5`, 1.5},
		{"float with trailing zero", `1.0`, 1.0},
		{"exponent", `1e3`, 1000.0},
		// Beyond int64, so it has to widen to float rather than overflow.
		{"beyond int64", `99999999999999999999`, 1e20},
		{"string", `"x"`, "x"},
		{"true", `true`, true},
		{"false", `false`, false},
		{"null", `null`, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fnJSONParse([]any{tt.input})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %#v (%T), want %#v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

func TestJSONParseNestedIntegersStayIntegers(t *testing.T) {
	got, err := fnJSONParse([]any{`{"a": [1, 2.5, {"b": 3}]}`})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"a": []any{int64(1), 2.5, map[string]any{"b": int64(3)}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestJSONParseRejectsBadInput(t *testing.T) {
	tests := []struct{ name, input string }{
		{"truncated object", `{"a":`},
		{"not json", `hello`},
		{"single quotes", `{'a': 1}`},
		{"empty", ``},
		// One call parses one document; trailing content is a mistake worth
		// reporting rather than silently ignoring.
		{"trailing content", `{"a":1} {"b":2}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := fnJSONParse([]any{tt.input}); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

// The executor bounds Timeout and MaxDepth; neither constrains a parser, so the
// depth cap is what stops a deeply nested payload from being unbounded work.
func TestJSONParseEnforcesDepthLimit(t *testing.T) {
	within := strings.Repeat("[", 30) + strings.Repeat("]", 30)
	if _, err := fnJSONParse([]any{within}); err != nil {
		t.Errorf("30 levels should parse: %v", err)
	}

	beyond := strings.Repeat("[", maxJSONDepth+10) + strings.Repeat("]", maxJSONDepth+10)
	_, err := fnJSONParse([]any{beyond})
	if err == nil {
		t.Fatal("expected an error beyond the depth limit")
	}
	if !strings.Contains(err.Error(), "nesting") {
		t.Errorf("error %q should explain that nesting was the problem", err)
	}
}

func TestJSONStringify(t *testing.T) {
	tests := []struct {
		name string
		args []any
		want string
	}{
		{"object", []any{map[string]any{"a": int64(1)}}, `{"a":1}`},
		{"array", []any{[]any{int64(1), "x"}}, `[1,"x"]`},
		{"string", []any{"x"}, `"x"`},
		{"null", []any{nil}, `null`},
		{"int stays int", []any{int64(42)}, `42`},
		{"float", []any{1.5}, `1.5`},
		{"indent", []any{map[string]any{"a": int64(1)}, int64(2)}, "{\n  \"a\": 1\n}"},
		{"indent zero is compact", []any{map[string]any{"a": int64(1)}, int64(0)}, `{"a":1}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fnJSONStringify(tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// Go escapes <, > and & by default, which corrupts any string carrying markup
// for no benefit here.
func TestJSONStringifyDoesNotEscapeMarkup(t *testing.T) {
	got, err := fnJSONStringify([]any{"<b>&</b>"})
	if err != nil {
		t.Fatal(err)
	}
	if got != `"<b>&</b>"` {
		t.Errorf("got %q, want %q", got, `"<b>&</b>"`)
	}
}

// A value that survives a round trip unchanged is the property both functions
// are really promising.
func TestJSONRoundTrip(t *testing.T) {
	values := []any{
		map[string]any{"i": int64(1), "f": 1.5, "s": "x", "b": true, "n": nil},
		[]any{int64(1), int64(2), int64(3)},
		map[string]any{"nested": map[string]any{"deep": []any{int64(1)}}},
		"plain string",
		int64(0),
		map[string]any{},
		[]any{},
	}

	for _, v := range values {
		encoded, err := fnJSONStringify([]any{v})
		if err != nil {
			t.Fatalf("stringify(%#v): %v", v, err)
		}
		decoded, err := fnJSONParse([]any{encoded})
		if err != nil {
			t.Fatalf("parse(%q): %v", encoded, err)
		}
		if !reflect.DeepEqual(decoded, v) {
			t.Errorf("round trip changed %#v into %#v (via %s)", v, decoded, encoded)
		}
	}
}

func TestJSONIsValid(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{`{"a":1}`, true},
		{`[1,2]`, true},
		{`"x"`, true},
		{`null`, true},
		{`{`, false},
		{``, false},
		{`nope`, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := fnJSONIsValid([]any{tt.input})
			if err != nil {
				t.Fatalf("is_valid should not error, got %v", err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

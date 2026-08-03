package stdlib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/xraph/dtl/executor"
)

// JSON parsing and serialisation, for the common case of a host handing DTL a
// text column that happens to contain JSON.
func registerJSON(m map[string]*executor.BuiltinFunc) {
	register(m, "json::parse", 1, 1, fnJSONParse,
		"json::parse(s) -> any -- Parses a JSON string. Whole numbers stay ints. Errors on invalid input or excessive nesting")
	register(m, "json::stringify", 1, 2, fnJSONStringify,
		"json::stringify(value, indent?) -> string -- Serialises to JSON. indent is a space count; omitted or 0 is compact")
	register(m, "json::is_valid", 1, 1, fnJSONIsValid,
		"json::is_valid(s) -> bool -- Whether the string parses as JSON within the nesting limit")
}

// maxJSONDepth bounds how deeply nested a parsed document may be.
//
// The executor limits Timeout and MaxDepth, and neither constrains a parser, so
// without this a deeply nested untrusted payload is unbounded work on an
// unbounded stack. 64 is far past any structure a transformation realistically
// walks, while still stopping a document nested thousands deep.
const maxJSONDepth = 64

func fnJSONParse(args []any) (any, error) {
	s := executor.ToString(args[0])

	dec := json.NewDecoder(strings.NewReader(s))
	// UseNumber is what preserves integers. Without it encoding/json produces
	// float64 for every number, so an id of 12345 would come back as a float
	// and be visible as one through type_of and through arithmetic.
	dec.UseNumber()

	var raw any
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("json::parse: %w", err)
	}
	// A second value after the first means the input was not one document.
	if dec.More() {
		return nil, fmt.Errorf("json::parse: unexpected trailing content after the top-level value")
	}

	return convertJSON(raw, 0)
}

// convertJSON rewrites the decoder's output into DTL's own value model:
// json.Number becomes int64 or float64, and maps and slices are rebuilt as the
// any-typed containers the executor expects.
func convertJSON(v any, depth int) (any, error) {
	if depth > maxJSONDepth {
		return nil, fmt.Errorf("json::parse: nesting deeper than %d levels", maxJSONDepth)
	}

	switch val := v.(type) {
	case json.Number:
		// An integer literal stays an integer. Anything with a fraction or an
		// exponent, or too large for an int64, becomes a float.
		if i, err := val.Int64(); err == nil {
			return i, nil
		}
		f, err := val.Float64()
		if err != nil {
			return nil, fmt.Errorf("json::parse: %q is not a representable number", val.String())
		}
		return f, nil

	case map[string]any:
		out := make(map[string]any, len(val))
		for k, item := range val {
			converted, err := convertJSON(item, depth+1)
			if err != nil {
				return nil, err
			}
			out[k] = converted
		}
		return out, nil

	case []any:
		out := make([]any, len(val))
		for i, item := range val {
			converted, err := convertJSON(item, depth+1)
			if err != nil {
				return nil, err
			}
			out[i] = converted
		}
		return out, nil

	default:
		// Strings, bools and null need no rewriting.
		return val, nil
	}
}

func fnJSONStringify(args []any) (any, error) {
	indent := 0
	if len(args) > 1 {
		indent = int(executor.ToInt(args[1]))
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	// Without this, encoding/json escapes <, > and & into < and friends,
	// which corrupts any string carrying markup for no benefit here.
	enc.SetEscapeHTML(false)
	if indent > 0 {
		enc.SetIndent("", strings.Repeat(" ", indent))
	}

	if err := enc.Encode(args[0]); err != nil {
		return nil, fmt.Errorf("json::stringify: %w", err)
	}
	// Encode always appends a newline; callers want the document alone.
	return strings.TrimSuffix(buf.String(), "\n"), nil
}

func fnJSONIsValid(args []any) (any, error) {
	if _, err := fnJSONParse(args); err != nil {
		return false, nil
	}
	return true, nil
}

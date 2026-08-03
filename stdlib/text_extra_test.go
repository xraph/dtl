package stdlib

import (
	"reflect"
	"testing"
)

func TestCaseConversions(t *testing.T) {
	tests := []struct {
		input                       string
		snake, kebab, camel, pascal string
	}{
		{"hello world", "hello_world", "hello-world", "helloWorld", "HelloWorld"},
		{"helloWorld", "hello_world", "hello-world", "helloWorld", "HelloWorld"},
		{"HelloWorld", "hello_world", "hello-world", "helloWorld", "HelloWorld"},
		{"hello_world", "hello_world", "hello-world", "helloWorld", "HelloWorld"},
		{"hello-world", "hello_world", "hello-world", "helloWorld", "HelloWorld"},
		{"  hello   world  ", "hello_world", "hello-world", "helloWorld", "HelloWorld"},
		{"hello.world", "hello_world", "hello-world", "helloWorld", "HelloWorld"},
		{"single", "single", "single", "single", "Single"},
		{"", "", "", "", ""},
		{"user2Name", "user2_name", "user2-name", "user2Name", "User2Name"},
		// An acronym stays one word rather than exploding into single letters.
		{"parseHTTPResponse", "parse_http_response", "parse-http-response", "parseHttpResponse", "ParseHttpResponse"},
		{"HTTPServer", "http_server", "http-server", "httpServer", "HttpServer"},
		{"ID", "id", "id", "id", "Id"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			check := func(label string, fn func([]any) (any, error), want string) {
				t.Helper()
				got, err := fn([]any{tt.input})
				if err != nil {
					t.Fatalf("%s: %v", label, err)
				}
				if got != want {
					t.Errorf("%s(%q) = %q, want %q", label, tt.input, got, want)
				}
			}
			check("snake_case", fnSnakeCase, tt.snake)
			check("kebab_case", fnKebabCase, tt.kebab)
			check("camel_case", fnCamelCase, tt.camel)
			check("pascal_case", fnPascalCase, tt.pascal)
		})
	}
}

// Round-tripping has to be stable, which is only true if every style segments
// the input the same way.
func TestCaseConversionsRoundTrip(t *testing.T) {
	for _, start := range []string{"hello_world", "user_id", "parse_http_response"} {
		t.Run(start, func(t *testing.T) {
			camel, _ := fnCamelCase([]any{start})
			back, _ := fnSnakeCase([]any{camel})
			if back != start {
				t.Errorf("%q -> camel %q -> snake %q, want %q", start, camel, back, start)
			}
		})
	}
}

// The reason every position here is a character offset: substr is rune-based,
// so an index_of that returned bytes would silently slice mid-character.
func TestIndexOfComposesWithSubstr(t *testing.T) {
	tests := []struct {
		name, s, needle, want string
	}{
		{"ascii", "hello world", "world", "world"},
		{"after multibyte", "café world", "world", "world"},
		{"multibyte needle", "café au lait", "é", "é au lait"},
		{"emoji", "a🎉b needle", "needle", "needle"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx, err := fnIndexOf([]any{tt.s, tt.needle})
			if err != nil {
				t.Fatal(err)
			}
			got, err := fnSubstr([]any{tt.s, idx})
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("substr(s, index_of(s, %q)) = %q, want %q", tt.needle, got, tt.want)
			}
		})
	}
}

func TestIndexOfAndLastIndexOf(t *testing.T) {
	tests := []struct {
		name, s, needle string
		first, last     int64
	}{
		{"ascii repeated", "abcabc", "b", 1, 4},
		{"absent", "abc", "z", -1, -1},
		{"whole string", "abc", "abc", 0, 0},
		{"after multibyte", "ééx", "x", 2, 2},
		{"multibyte repeated", "éaé", "é", 0, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := fnIndexOf([]any{tt.s, tt.needle})
			if got != tt.first {
				t.Errorf("index_of = %v, want %v", got, tt.first)
			}
			got, _ = fnLastIndexOf([]any{tt.s, tt.needle})
			if got != tt.last {
				t.Errorf("last_index_of = %v, want %v", got, tt.last)
			}
		})
	}
}

func TestLeftRightCharAtAreRuneBased(t *testing.T) {
	const s = "café"

	tests := []struct {
		name string
		got  func() any
		want any
	}{
		{"left within", func() any { v, _ := fnLeft([]any{s, 3}); return v }, "caf"},
		{"left whole", func() any { v, _ := fnLeft([]any{s, 4}); return v }, "café"},
		{"left past end", func() any { v, _ := fnLeft([]any{s, 99}); return v }, "café"},
		{"left zero", func() any { v, _ := fnLeft([]any{s, 0}); return v }, ""},
		{"left negative", func() any { v, _ := fnLeft([]any{s, -1}); return v }, ""},
		// If right were byte-based it would slice into the é and produce a
		// replacement character instead of the letter.
		{"right within", func() any { v, _ := fnRight([]any{s, 2}); return v }, "fé"},
		{"right whole", func() any { v, _ := fnRight([]any{s, 4}); return v }, "café"},
		{"right past end", func() any { v, _ := fnRight([]any{s, 99}); return v }, "café"},
		{"char_at last", func() any { v, _ := fnCharAt([]any{s, 3}); return v }, "é"},
		{"char_at first", func() any { v, _ := fnCharAt([]any{s, 0}); return v }, "c"},
		{"char_at out of range", func() any { v, _ := fnCharAt([]any{s, 9}); return v }, ""},
		{"char_at negative", func() any { v, _ := fnCharAt([]any{s, -1}); return v }, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.got(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTrimVariants(t *testing.T) {
	tests := []struct {
		name string
		got  func() any
		want any
	}{
		{"trim_start", func() any { v, _ := fnTrimStart([]any{"  x  "}); return v }, "x  "},
		{"trim_end", func() any { v, _ := fnTrimEnd([]any{"  x  "}); return v }, "  x"},
		{"trim_start tabs and newlines", func() any { v, _ := fnTrimStart([]any{"\t\n x"}); return v }, "x"},
		{"trim_chars", func() any { v, _ := fnTrimChars([]any{"xxhelloxx", "x"}); return v }, "hello"},
		{"trim_chars set", func() any { v, _ := fnTrimChars([]any{"-_hello_-", "-_"}); return v }, "hello"},
		{"trim_chars no match", func() any { v, _ := fnTrimChars([]any{"hello", "z"}); return v }, "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.got(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLines(t *testing.T) {
	tests := []struct {
		name, input string
		want        []any
	}{
		{"lf", "a\nb\nc", []any{"a", "b", "c"}},
		{"crlf", "a\r\nb", []any{"a", "b"}},
		{"mixed", "a\r\nb\nc", []any{"a", "b", "c"}},
		{"single line", "a", []any{"a"}},
		{"empty", "", []any{}},
		{"trailing newline yields a final empty line", "a\n", []any{"a", ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fnLines([]any{tt.input})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestMask(t *testing.T) {
	tests := []struct {
		name string
		args []any
		want string
	}{
		{"default keeps last four", []any{"4111111111111111"}, "************1111"},
		{"custom keep", []any{"secret", 2}, "****et"},
		{"custom mask char", []any{"secret", 2, "#"}, "####et"},
		{"keep zero", []any{"abc", 0}, "***"},
		// Short values are masked completely: returning them intact would
		// defeat the point of the call.
		{"shorter than window", []any{"abc"}, "***"},
		{"exactly the window", []any{"abcd"}, "****"},
		{"negative keep treated as zero", []any{"abc", -5}, "***"},
		{"empty string", []any{""}, ""},
		{"multibyte preserved", []any{"café1234"}, "****1234"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fnMask(tt.args)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMiscTextHelpers(t *testing.T) {
	tests := []struct {
		name string
		got  func() any
		want any
	}{
		{"normalize_space collapses runs", func() any { v, _ := fnNormalizeSpace([]any{"  a   b \n c  "}); return v }, "a b c"},
		{"normalize_space of blanks", func() any { v, _ := fnNormalizeSpace([]any{"   "}); return v }, ""},
		{"strip_prefix present", func() any { v, _ := fnStripPrefix([]any{"prefix-body", "prefix-"}); return v }, "body"},
		{"strip_prefix absent", func() any { v, _ := fnStripPrefix([]any{"body", "prefix-"}); return v }, "body"},
		{"strip_suffix present", func() any { v, _ := fnStripSuffix([]any{"name.txt", ".txt"}); return v }, "name"},
		{"strip_suffix absent", func() any { v, _ := fnStripSuffix([]any{"name", ".txt"}); return v }, "name"},
		{"count_occurrences", func() any { v, _ := fnCountOccurrences([]any{"abcabc", "bc"}); return v }, int64(2)},
		{"count_occurrences absent", func() any { v, _ := fnCountOccurrences([]any{"abc", "z"}); return v }, int64(0)},
		// strings.Count counts boundaries for an empty needle; zero is the
		// less surprising answer.
		{"count_occurrences empty needle", func() any { v, _ := fnCountOccurrences([]any{"abc", ""}); return v }, int64(0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.got(); got != tt.want {
				t.Errorf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

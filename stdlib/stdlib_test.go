package stdlib

import (
	"math"
	"testing"
	"time"

	"github.com/xraph/dtl/executor"
)

// helper to call a builtin by name
func callBuiltin(t testing.TB, builtins map[string]*executor.BuiltinFunc, name string, args ...any) any {
	t.Helper()
	fn, ok := builtins[name]
	if !ok {
		t.Fatalf("builtin %q not found", name)
	}
	result, err := fn.Fn(args)
	if err != nil {
		t.Fatalf("%s() error: %v", name, err)
	}
	return result
}

func callBuiltinError(t testing.TB, builtins map[string]*executor.BuiltinFunc, name string, args ...any) error {
	t.Helper()
	fn, ok := builtins[name]
	if !ok {
		t.Fatalf("builtin %q not found", name)
	}
	_, err := fn.Fn(args)
	return err
}

func setup() map[string]*executor.BuiltinFunc {
	m := make(map[string]*executor.BuiltinFunc)
	RegisterAll(m)
	return m
}

// --- RegisterAll ---

func TestRegisterAll_PopulatesBuiltins(t *testing.T) {
	m := setup()
	// Spot-check that key functions from each module exist
	expected := []string{
		"len", "type_of", "abs", "sqrt", // core
		"upper", "lower", "trim", "split", "join", // text
		"now", "today", // datetime (registered in core)
		"z_score", "correlation", // stats
		"as_float", "as_int", "as_string", "as_bool", // casting
	}
	for _, name := range expected {
		if _, ok := m[name]; !ok {
			t.Errorf("expected builtin %q to be registered", name)
		}
	}
}

// --- Core ---

func TestLen(t *testing.T) {
	m := setup()
	tests := []struct {
		input any
		want  int64
	}{
		{"hello", 5},
		{"", 0},
		{[]any{1, 2, 3}, 3},
		{[]any{}, 0},
		{map[string]any{"a": 1}, 1},
		{nil, 0},
	}
	for _, tc := range tests {
		got := callBuiltin(t, m, "len", tc.input)
		if got != tc.want {
			t.Errorf("len(%v): got %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestTypeOf(t *testing.T) {
	m := setup()
	tests := []struct {
		input any
		want  string
	}{
		{nil, "null"},
		{true, "bool"},
		{int64(42), "int"},
		{3.14, "float"},
		{"hello", "string"},
		{[]any{}, "array"},
		{map[string]any{}, "object"},
		{time.Now(), "datetime"},
	}
	for _, tc := range tests {
		got := callBuiltin(t, m, "type_of", tc.input)
		if got != tc.want {
			t.Errorf("type_of(%v): got %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestIsNull(t *testing.T) {
	m := setup()
	if callBuiltin(t, m, "is_null", nil) != true {
		t.Error("is_null(nil) should be true")
	}
	if callBuiltin(t, m, "is_null", int64(1)) != false {
		t.Error("is_null(1) should be false")
	}
}

func TestIsBlank(t *testing.T) {
	m := setup()
	if callBuiltin(t, m, "is_blank", nil) != true {
		t.Error("is_blank(nil) should be true")
	}
	if callBuiltin(t, m, "is_blank", "") != true {
		t.Error(`is_blank("") should be true`)
	}
	if callBuiltin(t, m, "is_blank", "   ") != true {
		t.Error(`is_blank("   ") should be true`)
	}
	if callBuiltin(t, m, "is_blank", []any{}) != true {
		t.Error("is_blank([]) should be true")
	}
	if callBuiltin(t, m, "is_blank", map[string]any{}) != true {
		t.Error("is_blank({}) should be true")
	}
	if callBuiltin(t, m, "is_blank", "x") != false {
		t.Error(`is_blank("x") should be false`)
	}
	if callBuiltin(t, m, "is_blank", int64(0)) != false {
		t.Error("is_blank(0) should be false")
	}
	if callBuiltin(t, m, "is_blank", []any{1}) != false {
		t.Error("is_blank([1]) should be false")
	}
}

func TestAbs(t *testing.T) {
	m := setup()
	got := callBuiltin(t, m, "abs", -5.0)
	if got != 5.0 {
		t.Errorf("abs(-5.0): got %v", got)
	}
}

func TestRound(t *testing.T) {
	m := setup()
	// Round with no decimals
	got := callBuiltin(t, m, "round", 3.7)
	if got != 4.0 {
		t.Errorf("round(3.7): got %v", got)
	}
	// Round with 2 decimals
	got = callBuiltin(t, m, "round", 3.456, int64(2))
	if got != 3.46 {
		t.Errorf("round(3.456, 2): got %v", got)
	}
}

func TestCeilFloor(t *testing.T) {
	m := setup()
	if callBuiltin(t, m, "ceil", 3.2) != 4.0 {
		t.Error("ceil(3.2) should be 4.0")
	}
	if callBuiltin(t, m, "floor", 3.7) != 3.0 {
		t.Error("floor(3.7) should be 3.0")
	}
}

func TestPower(t *testing.T) {
	m := setup()
	got := callBuiltin(t, m, "power", 2.0, 3.0)
	if got != 8.0 {
		t.Errorf("power(2,3): got %v", got)
	}
}

func TestSqrt(t *testing.T) {
	m := setup()
	got := callBuiltin(t, m, "sqrt", 16.0)
	if got != 4.0 {
		t.Errorf("sqrt(16): got %v", got)
	}
}

func TestSqrt_Negative(t *testing.T) {
	m := setup()
	err := callBuiltinError(t, m, "sqrt", -1.0)
	if err == nil {
		t.Error("sqrt(-1) should return error")
	}
}

func TestLog(t *testing.T) {
	m := setup()
	got := callBuiltin(t, m, "log", math.E).(float64)
	if math.Abs(got-1.0) > 0.0001 {
		t.Errorf("log(e): got %v, want ~1.0", got)
	}
}

func TestLog_NonPositive(t *testing.T) {
	m := setup()
	err := callBuiltinError(t, m, "log", 0.0)
	if err == nil {
		t.Error("log(0) should error")
	}
}

func TestLog10(t *testing.T) {
	m := setup()
	got := callBuiltin(t, m, "log10", 100.0).(float64)
	if math.Abs(got-2.0) > 0.0001 {
		t.Errorf("log10(100): got %v, want 2.0", got)
	}
}

func TestNow(t *testing.T) {
	m := setup()
	got := callBuiltin(t, m, "now")
	if _, ok := got.(time.Time); !ok {
		t.Errorf("now(): expected time.Time, got %T", got)
	}
}

func TestToday(t *testing.T) {
	m := setup()
	got := callBuiltin(t, m, "today")
	dt, ok := got.(time.Time)
	if !ok {
		t.Fatalf("today(): expected time.Time, got %T", got)
	}
	// Should be midnight
	if dt.Hour() != 0 || dt.Minute() != 0 || dt.Second() != 0 {
		t.Error("today() should return midnight")
	}
}

func TestPrint(t *testing.T) {
	m := setup()
	got := callBuiltin(t, m, "PRINT", "hello")
	if got != "hello" {
		t.Errorf("PRINT should return input, got %v", got)
	}
}

func TestDebug(t *testing.T) {
	m := setup()
	got := callBuiltin(t, m, "DEBUG", "label", "value")
	if got != "value" {
		t.Errorf("DEBUG should return last arg, got %v", got)
	}
}

// --- Text ---

func TestUpper(t *testing.T) {
	m := setup()
	if callBuiltin(t, m, "upper", "hello") != "HELLO" {
		t.Error("upper failed")
	}
}

func TestLower(t *testing.T) {
	m := setup()
	if callBuiltin(t, m, "lower", "HELLO") != "hello" {
		t.Error("lower failed")
	}
}

func TestTrim(t *testing.T) {
	m := setup()
	if callBuiltin(t, m, "trim", "  hello  ") != "hello" {
		t.Error("trim failed")
	}
}

func TestReplace(t *testing.T) {
	m := setup()
	got := callBuiltin(t, m, "replace", "hello world", "world", "there")
	if got != "hello there" {
		t.Errorf("replace: got %q", got)
	}
}

func TestSplit(t *testing.T) {
	m := setup()
	got := callBuiltin(t, m, "split", "a,b,c", ",")
	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", got)
	}
	if len(arr) != 3 {
		t.Errorf("split: got %d parts, want 3", len(arr))
	}
}

func TestJoin(t *testing.T) {
	m := setup()
	got := callBuiltin(t, m, "join", []any{"a", "b", "c"}, "-")
	if got != "a-b-c" {
		t.Errorf("join: got %q", got)
	}
}

func TestStartsWith(t *testing.T) {
	m := setup()
	if callBuiltin(t, m, "starts_with", "hello", "hel") != true {
		t.Error("starts_with failed")
	}
	if callBuiltin(t, m, "starts_with", "hello", "xyz") != false {
		t.Error("starts_with false case failed")
	}
}

func TestContains(t *testing.T) {
	m := setup()
	if callBuiltin(t, m, "contains", "hello world", "world") != true {
		t.Error("contains failed")
	}
	if callBuiltin(t, m, "contains", "hello", "xyz") != false {
		t.Error("contains false case failed")
	}
}

func TestSubstr(t *testing.T) {
	m := setup()
	got := callBuiltin(t, m, "substr", "hello", int64(1), int64(3))
	if got != "ell" {
		t.Errorf("substr: got %q, want %q", got, "ell")
	}
}

func TestSlugify(t *testing.T) {
	m := setup()
	got := callBuiltin(t, m, "slugify", "Hello World!")
	if got != "hello-world" {
		t.Errorf("slugify: got %q", got)
	}
}

func TestTruncate(t *testing.T) {
	m := setup()
	got := callBuiltin(t, m, "truncate", "hello world", int64(8))
	if got != "hello..." {
		t.Errorf("truncate: got %q", got)
	}
}

func TestExtractNumber(t *testing.T) {
	m := setup()
	got := callBuiltin(t, m, "extract_number", "price is 42.5 dollars")
	if got != 42.5 {
		t.Errorf("extract_number: got %v", got)
	}
}

func TestMatchRegex(t *testing.T) {
	m := setup()
	got := callBuiltin(t, m, "match_regex", "foo123bar456", `\d+`)
	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", got)
	}
	if len(arr) != 2 {
		t.Errorf("match_regex: got %d matches, want 2", len(arr))
	}
}

// --- Stats ---

func TestZScore(t *testing.T) {
	m := setup()
	// z-score of a value relative to a dataset
	data := []any{1.0, 2.0, 3.0, 4.0, 5.0}
	got := callBuiltin(t, m, "z_score", 5.0, data)
	f, ok := got.(float64)
	if !ok {
		t.Fatalf("expected float64, got %T", got)
	}
	// 5 should have a positive z-score
	if f <= 0 {
		t.Errorf("z_score(5, [1..5]): expected positive, got %v", f)
	}
}

func TestCorrelation(t *testing.T) {
	m := setup()
	x := []any{1.0, 2.0, 3.0, 4.0, 5.0}
	y := []any{2.0, 4.0, 6.0, 8.0, 10.0}
	got := callBuiltin(t, m, "correlation", x, y).(float64)
	if math.Abs(got-1.0) > 0.0001 {
		t.Errorf("correlation of perfect positive: got %v, want 1.0", got)
	}
}

func TestOutlierBounds(t *testing.T) {
	m := setup()
	data := []any{1.0, 2.0, 3.0, 4.0, 5.0}
	got := callBuiltin(t, m, "outlier_bounds", data)
	bounds, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", got)
	}
	if _, ok := bounds["lower"]; !ok {
		t.Error("missing lower bound")
	}
	if _, ok := bounds["upper"]; !ok {
		t.Error("missing upper bound")
	}
}

// --- Casting ---

func TestAsFloat(t *testing.T) {
	m := setup()
	tests := []struct {
		input any
		want  float64
	}{
		{3.14, 3.14},
		{int64(42), 42.0},
		{"3.14", 3.14},
		{true, 1.0},
		{false, 0.0},
		{nil, 0.0},
	}
	for _, tc := range tests {
		got := callBuiltin(t, m, "as_float", tc.input)
		if got != tc.want {
			t.Errorf("as_float(%v): got %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestAsFloat_InvalidString(t *testing.T) {
	m := setup()
	err := callBuiltinError(t, m, "as_float", "not_a_number")
	if err == nil {
		t.Error("as_float('not_a_number') should error")
	}
}

func TestAsInt(t *testing.T) {
	m := setup()
	tests := []struct {
		input any
		want  int64
	}{
		{int64(42), 42},
		{3.7, 3},
		{"42", 42},
		{true, 1},
		{false, 0},
		{nil, 0},
	}
	for _, tc := range tests {
		got := callBuiltin(t, m, "as_int", tc.input)
		if got != tc.want {
			t.Errorf("as_int(%v): got %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestAsString(t *testing.T) {
	m := setup()
	got := callBuiltin(t, m, "as_string", int64(42))
	if got != "42" {
		t.Errorf("as_string(42): got %v", got)
	}
}

func TestAsBool(t *testing.T) {
	m := setup()
	if callBuiltin(t, m, "as_bool", int64(1)) != true {
		t.Error("as_bool(1) should be true")
	}
	if callBuiltin(t, m, "as_bool", int64(0)) != false {
		t.Error("as_bool(0) should be false")
	}
}

func TestAsDatetime(t *testing.T) {
	m := setup()
	got := callBuiltin(t, m, "as_datetime", "2024-01-15")
	dt, ok := got.(time.Time)
	if !ok {
		t.Fatalf("expected time.Time, got %T", got)
	}
	if dt.Year() != 2024 || dt.Month() != 1 || dt.Day() != 15 {
		t.Errorf("as_datetime: got %v", dt)
	}
}

func TestAsDatetime_RFC3339(t *testing.T) {
	m := setup()
	got := callBuiltin(t, m, "as_datetime", "2024-01-15T10:30:00Z")
	dt, ok := got.(time.Time)
	if !ok {
		t.Fatalf("expected time.Time, got %T", got)
	}
	if dt.Hour() != 10 || dt.Minute() != 30 {
		t.Errorf("as_datetime RFC3339: got %v", dt)
	}
}

func TestAsDatetime_InvalidString(t *testing.T) {
	m := setup()
	err := callBuiltinError(t, m, "as_datetime", "not-a-date")
	if err == nil {
		t.Error("as_datetime('not-a-date') should error")
	}
}

// --- Namespace aliases ---

func TestNamespaceAliases(t *testing.T) {
	m := setup()
	aliases := []string{
		"system::text::slugify",
		"system::text::truncate",
		"system::text::extract_number",
		"system::stats::z_score",
		"system::stats::outlier_bounds",
		"system::stats::correlation",
	}
	for _, name := range aliases {
		if _, ok := m[name]; !ok {
			t.Errorf("namespace alias %q not registered", name)
		}
	}
}

// =============================================================================
// NEW function tests
// =============================================================================

// --- Text: ends_with, capitalize, title_case, pad_left, pad_right,
//     word_count, repeat, reverse_text ---

func TestEndsWith(t *testing.T) {
	m := setup()
	tests := []struct {
		s, suffix string
		want      bool
	}{
		{"hello", "lo", true},
		{"hello", "xyz", false},
		{"hello", "hello", true},
		{"hello", "", true},
		{"", "", true},
	}
	for _, tc := range tests {
		got := callBuiltin(t, m, "ends_with", tc.s, tc.suffix)
		if got != tc.want {
			t.Errorf("ends_with(%q, %q): got %v, want %v", tc.s, tc.suffix, got, tc.want)
		}
	}
}

func TestCapitalize(t *testing.T) {
	m := setup()
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "Hello"},
		{"", ""},
		{"HELLO", "HELLO"},
		{"a", "A"},
	}
	for _, tc := range tests {
		got := callBuiltin(t, m, "capitalize", tc.input)
		if got != tc.want {
			t.Errorf("capitalize(%q): got %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestTitleCase(t *testing.T) {
	m := setup()
	tests := []struct {
		input string
		want  string
	}{
		{"hello world", "Hello World"},
		{"", ""},
		{"already Title", "Already Title"},
		{"one", "One"},
	}
	for _, tc := range tests {
		got := callBuiltin(t, m, "title_case", tc.input)
		if got != tc.want {
			t.Errorf("title_case(%q): got %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestPadLeft(t *testing.T) {
	m := setup()
	// Default pad char (space)
	got := callBuiltin(t, m, "pad_left", "hi", int64(5))
	if got != "   hi" {
		t.Errorf("pad_left(\"hi\", 5): got %q, want %q", got, "   hi")
	}
	// Custom pad char
	got = callBuiltin(t, m, "pad_left", "hi", int64(5), "0")
	if got != "000hi" {
		t.Errorf("pad_left(\"hi\", 5, \"0\"): got %q, want %q", got, "000hi")
	}
	// Input longer than target length: truncates from the left
	got = callBuiltin(t, m, "pad_left", "hello", int64(3))
	if got != "llo" {
		t.Errorf("pad_left(\"hello\", 3): got %q, want %q", got, "llo")
	}
}

func TestPadRight(t *testing.T) {
	m := setup()
	got := callBuiltin(t, m, "pad_right", "hi", int64(5))
	if got != "hi   " {
		t.Errorf("pad_right(\"hi\", 5): got %q, want %q", got, "hi   ")
	}
	got = callBuiltin(t, m, "pad_right", "hi", int64(5), "0")
	if got != "hi000" {
		t.Errorf("pad_right(\"hi\", 5, \"0\"): got %q, want %q", got, "hi000")
	}
}

func TestWordCount(t *testing.T) {
	m := setup()
	tests := []struct {
		input string
		want  int64
	}{
		{"hello world", 2},
		{"", 0},
		{"  spaces  ", 1},
		{"one two three four", 4},
	}
	for _, tc := range tests {
		got := callBuiltin(t, m, "word_count", tc.input)
		if got != tc.want {
			t.Errorf("word_count(%q): got %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestRepeatStr(t *testing.T) {
	m := setup()
	tests := []struct {
		s    string
		n    int64
		want string
	}{
		{"ab", 3, "ababab"},
		{"x", 0, ""},
		{"hi", 1, "hi"},
	}
	for _, tc := range tests {
		got := callBuiltin(t, m, "repeat", tc.s, tc.n)
		if got != tc.want {
			t.Errorf("repeat(%q, %d): got %q, want %q", tc.s, tc.n, got, tc.want)
		}
	}
}

func TestReverseText(t *testing.T) {
	m := setup()
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "olleh"},
		{"", ""},
		{"a", "a"},
		{"ab", "ba"},
	}
	for _, tc := range tests {
		got := callBuiltin(t, m, "reverse_text", tc.input)
		if got != tc.want {
			t.Errorf("reverse_text(%q): got %q, want %q", tc.input, got, tc.want)
		}
	}
}

// --- Formatting: format_number, format_currency, format_percent ---

func TestFormatNumber(t *testing.T) {
	m := setup()
	// Default: 2 decimals, comma separator
	got := callBuiltin(t, m, "format_number", 1234567.89)
	if got != "1,234,567.89" {
		t.Errorf("format_number(1234567.89): got %q, want %q", got, "1,234,567.89")
	}
	// 0 decimals
	got = callBuiltin(t, m, "format_number", 1234567.89, int64(0))
	if got != "1,234,568" {
		t.Errorf("format_number(1234567.89, 0): got %q, want %q", got, "1,234,568")
	}
	// European style: dot as thousands separator, comma as decimal
	got = callBuiltin(t, m, "format_number", 1234.5, int64(2), ".")
	if got != "1.234,50" {
		t.Errorf("format_number(1234.5, 2, \".\"): got %q, want %q", got, "1.234,50")
	}
}

func TestFormatCurrency(t *testing.T) {
	m := setup()
	// Default USD
	got := callBuiltin(t, m, "format_currency", 1234.50)
	if got != "$1,234.50" {
		t.Errorf("format_currency(1234.50): got %q, want %q", got, "$1,234.50")
	}
	// EUR
	got = callBuiltin(t, m, "format_currency", 1234.50, "EUR")
	if got != "€1,234.50" {
		t.Errorf("format_currency(1234.50, EUR): got %q, want %q", got, "€1,234.50")
	}
	// GBP
	got = callBuiltin(t, m, "format_currency", 1234.50, "GBP")
	if got != "£1,234.50" {
		t.Errorf("format_currency(1234.50, GBP): got %q, want %q", got, "£1,234.50")
	}
}

func TestFormatPercent(t *testing.T) {
	m := setup()
	// Ratio auto-detected (between -1 and 1)
	got := callBuiltin(t, m, "format_percent", 0.856)
	if got != "85.6%" {
		t.Errorf("format_percent(0.856): got %q, want %q", got, "85.6%")
	}
	// Already a percentage value (>= 1)
	got = callBuiltin(t, m, "format_percent", 85.6)
	if got != "85.6%" {
		t.Errorf("format_percent(85.6): got %q, want %q", got, "85.6%")
	}
	// 0 decimals
	got = callBuiltin(t, m, "format_percent", 0.856, int64(0))
	if got != "86%" {
		t.Errorf("format_percent(0.856, 0): got %q, want %q", got, "86%")
	}
}

// --- Collections: includes, reverse, seq, min/max scalar ---
// Lambda-requiring functions (find, find_index, every, some, take_while,
// drop_while, distinct_by) are tested for registration only because
// callBuiltin cannot supply lambda closures. Full integration tests
// belong in the executor test suite.

func TestIncludes(t *testing.T) {
	m := setup()
	if callBuiltin(t, m, "includes", []any{int64(1), int64(2), int64(3)}, int64(2)) != true {
		t.Error("includes([1,2,3], 2) should be true")
	}
	if callBuiltin(t, m, "includes", []any{int64(1), int64(2), int64(3)}, int64(5)) != false {
		t.Error("includes([1,2,3], 5) should be false")
	}
	if callBuiltin(t, m, "includes", []any{"a", "b", "c"}, "b") != true {
		t.Error("includes([a,b,c], b) should be true")
	}
}

func TestReverseArr(t *testing.T) {
	m := setup()
	got := callBuiltin(t, m, "reverse", []any{int64(1), int64(2), int64(3)})
	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("reverse: expected []any, got %T", got)
	}
	if len(arr) != 3 || arr[0] != int64(3) || arr[1] != int64(2) || arr[2] != int64(1) {
		t.Errorf("reverse([1,2,3]): got %v, want [3,2,1]", arr)
	}
}

func TestSeq(t *testing.T) {
	m := setup()
	// seq(5) -> [0,1,2,3,4]
	got := callBuiltin(t, m, "seq", int64(5))
	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("seq(5): expected []any, got %T", got)
	}
	if len(arr) != 5 {
		t.Fatalf("seq(5): got %d elements, want 5", len(arr))
	}
	for i := 0; i < 5; i++ {
		if arr[i] != int64(i) {
			t.Errorf("seq(5)[%d]: got %v, want %d", i, arr[i], i)
		}
	}

	// seq(2, 5) -> [2,3,4]
	got = callBuiltin(t, m, "seq", int64(2), int64(5))
	arr = got.([]any)
	if len(arr) != 3 || arr[0] != int64(2) || arr[1] != int64(3) || arr[2] != int64(4) {
		t.Errorf("seq(2,5): got %v, want [2,3,4]", arr)
	}

	// seq(5, 0) -> [5,4,3,2,1] (descending, step auto = -1)
	got = callBuiltin(t, m, "seq", int64(5), int64(0))
	arr = got.([]any)
	if len(arr) != 5 {
		t.Fatalf("seq(5,0): got %d elements, want 5", len(arr))
	}
	expected := []int64{5, 4, 3, 2, 1}
	for i, want := range expected {
		if arr[i] != want {
			t.Errorf("seq(5,0)[%d]: got %v, want %d", i, arr[i], want)
		}
	}
}

func TestMinScalar(t *testing.T) {
	m := setup()
	got := callBuiltin(t, m, "min", 3.0, 1.0, 5.0)
	if got != 1.0 {
		t.Errorf("min(3, 1, 5): got %v, want 1.0", got)
	}
}

func TestMaxScalar(t *testing.T) {
	m := setup()
	got := callBuiltin(t, m, "max", 3.0, 1.0, 5.0)
	if got != 5.0 {
		t.Errorf("max(3, 1, 5): got %v, want 5.0", got)
	}
}

// --- Objects: keys, values, entries, merge, pick, omit, has_key ---

func TestKeys(t *testing.T) {
	m := setup()
	got := callBuiltin(t, m, "keys", map[string]any{"b": 2, "a": 1})
	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("keys: expected []any, got %T", got)
	}
	if len(arr) != 2 || arr[0] != "a" || arr[1] != "b" {
		t.Errorf("keys({b:2, a:1}): got %v, want [a, b]", arr)
	}
}

func TestValues(t *testing.T) {
	m := setup()
	got := callBuiltin(t, m, "values", map[string]any{"b": 2, "a": 1})
	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("values: expected []any, got %T", got)
	}
	// Sorted by key: a=1, b=2
	if len(arr) != 2 || arr[0] != 1 || arr[1] != 2 {
		t.Errorf("values({b:2, a:1}): got %v, want [1, 2]", arr)
	}
}

func TestEntries(t *testing.T) {
	m := setup()
	got := callBuiltin(t, m, "entries", map[string]any{"a": 1})
	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("entries: expected []any, got %T", got)
	}
	if len(arr) != 1 {
		t.Fatalf("entries({a:1}): got %d entries, want 1", len(arr))
	}
	entry, ok := arr[0].(map[string]any)
	if !ok {
		t.Fatalf("entries: element expected map[string]any, got %T", arr[0])
	}
	if entry["key"] != "a" || entry["value"] != 1 {
		t.Errorf("entries({a:1}): got %v, want {key:a, value:1}", entry)
	}
}

func TestMerge(t *testing.T) {
	m := setup()
	got := callBuiltin(t, m, "merge", map[string]any{"a": 1}, map[string]any{"b": 2})
	merged, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("merge: expected map[string]any, got %T", got)
	}
	if merged["a"] != 1 || merged["b"] != 2 {
		t.Errorf("merge({a:1}, {b:2}): got %v", merged)
	}
}

func TestPick(t *testing.T) {
	m := setup()
	got := callBuiltin(t, m, "pick", map[string]any{"a": 1, "b": 2, "c": 3}, []any{"a", "c"})
	picked, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("pick: expected map[string]any, got %T", got)
	}
	if len(picked) != 2 || picked["a"] != 1 || picked["c"] != 3 {
		t.Errorf("pick({a:1,b:2,c:3}, [a,c]): got %v", picked)
	}
}

func TestOmit(t *testing.T) {
	m := setup()
	got := callBuiltin(t, m, "omit", map[string]any{"a": 1, "b": 2, "c": 3}, []any{"b"})
	omitted, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("omit: expected map[string]any, got %T", got)
	}
	if len(omitted) != 2 || omitted["a"] != 1 || omitted["c"] != 3 {
		t.Errorf("omit({a:1,b:2,c:3}, [b]): got %v", omitted)
	}
}

func TestHasKey(t *testing.T) {
	m := setup()
	if callBuiltin(t, m, "has_key", map[string]any{"a": 1}, "a") != true {
		t.Error("has_key({a:1}, a) should be true")
	}
	if callBuiltin(t, m, "has_key", map[string]any{"a": 1}, "b") != false {
		t.Error("has_key({a:1}, b) should be false")
	}
}

// --- Math: sign, random, random_int ---

func TestSign(t *testing.T) {
	m := setup()
	tests := []struct {
		input float64
		want  float64
	}{
		{5.0, 1.0},
		{-3.0, -1.0},
		{0.0, 0.0},
	}
	for _, tc := range tests {
		got := callBuiltin(t, m, "sign", tc.input)
		if got != tc.want {
			t.Errorf("sign(%v): got %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestRandom(t *testing.T) {
	m := setup()
	for i := 0; i < 10; i++ {
		got := callBuiltin(t, m, "random").(float64)
		if got < 0 || got >= 1 {
			t.Errorf("random(): got %v, want [0, 1)", got)
		}
	}
}

func TestRandomInt(t *testing.T) {
	m := setup()
	for i := 0; i < 20; i++ {
		got := callBuiltin(t, m, "random_int", int64(1), int64(10)).(int64)
		if got < 1 || got > 10 {
			t.Errorf("random_int(1, 10): got %v, want [1, 10]", got)
		}
	}
}

// --- DateTime: minute, second ---

func TestMinute(t *testing.T) {
	m := setup()
	dt := time.Date(2024, 3, 15, 10, 45, 30, 0, time.UTC)
	got := callBuiltin(t, m, "minute", dt)
	if got != int64(45) {
		t.Errorf("minute(10:45:30): got %v, want 45", got)
	}
}

func TestSecond(t *testing.T) {
	m := setup()
	dt := time.Date(2024, 3, 15, 10, 45, 30, 0, time.UTC)
	got := callBuiltin(t, m, "second", dt)
	if got != int64(30) {
		t.Errorf("second(10:45:30): got %v, want 30", got)
	}
}

// --- Casting aliases: to_int, to_float, to_bool, to_date, to_datetime ---

func TestCastingAliases(t *testing.T) {
	m := setup()

	// to_int
	got := callBuiltin(t, m, "to_int", 3.7)
	if got != int64(3) {
		t.Errorf("to_int(3.7): got %v, want 3", got)
	}

	// to_float
	got = callBuiltin(t, m, "to_float", int64(42))
	if got != 42.0 {
		t.Errorf("to_float(42): got %v, want 42.0", got)
	}

	// to_bool
	got = callBuiltin(t, m, "to_bool", int64(1))
	if got != true {
		t.Errorf("to_bool(1): got %v, want true", got)
	}

	// to_date
	got = callBuiltin(t, m, "to_date", "2024-06-15")
	dt, ok := got.(time.Time)
	if !ok {
		t.Fatalf("to_date: expected time.Time, got %T", got)
	}
	if dt.Year() != 2024 || dt.Month() != 6 || dt.Day() != 15 {
		t.Errorf("to_date(2024-06-15): got %v", dt)
	}

	// to_datetime
	got = callBuiltin(t, m, "to_datetime", "2024-06-15T08:30:00Z")
	dt, ok = got.(time.Time)
	if !ok {
		t.Fatalf("to_datetime: expected time.Time, got %T", got)
	}
	if dt.Hour() != 8 || dt.Minute() != 30 {
		t.Errorf("to_datetime(2024-06-15T08:30:00Z): got %v", dt)
	}
}

// --- Registration: verify all new function names exist in the map ---

func TestRegisterAll_NewFunctions(t *testing.T) {
	m := setup()
	newFunctions := []string{
		// Text
		"ends_with", "capitalize", "title_case", "pad_left", "pad_right",
		"word_count", "repeat", "reverse_text",
		// Formatting
		"format_number", "format_currency", "format_percent",
		// Collections (including lambda-based ones)
		"find", "find_index", "includes", "every", "some",
		"reverse", "seq", "take_while", "drop_while", "distinct_by",
		// Objects
		"keys", "values", "entries", "merge", "pick", "omit", "has_key",
		// Math
		"sign", "random", "random_int",
		// DateTime
		"minute", "second",
		// Casting aliases
		"to_int", "to_float", "to_bool", "to_date", "to_datetime",
	}
	for _, name := range newFunctions {
		if _, ok := m[name]; !ok {
			t.Errorf("new function %q not registered", name)
		}
	}
}

// --- Lambda-based collection functions: registration check ---
// find, find_index, every, some, take_while, drop_while, distinct_by
// require lambda closures that callBuiltin cannot supply. We verify
// they are registered above in TestRegisterAll_NewFunctions. Full
// behavioral tests belong in the executor integration test suite.

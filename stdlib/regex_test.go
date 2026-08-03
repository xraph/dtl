package stdlib

import (
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestRegexFunctions(t *testing.T) {
	tests := []struct {
		name string
		got  func() (any, error)
		want any
	}{
		{"test matches", func() (any, error) { return fnRegexTest([]any{"hello", "^h"}) }, true},
		{"test does not match", func() (any, error) { return fnRegexTest([]any{"hello", "^z"}) }, false},

		{"find first", func() (any, error) { return fnRegexFind([]any{"a1b22c", `\d+`}) }, "1"},
		// Null, not "", so "no match" stays distinct from "matched empty".
		{"find absent yields null", func() (any, error) { return fnRegexFind([]any{"abc", `\d`}) }, nil},

		{"find_all", func() (any, error) { return fnRegexFindAll([]any{"a1b22c", `\d+`}) }, []any{"1", "22"}},
		{"find_all none", func() (any, error) { return fnRegexFindAll([]any{"abc", `\d`}) }, []any{}},

		{"replace", func() (any, error) { return fnRegexReplace([]any{"a1b2", `\d`, "#"}) }, "a#b#"},
		{"replace expands groups", func() (any, error) {
			return fnRegexReplace([]any{"john smith", `(\w+) (\w+)`, "$2, $1"})
		}, "smith, john"},

		{"split", func() (any, error) { return fnRegexSplit([]any{"a1b22c", `\d+`}) }, []any{"a", "b", "c"}},

		{"escape", func() (any, error) { return fnRegexEscape([]any{"a.b*c"}) }, `a\.b\*c`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.got()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

// escape exists so a literal string can be used as a pattern safely.
func TestRegexEscapeMakesLiteralsMatchLiterally(t *testing.T) {
	literal := "a.b"
	escaped, err := fnRegexEscape([]any{literal})
	if err != nil {
		t.Fatal(err)
	}

	// The escaped pattern matches the literal text and nothing else.
	if got, _ := fnRegexTest([]any{"a.b", escaped}); got != true {
		t.Error("escaped pattern should match the literal text")
	}
	if got, _ := fnRegexTest([]any{"axb", escaped}); got != false {
		t.Error("escaped pattern should not treat . as a wildcard")
	}
}

func TestRegexGroups(t *testing.T) {
	t.Run("numbered and named", func(t *testing.T) {
		got, err := fnRegexGroups([]any{"2024-06-01", `(?P<year>\d{4})-(\d{2})-(?P<day>\d{2})`})
		if err != nil {
			t.Fatal(err)
		}
		obj, ok := got.(map[string]any)
		if !ok {
			t.Fatalf("got %T, want an object", got)
		}
		want := map[string]any{
			"0":    "2024-06-01",
			"1":    "2024",
			"2":    "06",
			"3":    "01",
			"year": "2024",
			"day":  "01",
		}
		if !reflect.DeepEqual(obj, want) {
			t.Errorf("got %#v, want %#v", obj, want)
		}
	})

	t.Run("no match yields null", func(t *testing.T) {
		got, err := fnRegexGroups([]any{"nope", `(\d+)`})
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Errorf("got %#v, want nil", got)
		}
	})
}

func TestInvalidPatternsAreReportedAgainstTheCallingFunction(t *testing.T) {
	const bad = "(unclosed"

	tests := []struct {
		name string
		call func() (any, error)
	}{
		{"regex::test", func() (any, error) { return fnRegexTest([]any{"s", bad}) }},
		{"regex::find", func() (any, error) { return fnRegexFind([]any{"s", bad}) }},
		{"regex::find_all", func() (any, error) { return fnRegexFindAll([]any{"s", bad}) }},
		{"regex::replace", func() (any, error) { return fnRegexReplace([]any{"s", bad, "x"}) }},
		{"regex::split", func() (any, error) { return fnRegexSplit([]any{"s", bad}) }},
		{"regex::groups", func() (any, error) { return fnRegexGroups([]any{"s", bad}) }},
		{"match_regex", func() (any, error) { return fnMatchRegex([]any{"s", bad}) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.call()
			if err == nil {
				t.Fatal("expected an error for an invalid pattern")
			}
			// The message must name the function the author called, not the
			// shared helper underneath it.
			if !strings.Contains(err.Error(), tt.name) {
				t.Errorf("error %q should name %q", err.Error(), tt.name)
			}
		})
	}
}

// match_regex and regex::find_all are one implementation under two names, so
// they must never disagree.
func TestMatchRegexAgreesWithFindAll(t *testing.T) {
	cases := [][2]string{
		{"a1b22c", `\d+`},
		{"abc", `\d`},
		{"aaa", "a"},
		{"", ".*"},
	}

	for _, c := range cases {
		t.Run(c[1], func(t *testing.T) {
			legacy, err1 := fnMatchRegex([]any{c[0], c[1]})
			modern, err2 := fnRegexFindAll([]any{c[0], c[1]})
			if (err1 == nil) != (err2 == nil) {
				t.Fatalf("errors disagree: %v vs %v", err1, err2)
			}
			if !reflect.DeepEqual(legacy, modern) {
				t.Errorf("match_regex = %#v but regex::find_all = %#v", legacy, modern)
			}
		})
	}
}

func TestPatternCacheReusesCompilations(t *testing.T) {
	const pattern = `^cache-test-\d+$`

	first, err := compilePattern(pattern)
	if err != nil {
		t.Fatal(err)
	}
	second, err := compilePattern(pattern)
	if err != nil {
		t.Fatal(err)
	}
	// Same pointer means the second call did not recompile.
	if first != second {
		t.Error("expected the cached *regexp.Regexp to be reused")
	}
}

// Caching failures matters as much as caching successes: a bad pattern inside a
// map over a large array would otherwise be recompiled once per element.
func TestPatternCacheRemembersFailures(t *testing.T) {
	const bad = "(cache-failure-test"

	_, err1 := compilePattern(bad)
	if err1 == nil {
		t.Fatal("expected a compile error")
	}
	_, err2 := compilePattern(bad)
	if err2 == nil {
		t.Fatal("expected a compile error on the second call too")
	}
	// The same error value comes back, which it only can if it was stored.
	if err1 != err2 {
		t.Error("expected the compile failure to be cached")
	}
}

// The bound is the security-relevant property: patterns can be built from data,
// so the cache must not grow without limit.
func TestPatternCacheIsBounded(t *testing.T) {
	for i := 0; i < patternCacheSize*3; i++ {
		if _, err := compilePattern("bounded-" + strconv.Itoa(i) + `-\d`); err != nil {
			t.Fatal(err)
		}
	}

	patternCache.mu.Lock()
	entries, order := len(patternCache.entries), patternCache.order.Len()
	patternCache.mu.Unlock()

	if entries > patternCacheSize {
		t.Errorf("cache holds %d entries, above the %d bound", entries, patternCacheSize)
	}
	// The index and the LRU list must stay in step, or eviction leaks.
	if entries != order {
		t.Errorf("index holds %d entries but the LRU list holds %d", entries, order)
	}
}

// Builtins are shared across goroutines by a registry serving concurrent
// executions, so the cache has to be race-safe. Run under -race.
func TestPatternCacheIsConcurrencySafe(t *testing.T) {
	var wg sync.WaitGroup
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				// A mix of shared and goroutine-private patterns, so the test
				// exercises both cache hits and concurrent eviction.
				if _, err := fnRegexTest([]any{"value-42", `^value-\d+$`}); err != nil {
					t.Error(err)
					return
				}
				if _, err := fnRegexTest([]any{"x", "concurrent-" + strconv.Itoa(g*100+i)}); err != nil {
					t.Error(err)
					return
				}
			}
		}(g)
	}
	wg.Wait()
}

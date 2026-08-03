package stdlib

import (
	"container/list"
	"fmt"
	"regexp"
	"sync"

	"github.com/xraph/dtl/executor"
)

// Regular expressions, over a bounded cache of compiled patterns.
//
// Go's regexp is RE2: matching is linear in the input and there is no
// catastrophic backtracking, so a pattern from untrusted data cannot be used to
// hang the executor. Compilation is the cost that matters, and without a cache
// it is paid per call — map(rows, r => regex::test(r.name, p)) over a large
// array compiles the same pattern once per row.
func registerRegex(m map[string]*executor.BuiltinFunc) {
	register(m, "regex::test", 2, 2, fnRegexTest,
		"regex::test(s, pattern) -> bool -- Whether the pattern matches anywhere in s")
	register(m, "regex::find", 2, 2, fnRegexFind,
		"regex::find(s, pattern) -> string -- First match, or null when there is none")
	register(m, "regex::find_all", 2, 2, fnRegexFindAll,
		"regex::find_all(s, pattern) -> string[] -- Every match of the pattern in s")
	register(m, "regex::replace", 3, 3, fnRegexReplace,
		"regex::replace(s, pattern, replacement) -> string -- Replaces every match. $1 and ${name} expand to captured groups")
	register(m, "regex::split", 2, 2, fnRegexSplit,
		"regex::split(s, pattern) -> string[] -- Splits s around every match of the pattern")
	register(m, "regex::groups", 2, 2, fnRegexGroups,
		"regex::groups(s, pattern) -> object -- Capture groups of the first match, keyed by name and by number. Null when nothing matches")
	register(m, "regex::escape", 1, 1, fnRegexEscape,
		"regex::escape(s) -> string -- Quotes regex metacharacters so s matches literally")
}

// patternCacheSize bounds the compiled-pattern cache.
//
// The bound is the point, not an optimisation: patterns can be built from data
// at runtime, so an unbounded cache would grow without limit on generated or
// adversarial input. A few hundred distinct patterns comfortably covers real
// programs, which reuse a handful.
const patternCacheSize = 256

// regexCache is an LRU over compile results, successes and failures alike.
// Caching failures matters as much as caching successes: an invalid pattern
// inside a map over a large array would otherwise be recompiled, and fail, once
// per element.
type regexCache struct {
	mu      sync.Mutex
	entries map[string]*list.Element
	order   *list.List // front is most recently used
}

type cacheEntry struct {
	pattern string
	re      *regexp.Regexp
	err     error
}

var patternCache = &regexCache{
	entries: make(map[string]*list.Element, patternCacheSize),
	order:   list.New(),
}

// compilePattern returns the compiled form of a pattern, compiling it at most
// once per eviction cycle.
//
// The whole operation holds one mutex, including the compile. Builtins are
// shared across goroutines by a registry that serves concurrent executions, so
// this must be race-safe; a double-checked scheme would let two goroutines
// compile the same pattern concurrently, which trades a rare duplicated
// compile for more moving parts than this is worth.
func compilePattern(pattern string) (*regexp.Regexp, error) {
	patternCache.mu.Lock()
	defer patternCache.mu.Unlock()

	if el, ok := patternCache.entries[pattern]; ok {
		patternCache.order.MoveToFront(el)
		entry, _ := el.Value.(*cacheEntry)
		return entry.re, entry.err
	}

	re, err := regexp.Compile(pattern)
	el := patternCache.order.PushFront(&cacheEntry{pattern: pattern, re: re, err: err})
	patternCache.entries[pattern] = el

	if patternCache.order.Len() > patternCacheSize {
		oldest := patternCache.order.Back()
		if oldest != nil {
			patternCache.order.Remove(oldest)
			if entry, ok := oldest.Value.(*cacheEntry); ok {
				delete(patternCache.entries, entry.pattern)
			}
		}
	}

	return re, err
}

// patternArg compiles the pattern argument, labelling any failure with the
// calling function so the author knows which call to fix.
func patternArg(fnName string, arg any) (*regexp.Regexp, error) {
	re, err := compilePattern(executor.ToString(arg))
	if err != nil {
		return nil, fmt.Errorf("%s: invalid pattern: %w", fnName, err)
	}
	return re, nil
}

func fnRegexTest(args []any) (any, error) {
	re, err := patternArg("regex::test", args[1])
	if err != nil {
		return nil, err
	}
	return re.MatchString(executor.ToString(args[0])), nil
}

// fnRegexFind returns null rather than an empty string when nothing matches, so
// "no match" is distinguishable from "matched an empty string".
func fnRegexFind(args []any) (any, error) {
	re, err := patternArg("regex::find", args[1])
	if err != nil {
		return nil, err
	}
	loc := re.FindStringIndex(executor.ToString(args[0]))
	if loc == nil {
		return nil, nil
	}
	return executor.ToString(args[0])[loc[0]:loc[1]], nil
}

func fnRegexFindAll(args []any) (any, error) {
	return findAll("regex::find_all", args)
}

// findAll backs both regex::find_all and the legacy match_regex. They share one
// implementation so the two spellings cannot come to disagree, but each passes
// its own name so a bad pattern is reported against the call the author wrote.
func findAll(fnName string, args []any) (any, error) {
	re, err := patternArg(fnName, args[1])
	if err != nil {
		return nil, err
	}
	matches := re.FindAllString(executor.ToString(args[0]), -1)
	result := make([]any, len(matches))
	for i, match := range matches {
		result[i] = match
	}
	return result, nil
}

func fnRegexReplace(args []any) (any, error) {
	re, err := patternArg("regex::replace", args[1])
	if err != nil {
		return nil, err
	}
	return re.ReplaceAllString(executor.ToString(args[0]), executor.ToString(args[2])), nil
}

func fnRegexSplit(args []any) (any, error) {
	re, err := patternArg("regex::split", args[1])
	if err != nil {
		return nil, err
	}
	parts := re.Split(executor.ToString(args[0]), -1)
	result := make([]any, len(parts))
	for i, p := range parts {
		result[i] = p
	}
	return result, nil
}

// fnRegexGroups exposes the first match's captures under both their number and,
// where the pattern names them, their name. Numbers are always present so a
// pattern does not have to be rewritten with names to be readable from DTL;
// group "0" is the whole match.
func fnRegexGroups(args []any) (any, error) {
	re, err := patternArg("regex::groups", args[1])
	if err != nil {
		return nil, err
	}

	match := re.FindStringSubmatch(executor.ToString(args[0]))
	if match == nil {
		return nil, nil
	}

	names := re.SubexpNames()
	result := make(map[string]any, len(match)*2)
	for i, group := range match {
		result[fmt.Sprintf("%d", i)] = group
		if i < len(names) && names[i] != "" {
			result[names[i]] = group
		}
	}
	return result, nil
}

func fnRegexEscape(args []any) (any, error) {
	return regexp.QuoteMeta(executor.ToString(args[0])), nil
}

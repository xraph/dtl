package stdlib

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/xraph/dtl/executor"
)

func registerText(m map[string]*executor.BuiltinFunc) {
	register(m, "upper", 1, 1, fnUpper,
		"upper(s) -> string -- Converts to UPPERCASE")
	register(m, "lower", 1, 1, fnLower,
		"lower(s) -> string -- Converts to lowercase")
	register(m, "trim", 1, 1, fnTrim,
		"trim(s) -> string -- Removes leading and trailing whitespace")
	register(m, "replace", 3, 3, fnReplace,
		"replace(s, old, new) -> string -- Replaces every occurrence of old with new")
	register(m, "split", 2, 2, fnSplit,
		"split(s, sep) -> string[] -- Splits the string on a literal separator")
	register(m, "join", 2, 2, fnJoin,
		"join(arr, sep) -> string -- Joins array elements with a separator")
	register(m, "starts_with", 2, 2, fnStartsWith,
		"starts_with(s, prefix) -> bool -- Whether s begins with prefix")
	register(m, "contains", 2, 2, fnContains,
		"contains(s, substr) -> bool -- Whether s contains substr")
	register(m, "substr", 2, 3, fnSubstr,
		"substr(s, start, length?) -> string -- Substring from start, measured in characters. Runs to the end when length is omitted")
	register(m, "slugify", 1, 1, fnSlugify,
		"slugify(s) -> string -- Normalises text into a lowercase URL-safe slug")
	register(m, "truncate", 2, 3, fnTruncate,
		"truncate(s, length, suffix?) -> string -- Shortens s to length, appending suffix (default '...') when cut")
	register(m, "extract_number", 1, 1, fnExtractNumber,
		"extract_number(s) -> float -- First number found in the string")
	register(m, "match_regex", 2, 2, fnMatchRegex,
		"match_regex(s, pattern) -> string[] -- Every match of pattern in s. Legacy spelling of regex::find_all")
	register(m, "ends_with", 2, 2, fnEndsWith,
		"ends_with(s, suffix) -> bool -- Whether s ends with suffix")
	register(m, "capitalize", 1, 1, fnCapitalize,
		"capitalize(s) -> string -- Uppercases the first letter")
	register(m, "title_case", 1, 1, fnTitleCase,
		"title_case(s) -> string -- Uppercases the first letter of each word")
	register(m, "pad_left", 2, 3, fnPadLeft,
		"pad_left(s, length, char?) -> string -- Pads the start of s to length characters (default pad ' ')")
	register(m, "pad_right", 2, 3, fnPadRight,
		"pad_right(s, length, char?) -> string -- Pads the end of s to length characters (default pad ' ')")
	register(m, "word_count", 1, 1, fnWordCount,
		"word_count(s) -> int -- Number of whitespace-separated words")
	register(m, "repeat", 2, 2, fnRepeatStr,
		"repeat(s, n) -> string -- Concatenates n copies of s")
	register(m, "reverse_text", 1, 1, fnReverseText,
		"reverse_text(s) -> string -- Reverses the string, preserving multi-byte characters")
	register(m, "trim_start", 1, 1, fnTrimStart,
		"trim_start(s) -> string -- Removes leading whitespace")
	register(m, "trim_end", 1, 1, fnTrimEnd,
		"trim_end(s) -> string -- Removes trailing whitespace")
	register(m, "trim_chars", 2, 2, fnTrimChars,
		"trim_chars(s, chars) -> string -- Removes any of the given characters from both ends")
	register(m, "index_of", 2, 2, fnIndexOf,
		"index_of(s, substr) -> int -- Character position of the first occurrence, or -1. Composes with substr")
	register(m, "last_index_of", 2, 2, fnLastIndexOf,
		"last_index_of(s, substr) -> int -- Character position of the last occurrence, or -1")
	register(m, "count_occurrences", 2, 2, fnCountOccurrences,
		"count_occurrences(s, substr) -> int -- Number of non-overlapping occurrences")
	register(m, "left", 2, 2, fnLeft,
		"left(s, n) -> string -- First n characters")
	register(m, "right", 2, 2, fnRight,
		"right(s, n) -> string -- Last n characters")
	register(m, "char_at", 2, 2, fnCharAt,
		"char_at(s, i) -> string -- Character at position i, or an empty string when out of range")
	register(m, "lines", 1, 1, fnLines,
		"lines(s) -> string[] -- Splits on line breaks, handling both LF and CRLF")
	register(m, "normalize_space", 1, 1, fnNormalizeSpace,
		"normalize_space(s) -> string -- Trims and collapses every internal whitespace run to a single space")
	register(m, "strip_prefix", 2, 2, fnStripPrefix,
		"strip_prefix(s, prefix) -> string -- Removes prefix when present, otherwise returns s unchanged")
	register(m, "strip_suffix", 2, 2, fnStripSuffix,
		"strip_suffix(s, suffix) -> string -- Removes suffix when present, otherwise returns s unchanged")
	register(m, "mask", 1, 3, fnMask,
		"mask(s, keep_last?, char?) -> string -- Replaces all but the last characters (default 4) with a mask character (default '*')")

	// Legacy namespace spellings, aliased so they cannot drift from the bare names.
	alias(m, "system::text::slugify", "slugify")
	alias(m, "system::text::truncate", "truncate")
	alias(m, "system::text::extract_number", "extract_number")
	alias(m, "system::text::ends_with", "ends_with")
	alias(m, "system::text::capitalize", "capitalize")
	alias(m, "system::text::title_case", "title_case")
	alias(m, "system::text::pad_left", "pad_left")
	alias(m, "system::text::pad_right", "pad_right")
}

func fnUpper(args []any) (any, error) {
	return strings.ToUpper(executor.ToString(args[0])), nil
}

func fnLower(args []any) (any, error) {
	return strings.ToLower(executor.ToString(args[0])), nil
}

func fnTrim(args []any) (any, error) {
	return strings.TrimSpace(executor.ToString(args[0])), nil
}

func fnReplace(args []any) (any, error) {
	s := executor.ToString(args[0])
	old := executor.ToString(args[1])
	newStr := executor.ToString(args[2])
	return strings.ReplaceAll(s, old, newStr), nil
}

func fnSplit(args []any) (any, error) {
	s := executor.ToString(args[0])
	sep := executor.ToString(args[1])
	parts := strings.Split(s, sep)
	result := make([]any, len(parts))
	for i, p := range parts {
		result[i] = p
	}
	return result, nil
}

func fnJoin(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok {
		return executor.ToString(args[0]), nil
	}
	sep := executor.ToString(args[1])
	parts := make([]string, len(arr))
	for i, v := range arr {
		parts[i] = executor.ToString(v)
	}
	return strings.Join(parts, sep), nil
}

func fnStartsWith(args []any) (any, error) {
	s := executor.ToString(args[0])
	prefix := executor.ToString(args[1])
	return strings.HasPrefix(s, prefix), nil
}

func fnContains(args []any) (any, error) {
	s := executor.ToString(args[0])
	substr := executor.ToString(args[1])
	return strings.Contains(s, substr), nil
}

func fnSubstr(args []any) (any, error) {
	s := executor.ToString(args[0])
	start := int(executor.ToInt(args[1]))
	runes := []rune(s)

	if start < 0 {
		start = 0
	}
	if start >= len(runes) {
		return "", nil
	}

	end := len(runes)
	if len(args) > 2 {
		length := int(executor.ToInt(args[2]))
		end = start + length
		if end > len(runes) {
			end = len(runes)
		}
	}

	return string(runes[start:end]), nil
}

func fnSlugify(args []any) (any, error) {
	s := executor.ToString(args[0])
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	// Remove non-alphanumeric except hyphens
	re := regexp.MustCompile(`[^a-z0-9-]`)
	s = re.ReplaceAllString(s, "")
	return s, nil
}

func fnTruncate(args []any) (any, error) {
	s := executor.ToString(args[0])
	maxLen := int(executor.ToInt(args[1]))
	suffix := "..."
	if len(args) > 2 {
		suffix = executor.ToString(args[2])
	}

	runes := []rune(s)
	if len(runes) <= maxLen {
		return s, nil
	}
	cutoff := maxLen - len([]rune(suffix))
	if cutoff < 0 {
		cutoff = 0
	}
	return string(runes[:cutoff]) + suffix, nil
}

func fnExtractNumber(args []any) (any, error) {
	s := executor.ToString(args[0])
	re := regexp.MustCompile(`[0-9]+\.?[0-9]*`)
	match := re.FindString(s)
	if match == "" {
		return nil, nil
	}
	var f float64
	_, err := fmt.Sscanf(match, "%f", &f)
	if err != nil {
		return nil, nil
	}
	return f, nil
}

// fnMatchRegex is the legacy spelling of regex::find_all, which it delegates to
// rather than reimplementing. Keeping one implementation means the two names
// cannot come to disagree, and match_regex picks up the compiled-pattern cache
// it previously lacked — it used to call regexp.Compile on every invocation, so
// mapping it over a large array recompiled the same pattern once per element.
func fnMatchRegex(args []any) (any, error) {
	return findAll("match_regex", args)
}

func fnEndsWith(args []any) (any, error) {
	s := executor.ToString(args[0])
	suffix := executor.ToString(args[1])
	return strings.HasSuffix(s, suffix), nil
}

func fnCapitalize(args []any) (any, error) {
	s := executor.ToString(args[0])
	if s == "" {
		return s, nil
	}
	runes := []rune(s)
	runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
	return string(runes), nil
}

func fnTitleCase(args []any) (any, error) {
	s := executor.ToString(args[0])
	words := strings.Fields(s)
	for i, w := range words {
		if w == "" {
			continue
		}
		runes := []rune(w)
		runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
		words[i] = string(runes)
	}
	return strings.Join(words, " "), nil
}

func fnPadLeft(args []any) (any, error) {
	s := executor.ToString(args[0])
	length := int(executor.ToInt(args[1]))
	pad := " "
	if len(args) > 2 {
		pad = executor.ToString(args[2])
	}
	if pad == "" {
		pad = " "
	}
	for len([]rune(s)) < length {
		s = pad + s
	}
	runes := []rune(s)
	if len(runes) > length {
		s = string(runes[len(runes)-length:])
	}
	return s, nil
}

func fnPadRight(args []any) (any, error) {
	s := executor.ToString(args[0])
	length := int(executor.ToInt(args[1]))
	pad := " "
	if len(args) > 2 {
		pad = executor.ToString(args[2])
	}
	if pad == "" {
		pad = " "
	}
	for len([]rune(s)) < length {
		s = s + pad
	}
	runes := []rune(s)
	if len(runes) > length {
		s = string(runes[:length])
	}
	return s, nil
}

func fnWordCount(args []any) (any, error) {
	s := executor.ToString(args[0])
	words := strings.Fields(s)
	return int64(len(words)), nil
}

func fnRepeatStr(args []any) (any, error) {
	s := executor.ToString(args[0])
	n := int(executor.ToInt(args[1]))
	if n < 0 {
		n = 0
	}
	return strings.Repeat(s, n), nil
}

func fnReverseText(args []any) (any, error) {
	s := executor.ToString(args[0])
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes), nil
}

// The functions below measure and index in characters, not bytes.
//
// That matches substr, pad_left, truncate, and reverse_text, which all convert
// to []rune. The point is composition: substr(s, index_of(s, x)) has to mean
// something, and it only does if both count the same units. Go's strings.Index
// returns a byte offset, so every position here is converted before it is
// returned.

func fnTrimStart(args []any) (any, error) {
	return strings.TrimLeftFunc(executor.ToString(args[0]), unicode.IsSpace), nil
}

func fnTrimEnd(args []any) (any, error) {
	return strings.TrimRightFunc(executor.ToString(args[0]), unicode.IsSpace), nil
}

func fnTrimChars(args []any) (any, error) {
	return strings.Trim(executor.ToString(args[0]), executor.ToString(args[1])), nil
}

// runeIndex converts a byte offset into a character offset, passing -1 through
// unchanged so "not found" survives the conversion.
func runeIndex(s string, byteOffset int) int64 {
	if byteOffset < 0 {
		return -1
	}
	return int64(len([]rune(s[:byteOffset])))
}

func fnIndexOf(args []any) (any, error) {
	s := executor.ToString(args[0])
	return runeIndex(s, strings.Index(s, executor.ToString(args[1]))), nil
}

func fnLastIndexOf(args []any) (any, error) {
	s := executor.ToString(args[0])
	return runeIndex(s, strings.LastIndex(s, executor.ToString(args[1]))), nil
}

func fnCountOccurrences(args []any) (any, error) {
	sub := executor.ToString(args[1])
	if sub == "" {
		// strings.Count counts boundaries for an empty needle, which is a
		// surprising answer to "how many times does '' appear".
		return int64(0), nil
	}
	return int64(strings.Count(executor.ToString(args[0]), sub)), nil
}

func fnLeft(args []any) (any, error) {
	runes := []rune(executor.ToString(args[0]))
	n := int(executor.ToInt(args[1]))
	if n <= 0 {
		return "", nil
	}
	if n >= len(runes) {
		return string(runes), nil
	}
	return string(runes[:n]), nil
}

func fnRight(args []any) (any, error) {
	runes := []rune(executor.ToString(args[0]))
	n := int(executor.ToInt(args[1]))
	if n <= 0 {
		return "", nil
	}
	if n >= len(runes) {
		return string(runes), nil
	}
	return string(runes[len(runes)-n:]), nil
}

func fnCharAt(args []any) (any, error) {
	runes := []rune(executor.ToString(args[0]))
	i := int(executor.ToInt(args[1]))
	if i < 0 || i >= len(runes) {
		return "", nil
	}
	return string(runes[i]), nil
}

func fnLines(args []any) (any, error) {
	s := executor.ToString(args[0])
	if s == "" {
		return []any{}, nil
	}
	// Normalise CRLF first so a Windows-authored payload does not leave a
	// stray carriage return at the end of every line.
	s = strings.ReplaceAll(s, "\r\n", "\n")
	parts := strings.Split(s, "\n")
	out := make([]any, len(parts))
	for i, p := range parts {
		out[i] = p
	}
	return out, nil
}

func fnNormalizeSpace(args []any) (any, error) {
	return strings.Join(strings.Fields(executor.ToString(args[0])), " "), nil
}

func fnStripPrefix(args []any) (any, error) {
	return strings.TrimPrefix(executor.ToString(args[0]), executor.ToString(args[1])), nil
}

func fnStripSuffix(args []any) (any, error) {
	return strings.TrimSuffix(executor.ToString(args[0]), executor.ToString(args[1])), nil
}

// fnMask redacts all but a trailing window. The default of four trailing
// characters is the familiar card- and account-number convention.
//
// When the string is no longer than the window it is masked completely rather
// than returned intact: revealing a short value in full would defeat the point
// of calling mask on it.
func fnMask(args []any) (any, error) {
	runes := []rune(executor.ToString(args[0]))

	keep := 4
	if len(args) > 1 {
		keep = int(executor.ToInt(args[1]))
	}
	if keep < 0 {
		keep = 0
	}

	maskChar := '*'
	if len(args) > 2 {
		if mc := []rune(executor.ToString(args[2])); len(mc) > 0 {
			maskChar = mc[0]
		}
	}

	if keep >= len(runes) {
		return strings.Repeat(string(maskChar), len(runes)), nil
	}

	masked := make([]rune, len(runes))
	cut := len(runes) - keep
	for i := range runes {
		if i < cut {
			masked[i] = maskChar
		} else {
			masked[i] = runes[i]
		}
	}
	return string(masked), nil
}

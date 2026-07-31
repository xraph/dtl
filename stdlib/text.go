package stdlib

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/xraph/dtl/executor"
)

func registerText(m map[string]*executor.BuiltinFunc) {
	register(m, "upper", 1, 1, fnUpper)
	register(m, "lower", 1, 1, fnLower)
	register(m, "trim", 1, 1, fnTrim)
	register(m, "replace", 3, 3, fnReplace)
	register(m, "split", 2, 2, fnSplit)
	register(m, "join", 2, 2, fnJoin)
	register(m, "starts_with", 2, 2, fnStartsWith)
	register(m, "contains", 2, 2, fnContains)
	register(m, "substr", 2, 3, fnSubstr)
	register(m, "slugify", 1, 1, fnSlugify)
	register(m, "truncate", 2, 3, fnTruncate)
	register(m, "extract_number", 1, 1, fnExtractNumber)
	register(m, "match_regex", 2, 2, fnMatchRegex)
	register(m, "ends_with", 2, 2, fnEndsWith)
	register(m, "capitalize", 1, 1, fnCapitalize)
	register(m, "title_case", 1, 1, fnTitleCase)
	register(m, "pad_left", 2, 3, fnPadLeft)
	register(m, "pad_right", 2, 3, fnPadRight)
	register(m, "word_count", 1, 1, fnWordCount)
	register(m, "repeat", 2, 2, fnRepeatStr)
	register(m, "reverse_text", 1, 1, fnReverseText)

	// Namespace aliases
	register(m, "system::text::slugify", 1, 1, fnSlugify)
	register(m, "system::text::truncate", 2, 3, fnTruncate)
	register(m, "system::text::extract_number", 1, 1, fnExtractNumber)
	register(m, "system::text::ends_with", 2, 2, fnEndsWith)
	register(m, "system::text::capitalize", 1, 1, fnCapitalize)
	register(m, "system::text::title_case", 1, 1, fnTitleCase)
	register(m, "system::text::pad_left", 2, 3, fnPadLeft)
	register(m, "system::text::pad_right", 2, 3, fnPadRight)
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

func fnMatchRegex(args []any) (any, error) {
	s := executor.ToString(args[0])
	pattern := executor.ToString(args[1])
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("match_regex: invalid pattern: %w", err)
	}
	matches := re.FindAllString(s, -1)
	result := make([]any, len(matches))
	for i, m := range matches {
		result[i] = m
	}
	return result, nil
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

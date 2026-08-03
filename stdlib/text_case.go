package stdlib

import (
	"strings"
	"unicode"

	"github.com/xraph/dtl/executor"
)

// Case conversions between the naming styles data arrives in. They share one
// word splitter so that every style agrees on where the word boundaries are:
// converting to snake_case and back to camelCase has to be stable, and it only
// is if both start from the same segmentation.
func registerTextCase(m map[string]*executor.BuiltinFunc) {
	register(m, "snake_case", 1, 1, fnSnakeCase,
		"snake_case(s) -> string -- Converts to snake_case")
	register(m, "camel_case", 1, 1, fnCamelCase,
		"camel_case(s) -> string -- Converts to camelCase")
	register(m, "pascal_case", 1, 1, fnPascalCase,
		"pascal_case(s) -> string -- Converts to PascalCase")
	register(m, "kebab_case", 1, 1, fnKebabCase,
		"kebab_case(s) -> string -- Converts to kebab-case")
}

// splitWords segments a string into lowercase words.
//
// Boundaries are any run of non-alphanumeric characters, plus the transitions
// within camelCase and PascalCase. An acronym is kept whole: "parseHTTPResponse"
// segments as parse/http/response rather than parse/h/t/t/p/response, because
// the uppercase run only breaks before the final capital when a lowercase
// letter follows it.
func splitWords(s string) []string {
	var words []string
	var current []rune

	flush := func() {
		if len(current) > 0 {
			words = append(words, strings.ToLower(string(current)))
			current = nil
		}
	}

	runes := []rune(s)
	for i, r := range runes {
		switch {
		case unicode.IsUpper(r):
			// Start a new word when the previous rune was lowercase or a digit
			// ("fooBar"), or when this capital begins a word after an acronym
			// ("HTTPResponse" -> http, response).
			if i > 0 {
				prev := runes[i-1]
				nextIsLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
				if unicode.IsLower(prev) || unicode.IsDigit(prev) ||
					(unicode.IsUpper(prev) && nextIsLower) {
					flush()
				}
			}
			current = append(current, r)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			current = append(current, r)
		default:
			// Any separator — space, hyphen, underscore, punctuation.
			flush()
		}
	}
	flush()
	return words
}

func fnSnakeCase(args []any) (any, error) {
	return strings.Join(splitWords(executor.ToString(args[0])), "_"), nil
}

func fnKebabCase(args []any) (any, error) {
	return strings.Join(splitWords(executor.ToString(args[0])), "-"), nil
}

func fnCamelCase(args []any) (any, error) {
	words := splitWords(executor.ToString(args[0]))
	if len(words) == 0 {
		return "", nil
	}
	var b strings.Builder
	b.WriteString(words[0])
	for _, w := range words[1:] {
		b.WriteString(upperFirst(w))
	}
	return b.String(), nil
}

func fnPascalCase(args []any) (any, error) {
	var b strings.Builder
	for _, w := range splitWords(executor.ToString(args[0])) {
		b.WriteString(upperFirst(w))
	}
	return b.String(), nil
}

// upperFirst uppercases the first rune, leaving the rest alone. Operating on
// runes rather than bytes so a leading multi-byte character is not corrupted.
func upperFirst(w string) string {
	if w == "" {
		return w
	}
	runes := []rune(w)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

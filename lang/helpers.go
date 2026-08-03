package lang

import (
	"fmt"
	"strings"
)

func extractWordPrefix(source string, position int) string {
	if position > len(source) {
		position = len(source)
	}
	end := position
	start := end
	for start > 0 {
		ch := source[start-1]
		if isIdentChar(ch) {
			start--
		} else {
			break
		}
	}
	if start == end {
		return ""
	}
	return source[start:end]
}

func extractWordAtPosition(source string, position int) string {
	if position > len(source) {
		position = len(source)
	}
	start := position
	for start > 0 && isIdentChar(source[start-1]) {
		start--
	}
	end := position
	for end < len(source) && isIdentChar(source[end]) {
		end++
	}
	if start == end {
		return ""
	}
	return source[start:end]
}

func isIdentChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_'
}

func kindOrder(kind string) int {
	switch kind {
	case "keyword":
		return 0
	case "function":
		return 1
	case "variable":
		return 2
	case "type":
		return 3
	case "snippet":
		return 4
	default:
		return 5
	}
}

// globalObjectDoc returns documentation for global.* paths based on cursor context.
func globalObjectDoc(source string, position int, word string) string {
	// Expand the word to include dotted context (e.g., "global.env" when cursor is on "env")
	fullPath := expandDottedPath(source, position)

	switch {
	case fullPath == "global" || word == "global":
		return "global -- Read-only context object providing access to environment variables and secrets.\n\nNamespaces:\n  global.env.KEY_NAME      -- access environment variables\n  global.secrets.KEY_NAME  -- access encrypted secrets\n\nSupports dot-path navigation into JSON/YAML values:\n  global.secrets.DB_CONFIG.host"
	case fullPath == "global.env" || (word == "env" && strings.Contains(source, "global.env")):
		return "global.env -- Environment variables for the current workspace.\n\nAccess by key name: global.env.DATABASE_URL\nDot-path into JSON values: global.env.FEATURE_FLAGS.dark_mode\n\nEquivalent to: ENV(\"KEY_NAME\")"
	case fullPath == "global.secrets" || (word == "secrets" && strings.Contains(source, "global.secrets")):
		return "global.secrets -- Encrypted secrets for the current workspace.\n\nAccess by key name: global.secrets.API_KEY\nDot-path into JSON/YAML values: global.secrets.DB_CONFIG.host\n\nEquivalent to: SECRET(\"KEY_NAME\")"
	case strings.HasPrefix(fullPath, "global.env."):
		key := strings.TrimPrefix(fullPath, "global.env.")
		return fmt.Sprintf("global.env.%s -- Environment variable '%s'\n\nEquivalent to: ENV(\"%s\")", key, key, key)
	case strings.HasPrefix(fullPath, "global.secrets."):
		key := strings.TrimPrefix(fullPath, "global.secrets.")
		return fmt.Sprintf("global.secrets.%s -- Encrypted secret '%s'\n\nEquivalent to: SECRET(\"%s\")", key, key, key)
	}
	return ""
}

// expandDottedPath extracts the full dotted path around the cursor position.
func expandDottedPath(source string, position int) string {
	if position > len(source) {
		position = len(source)
	}
	start := position
	for start > 0 && (isIdentChar(source[start-1]) || source[start-1] == '.') {
		start--
	}
	end := position
	for end < len(source) && (isIdentChar(source[end]) || source[end] == '.') {
		end++
	}
	if start == end {
		return ""
	}
	return source[start:end]
}

// keywordDoc returns documentation for DTL keywords.
func keywordDoc(word string) string {
	docs := map[string]string{
		"fn":     "fn name(params) -> type: body -- Defines a function",
		"let":    "let name = expr -- Declares an immutable variable",
		"return": "return expr -- Returns a value from the function",
		"if":     "if condition then value else other -- Conditional expression",
		"match":  "match subject: when pattern => result -- Pattern matching",
		"try":    "try expr catch default -- Error handling (returns default on failure)",
		"for":    "for item in collection: body -- Maps over a collection, returns array",
		"in":     "value in collection -- Membership test (arrays and objects)",
		"raise":  "raise message -- Raises a user-defined error (caught by try/catch)",
		"use":    "use namespace -- Imports a namespace shortcut (e.g., use maintenance)",
	}
	return docs[word]
}

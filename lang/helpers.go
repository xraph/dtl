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

// builtinDocs returns documentation for known builtin functions.
func builtinDocs(name string) string {
	// #nosec G101 -- these are documentation strings for builtin functions, not
	// credentials. The scanner matches on entries describing the secrets and
	// env builtins; the values are help text shown in an editor tooltip.
	docs := map[string]string{
		// Core
		"len":       "len(x) -> int -- Returns the length of a string, array, or object",
		"type_of":   "type_of(x) -> string -- Returns the type name: 'int', 'float', 'string', 'bool', 'null', 'array', 'object', 'datetime'",
		"is_null":   "is_null(x) -> bool -- Returns true if x is null",
		"is_blank":  "is_blank(x) -> bool -- Returns true if x is null, an empty/whitespace string, or an empty array/object",
		"to_string": "to_string(x) -> string -- Converts any value to its string representation",
		"abs":       "abs(x) -> number -- Returns the absolute value",
		"round":     "round(x, decimals?) -> number -- Rounds to the given decimal places (default: 0)",
		"ceil":      "ceil(x) -> int -- Rounds up to nearest integer",
		"floor":     "floor(x) -> int -- Rounds down to nearest integer",
		"power":     "power(base, exp) -> number -- Returns base raised to the power of exp",
		"sqrt":      "sqrt(x) -> float -- Returns the square root",
		"now":       "now() -> datetime -- Returns the current date and time",
		"today":     "today() -> datetime -- Returns today's date at midnight",
		// Text
		"upper":       "upper(s) -> string -- Converts to UPPERCASE",
		"lower":       "lower(s) -> string -- Converts to lowercase",
		"trim":        "trim(s) -> string -- Removes leading/trailing whitespace",
		"replace":     "replace(s, old, new) -> string -- Replaces all occurrences of old with new",
		"split":       "split(s, sep) -> string[] -- Splits string by separator",
		"join":        "join(arr, sep) -> string -- Joins array elements with separator",
		"starts_with": "starts_with(s, prefix) -> bool -- Checks if s starts with prefix",
		"ends_with":   "ends_with(s, suffix) -> bool -- Checks if s ends with suffix",
		"contains":    "contains(s, substr) -> bool -- Checks if s contains substr",
		"capitalize":  "capitalize(s) -> string -- Uppercases the first letter",
		"title_case":  "title_case(s) -> string -- Uppercases first letter of each word",
		"pad_left":    "pad_left(s, length, char?) -> string -- Pads string on the left to reach length",
		"pad_right":   "pad_right(s, length, char?) -> string -- Pads string on the right to reach length",
		"word_count":  "word_count(s) -> int -- Counts words in the string",
		// Collections
		"map":      "map(arr, fn) -> array -- Applies fn to each element",
		"filter":   "filter(arr, fn) -> array -- Keeps elements where fn returns true",
		"reduce":   "reduce(arr, init, fn) -> any -- Reduces array to single value",
		"sort":     "sort(arr, dir?) -> array -- Sorts array (dir: 'asc' or 'desc')",
		"find":     "find(arr, fn) -> any -- Returns first element where fn returns true",
		"includes": "includes(arr, value) -> bool -- Checks if value exists in array",
		"every":    "every(arr, fn) -> bool -- True if fn returns true for all elements",
		"some":     "some(arr, fn) -> bool -- True if fn returns true for any element",
		"sum":      "sum(arr) -> number -- Sums all elements",
		"avg":      "avg(arr) -> number -- Averages all elements",
		"min":      "min(arr) or min(a, b, ...) -> number -- Minimum value",
		"max":      "max(arr) or max(a, b, ...) -> number -- Maximum value",
		"count":    "count(arr) -> int -- Number of elements",
		"unique":   "unique(arr) -> array -- Removes duplicates",
		"reverse":  "reverse(arr) -> array -- Reverses array order",
		"seq":      "seq(end) or seq(start, end, step?) -> int[] -- Generates a sequence of integers",
		// Formatting
		"format_number":   "format_number(n, decimals?, separator?) -> string -- Formats number with thousands separator",
		"format_currency": "format_currency(n, currency?, locale?) -> string -- Formats as currency (USD, EUR, GBP, etc.)",
		"format_percent":  "format_percent(n, decimals?) -> string -- Formats as percentage",
		// Objects
		"keys":    "keys(obj) -> string[] -- Returns sorted keys of an object",
		"values":  "values(obj) -> array -- Returns values of an object (sorted by key)",
		"entries": "entries(obj) -> array -- Returns [{key, value}] entries",
		"merge":   "merge(obj1, obj2, ...) -> object -- Merges objects (later values win)",
		"pick":    "pick(obj, keys) -> object -- Picks only specified keys",
		"omit":    "omit(obj, keys) -> object -- Omits specified keys",
		"has_key": "has_key(obj, key) -> bool -- Checks if key exists",
		// Math
		"clamp":      "clamp(val, min, max) -> number -- Restricts value to [min, max] range",
		"sign":       "sign(x) -> number -- Returns -1, 0, or 1",
		"random":     "random() -> float -- Random number between 0 and 1",
		"random_int": "random_int(min, max) -> int -- Random integer in [min, max]",
		// DateTime
		"dt_add":    "dt_add(dt, amount, unit) -> datetime -- Adds time to a datetime",
		"dt_format": "dt_format(dt, format) -> string -- Formats datetime (YYYY-MM-DD HH:mm:ss)",
		"year":      "year(dt) -> int -- Extracts the year",
		"month":     "month(dt) -> int -- Extracts the month (1-12)",
		"day":       "day(dt) -> int -- Extracts the day of month",
		"hour":      "hour(dt) -> int -- Extracts the hour (0-23)",
		"minute":    "minute(dt) -> int -- Extracts the minute (0-59)",
		"second":    "second(dt) -> int -- Extracts the second (0-59)",
		// Casting
		"to_int":      "to_int(x) -> int -- Converts to integer",
		"to_float":    "to_float(x) -> float -- Converts to float",
		"to_bool":     "to_bool(x) -> bool -- Converts to boolean",
		"to_date":     "to_date(x, format?) -> datetime -- Parses string to datetime",
		"to_datetime": "to_datetime(x, format?) -> datetime -- Parses string to datetime",
		// Env/Secrets
		"ENV":           "ENV(key) -> any -- Returns an environment variable value. Supports dot-path: ENV(\"DB_CONFIG.host\")",
		"SECRET":        "SECRET(key) -> any -- Returns a decrypted secret value. Supports dot-path: SECRET(\"CREDS.password\")",
		"ENV_EXISTS":    "ENV_EXISTS(key) -> bool -- Returns true if the environment variable exists",
		"SECRET_EXISTS": "SECRET_EXISTS(key) -> bool -- Returns true if the secret exists",
		// Platform — namespaced builtins
		"query":                    "query(dataset) -> array -- Keyword form of dataset::query; supports pipe chains",
		"dataset::query":           "dataset::query(dataset, dsl?) -> array -- Queries a dataset",
		"dataset::count":           "dataset::count(dataset, where?) -> int -- Counts matching rows",
		"dataset::schema":          "dataset::schema(name) -> object -- Returns schema definition",
		"dataset::columns":         "dataset::columns(name) -> array -- Returns schema columns",
		"dataset::validate":        "dataset::validate(name, rows) -> object -- Validates rows against a schema",
		"dataset::insert":          "dataset::insert(name, row | rows) -> {inserted: int} -- Inserts row(s) into a dataset",
		"dataset::update":          "dataset::update(name, row | rows) -> {updated: int} -- Updates row(s) by primary key",
		"dataset::delete":          "dataset::delete(name, where) -> {deleted: int} -- Deletes rows matching a predicate (where clause is required)",
		"dataset::upsert":          "dataset::upsert(name, key_columns?, row | rows) -> {affected: int} -- Inserts or updates row(s); key_columns defaults to the dataset's primary key",
		"dataset::get":             "dataset::get(name, id) -> object | null -- Returns a single row by primary identifier, or null when not found",
		"http::get":                "http::get(url, headers?) -> any -- Makes an HTTP GET request",
		"http::post":               "http::post(url, body, headers?) -> any -- Makes an HTTP POST request",
		"pipeline::run":            "pipeline::run(name, input?) -> object -- Runs a pipeline by name",
		"viz::transform":           "viz::transform(data, config) -> object -- Applies a viz transform to data",
		"agent::call":              "agent::call(agent, prompt, context?) -> object -- Calls an agent",
		"event::publish":           "event::publish(topic, payload?) -> null -- Publishes an event to the workspace event bus",
		"identity::current_user":   "identity::current_user() -> object -- Returns the authenticated user (id, app_id, org_id, session_id, auth_method, is_authenticated)",
		"identity::has_permission": "identity::has_permission(action, resource) -> bool -- Checks whether the current user has the given permission",
		"time::now":                "time::now() -> datetime -- Returns the current UTC time",
		"id::uuid":                 "id::uuid() -> string -- Returns a freshly generated UUID v4",
		"id::slug":                 "id::slug(text) -> string -- Normalises text into a URL-safe slug",
	}
	return docs[name]
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

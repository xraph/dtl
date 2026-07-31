// Package slug derives URL-safe identifiers from human-readable names.
//
// This is the deterministic half of a larger slug package, vendored so the
// id::slug builtin keeps working without the language depending on a host.
// The collision-resolving variants need a datastore to ask "does this
// candidate already exist?", which a language runtime must not require —
// id::slug always called Generate with a nil existence check, so only the
// normalization below was ever reachable from DTL.
package slug

import (
	"strings"
	"unicode"
)

// Generate produces a URL-safe slug from name.
//
// Rules:
//   - Lowercase
//   - Letters and digits pass through
//   - Whitespace, dashes, underscores, slashes and dots collapse to a single "-"
//   - Anything else is dropped
//   - Leading/trailing dashes trimmed
//   - Empty result becomes "untitled"
func Generate(name string) string {
	if base := normalize(name); base != "" {
		return base
	}
	return "untitled"
}

func normalize(name string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range name {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteRune(unicode.ToLower(r))
			prevDash = false
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevDash = false
		case r == ' ' || r == '-' || r == '_' || r == '/' || r == '.':
			if !prevDash && b.Len() > 0 {
				b.WriteRune('-')
				prevDash = true
			}
		default:
			// Drop unrecognised runes; the slug stays ASCII.
		}
	}
	return strings.Trim(b.String(), "-")
}

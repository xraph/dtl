package lang

import (
	"sort"
	"strings"
)

// Complete returns the suggestions for a cursor position in source.
//
// Ordering is deliberate and part of the contract: keywords first, then
// functions, then types, then everything else, alphabetically within each
// group. Editors that do their own sorting will override it; those that do not
// get a usable list.
func Complete(source string, position int, ctx Context) []Item {
	items := make([]Item, 0, 64)

	for _, kw := range keywords {
		items = append(items, Item{Label: kw, Kind: KindKeyword, Detail: "keyword", Doc: keywordDoc(kw)})
	}
	for _, typ := range types {
		items = append(items, Item{Label: typ, Kind: KindType, Detail: "type"})
	}

	for _, fn := range ctx.Functions {
		detail := "function"
		if fn.Builtin {
			detail = "builtin"
			// Namespaced builtins are offered through their namespace object,
			// not as bare identifiers, so they are skipped here.
			if strings.Contains(fn.Name, "::") {
				continue
			}
		}
		doc := fn.Doc
		if doc == "" && fn.Builtin {
			doc = builtinDocs(fn.Name)
		}
		items = append(items, Item{
			Label:      fn.Name,
			Kind:       KindFunction,
			Detail:     detail,
			InsertText: fn.Name + "()",
			Doc:        doc,
		})
	}

	for _, g := range ctx.Globals {
		items = append(items, Item{Label: g.Name, Kind: KindVariable, Detail: "global", Doc: g.Doc})
		for _, key := range g.Keys {
			items = append(items, Item{
				Label:  g.Name + "." + key,
				Kind:   KindProperty,
				Detail: g.Name + " key",
			})
		}
	}

	for _, ds := range ctx.Datasets {
		items = append(items, Item{
			Label:      ds,
			Kind:       KindVariable,
			Detail:     "dataset",
			InsertText: `"` + ds + `"`,
		})
	}

	// Narrow to what the cursor is actually typing. Done after collection so
	// every source contributes on equal terms.
	if prefix := extractWordPrefix(source, position); prefix != "" {
		filtered := items[:0]
		for _, it := range items {
			if strings.HasPrefix(strings.ToLower(it.Label), strings.ToLower(prefix)) {
				filtered = append(filtered, it)
			}
		}
		items = filtered
	}

	sort.SliceStable(items, func(i, j int) bool {
		ki, kj := kindOrder(string(items[i].Kind)), kindOrder(string(items[j].Kind))
		if ki != kj {
			return ki < kj
		}
		return items[i].Label < items[j].Label
	})

	return items
}

// Hover returns documentation for the token under the cursor, or nil when the
// cursor is not on something documented.
func Hover(source string, position int, ctx Context) *HoverInfo {
	word := extractWordAtPosition(source, position)
	if word == "" {
		return nil
	}

	// A dotted path resolves against the ambient globals first — `env.FOO`
	// should describe the key, not the word `FOO`.
	if dotted := expandDottedPath(source, position); dotted != "" {
		if doc := globalObjectDoc(source, position, dotted); doc != "" {
			return &HoverInfo{Word: dotted, Doc: doc}
		}
	}

	if doc := keywordDoc(word); doc != "" {
		return &HoverInfo{Word: word, Doc: doc}
	}
	if doc := builtinDocs(word); doc != "" {
		return &HoverInfo{Word: word, Doc: doc}
	}
	for _, fn := range ctx.Functions {
		if fn.Name == word && fn.Doc != "" {
			return &HoverInfo{Word: word, Doc: fn.Doc}
		}
	}
	return nil
}

// HoverInfo is documentation for one token.
type HoverInfo struct {
	Word string `json:"word"`
	Doc  string `json:"documentation"`
}

// Signature reports the parameter count for a callable, so an editor can show
// signature help. ok is false when the name is not callable here.
func Signature(name string, ctx Context) (params int, ok bool) {
	for _, fn := range ctx.Functions {
		if fn.Name == name {
			return fn.Params, true
		}
	}
	return 0, false
}

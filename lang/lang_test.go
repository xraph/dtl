package lang

import (
	"strings"
	"testing"
)

func TestComplete_emptyContextStillOffersTheLanguage(t *testing.T) {
	// An editor that cannot reach a server must still get keywords and types.
	// This is the property that makes the package usable standalone.
	items := Complete("", 0, Context{})

	var kinds = map[Kind]int{}
	for _, it := range items {
		kinds[it.Kind]++
	}
	if kinds[KindKeyword] != len(keywords) {
		t.Errorf("keywords: got %d, want %d", kinds[KindKeyword], len(keywords))
	}
	if kinds[KindType] != len(types) {
		t.Errorf("types: got %d, want %d", kinds[KindType], len(types))
	}
	if kinds[KindFunction] != 0 {
		t.Errorf("an empty context must contribute no functions, got %d", kinds[KindFunction])
	}
}

func TestComplete_ordersKeywordsBeforeFunctionsBeforeTypes(t *testing.T) {
	items := Complete("", 0, Context{
		Functions: []Function{{Name: "aaa_fn", Builtin: true}},
	})

	var seen []Kind
	for _, it := range items {
		if len(seen) == 0 || seen[len(seen)-1] != it.Kind {
			seen = append(seen, it.Kind)
		}
	}
	// aaa_fn sorts before every keyword alphabetically, so if ordering were
	// alphabetical-first it would lead. Kind must win.
	if seen[0] != KindKeyword {
		t.Errorf("keywords must lead regardless of label, got %v", seen)
	}
}

func TestComplete_filtersByCursorPrefix(t *testing.T) {
	src := "fn f() -> int => ret"
	items := Complete(src, len(src), Context{})

	if len(items) == 0 {
		t.Fatal("expected the prefix to match at least `return`")
	}
	for _, it := range items {
		if !strings.HasPrefix(strings.ToLower(it.Label), "ret") {
			t.Errorf("item %q does not match the cursor prefix", it.Label)
		}
	}
}

func TestComplete_namespacedBuiltinsAreNotOfferedBare(t *testing.T) {
	// dataset::query is reached through its namespace, not as a bare
	// identifier, so offering it flat would insert something uncallable.
	items := Complete("", 0, Context{
		Functions: []Function{{Name: "dataset::query", Builtin: true}, {Name: "round", Builtin: true}},
	})

	for _, it := range items {
		if it.Label == "dataset::query" {
			t.Error("namespaced builtin was offered as a bare completion")
		}
	}
	var foundRound bool
	for _, it := range items {
		if it.Label == "round" {
			foundRound = true
		}
	}
	if !foundRound {
		t.Error("un-namespaced builtin should still be offered")
	}
}

func TestComplete_globalKeysBecomeDottedProperties(t *testing.T) {
	items := Complete("", 0, Context{
		Globals: []Global{{Name: "env", Keys: []string{"API_URL"}}},
	})

	var foundObject, foundKey bool
	for _, it := range items {
		switch it.Label {
		case "env":
			foundObject = true
		case "env.API_URL":
			foundKey = true
		}
	}
	if !foundObject || !foundKey {
		t.Errorf("expected both the global and its dotted key; object=%v key=%v", foundObject, foundKey)
	}
}

func TestComplete_datasetsUseTheHostsReferenceConvention(t *testing.T) {
	// How a dataset is written in source is the host's convention. The
	// language must pass it through rather than inventing one, or the
	// completion inserts something that does not parse.
	items := Complete("", 0, Context{Datasets: []Dataset{{
		Name:       "sensor_readings",
		Detail:     "dataset (workspace)",
		InsertText: `query("sensor_readings")`,
	}}})

	for _, it := range items {
		if it.Label == "sensor_readings" {
			if it.InsertText != `query("sensor_readings")` {
				t.Errorf("insert text = %q, want the host's form", it.InsertText)
			}
			if it.Detail != "dataset (workspace)" {
				t.Errorf("detail = %q, want the host's", it.Detail)
			}
			return
		}
	}
	t.Error("dataset was not offered")
}

func TestComplete_datasetDefaultsWhenHostGivesNoConvention(t *testing.T) {
	items := Complete("", 0, Context{Datasets: []Dataset{{Name: "plain"}}})
	for _, it := range items {
		if it.Label == "plain" {
			if it.InsertText != "plain" || it.Detail != "dataset" {
				t.Errorf("defaults wrong: insert=%q detail=%q", it.InsertText, it.Detail)
			}
			return
		}
	}
	t.Error("dataset was not offered")
}

func TestSignature_reportsParamCountForKnownCallables(t *testing.T) {
	ctx := Context{Functions: []Function{{Name: "round", Params: 2, Builtin: true}}}

	if n, ok := Signature("round", ctx); !ok || n != 2 {
		t.Errorf("Signature(round) = (%d, %v), want (2, true)", n, ok)
	}
	if _, ok := Signature("nope", ctx); ok {
		t.Error("unknown callable must report ok=false")
	}
}

func TestHover_returnsNilOffToken(t *testing.T) {
	if h := Hover("   ", 1, Context{}); h != nil {
		t.Errorf("hover on whitespace should be nil, got %+v", h)
	}
}

func TestHover_prefersHostFunctionDocs(t *testing.T) {
	ctx := Context{Functions: []Function{{Name: "my_helper", Doc: "does the thing"}}}

	h := Hover("my_helper", 2, ctx)
	if h == nil || h.Doc != "does the thing" {
		t.Errorf("expected the host-supplied doc, got %+v", h)
	}
}

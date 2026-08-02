package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/xraph/dtl/registry"
	"github.com/xraph/langserver"
	"github.com/xraph/langserver/lsp"
	"github.com/xraph/langserver/stdio"
)

// These drive the server the way an editor does — framed messages in, framed
// messages out — rather than calling handlers directly. A handler that works
// in isolation but is wired to the wrong method name, or answers with a shape
// the protocol does not expect, passes a unit test and fails in an editor.

type conversation struct {
	t    *testing.T
	out  *bytes.Buffer
	srv  *server
	next int
}

func newConversation(t *testing.T) *conversation {
	t.Helper()
	return &conversation{t: t, out: &bytes.Buffer{}, next: 1}
}

// send runs one batch of messages through a fresh Serve loop and returns every
// frame the server wrote. Serve reads to EOF, so each call is one exchange.
func (c *conversation) send(bodies ...string) []map[string]any {
	c.t.Helper()

	var in bytes.Buffer
	for _, b := range bodies {
		if err := stdio.WriteMessage(&in, []byte(b)); err != nil {
			c.t.Fatalf("framing request: %v", err)
		}
	}

	if c.srv == nil {
		c.srv = &server{docs: newDocs(), reg: registry.New(registry.Config{})}
	}
	c.srv.conn = stdio.NewConn(&in, c.out, langserver.NewBaseSession("test", ""))
	if err := c.srv.conn.Serve(c.srv.methods()); err != nil {
		c.t.Fatalf("Serve: %v", err)
	}

	var msgs []map[string]any
	r := bufio.NewReader(c.out)
	for {
		body, err := stdio.ReadMessage(r)
		if err != nil {
			break
		}
		var m map[string]any
		if err := json.Unmarshal(body, &m); err != nil {
			c.t.Fatalf("server wrote invalid JSON: %v", err)
		}
		msgs = append(msgs, m)
	}
	c.out.Reset()
	return msgs
}

func req(id int, method, params string) string {
	if params == "" {
		params = "null"
	}
	return `{"jsonrpc":"2.0","id":` + itoa(id) + `,"method":"` + method + `","params":` + params + `}`
}

func note(method, params string) string {
	return `{"jsonrpc":"2.0","method":"` + method + `","params":` + params + `}`
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}

func TestInitialize_advertisesWhatItImplements(t *testing.T) {
	msgs := newConversation(t).send(req(1, "initialize", "{}"))

	if len(msgs) != 1 {
		t.Fatalf("want one response, got %d", len(msgs))
	}
	caps, ok := msgs[0]["result"].(map[string]any)["capabilities"].(map[string]any)
	if !ok {
		t.Fatalf("no capabilities in %v", msgs[0])
	}
	// Advertising a capability the server does not implement makes an editor
	// send requests that go unanswered, which looks like a hang.
	if caps["hoverProvider"] != true {
		t.Error("hover is implemented but not advertised")
	}
	if _, found := caps["completionProvider"]; !found {
		t.Error("completion is implemented but not advertised")
	}
	if caps["textDocumentSync"] != float64(1) {
		t.Errorf("textDocumentSync = %v, want 1 (full)", caps["textDocumentSync"])
	}
}

func TestDidOpen_publishesDiagnosticsForBrokenSource(t *testing.T) {
	c := newConversation(t)
	msgs := c.send(note("textDocument/didOpen",
		`{"textDocument":{"uri":"file:///a.dtl","text":"fn broken( -> "}}`))

	if len(msgs) == 0 {
		t.Fatal("opening a document with a syntax error published nothing")
	}
	if msgs[0]["method"] != "textDocument/publishDiagnostics" {
		t.Fatalf("expected a diagnostics notification, got %v", msgs[0]["method"])
	}
	diags := msgs[0]["params"].(map[string]any)["diagnostics"].([]any)
	if len(diags) == 0 {
		t.Error("broken source produced no diagnostics")
	}
}

func TestDidOpen_validSourcePublishesAnEmptyList(t *testing.T) {
	// An empty list is not the same as sending nothing: it is what clears a
	// stale error from the editor's gutter after a fix.
	c := newConversation(t)
	msgs := c.send(note("textDocument/didOpen",
		`{"textDocument":{"uri":"file:///ok.dtl","text":"fn f() -> int => 1"}}`))

	if len(msgs) == 0 {
		t.Fatal("nothing published — a fixed document would keep its old errors")
	}
	diags := msgs[0]["params"].(map[string]any)["diagnostics"].([]any)
	if len(diags) != 0 {
		t.Errorf("valid source produced diagnostics: %v", diags)
	}
}

func TestCompletion_offersTheLanguageOffline(t *testing.T) {
	c := newConversation(t)
	msgs := c.send(
		note("textDocument/didOpen", `{"textDocument":{"uri":"file:///a.dtl","text":"fn f() -> int => re"}}`),
		req(2, "textDocument/completion",
			`{"textDocument":{"uri":"file:///a.dtl"},"position":{"line":0,"character":19}}`),
	)

	var items []any
	for _, m := range msgs {
		if res, ok := m["result"].(map[string]any); ok {
			items, _ = res["items"].([]any)
		}
	}
	if len(items) == 0 {
		t.Fatal("no completions with no platform attached — the offline case is the point")
	}
	var sawReturn bool
	for _, it := range items {
		if it.(map[string]any)["label"] == "return" {
			sawReturn = true
		}
	}
	if !sawReturn {
		t.Errorf("prefix `re` did not offer `return`; got %d items", len(items))
	}
}

// The UTF-16 conversion is the thing most likely to be wrong, and it is
// invisible in ASCII. This puts the cursor after an emoji and checks the
// server resolves the same prefix it would without one.
func TestCompletion_positionIsCorrectAfterAnEmoji(t *testing.T) {
	src := `fn f() -> string => "🌍" ret`

	// Derive the position rather than hand-counting code units. Counting them
	// by eye is exactly the error this test exists to catch, and doing it in
	// the test just moves the mistake. The cursor goes at end-of-document.
	pos := lsp.PositionFromOffset(src, len(src))
	if pos.Character == len([]rune(src)) {
		t.Fatal("character equals the rune count — the surrogate pair was not counted twice")
	}

	c := newConversation(t)
	msgs := c.send(
		note("textDocument/didOpen", `{"textDocument":{"uri":"file:///e.dtl","text":`+jsonString(src)+`}}`),
		req(2, "textDocument/completion",
			`{"textDocument":{"uri":"file:///e.dtl"},"position":{"line":0,"character":`+itoa(pos.Character)+`}}`),
	)

	var items []any
	for _, m := range msgs {
		if res, ok := m["result"].(map[string]any); ok {
			items, _ = res["items"].([]any)
		}
	}
	if len(items) == 0 {
		t.Fatal("no completions after an emoji — the position mapping is wrong")
	}
	for _, it := range items {
		label, _ := it.(map[string]any)["label"].(string)
		if !strings.HasPrefix(strings.ToLower(label), "ret") {
			t.Fatalf("cursor resolved to the wrong token: got %q, expected a `ret` prefix", label)
		}
	}
}

func TestCompletion_onAnUnopenedDocumentIsAnError(t *testing.T) {
	msgs := newConversation(t).send(req(1, "textDocument/completion",
		`{"textDocument":{"uri":"file:///never-opened.dtl"},"position":{"line":0,"character":0}}`))

	if len(msgs) != 1 {
		t.Fatalf("want one response, got %d", len(msgs))
	}
	if _, hasErr := msgs[0]["error"]; !hasErr {
		t.Error("a request for an unknown document should report an error, not empty results")
	}
}

func TestDidClose_clearsDiagnostics(t *testing.T) {
	c := newConversation(t)
	c.send(note("textDocument/didOpen",
		`{"textDocument":{"uri":"file:///a.dtl","text":"fn broken( -> "}}`))

	msgs := c.send(note("textDocument/didClose", `{"textDocument":{"uri":"file:///a.dtl"}}`))
	if len(msgs) == 0 {
		t.Fatal("closing published nothing — the errors would stay in the gutter")
	}
	diags := msgs[0]["params"].(map[string]any)["diagnostics"].([]any)
	if len(diags) != 0 {
		t.Errorf("close should clear diagnostics, got %v", diags)
	}
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

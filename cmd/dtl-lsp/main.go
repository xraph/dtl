// Command dtl-lsp is a Language Server Protocol server for DTL.
//
// It speaks LSP over stdin and stdout, which is how editors launch a language
// server, and answers from the language's own packages: completions and hover
// from dtl/lang, diagnostics from the compiler in dtl/registry.
//
// It holds no connection to any platform. Everything it knows about a document
// arrives from the editor, and everything it knows about the language is in the
// language. That is what makes it work for a file on disk with no server
// running anywhere — the case a hosted completion endpoint cannot serve.
//
//	go install github.com/xraph/dtl/cmd/dtl-lsp@latest
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/xraph/dtl/lang"
	"github.com/xraph/dtl/registry"
	"github.com/xraph/langserver"
	"github.com/xraph/langserver/lsp"
	"github.com/xraph/langserver/stdio"
)

type server struct {
	docs *docs
	reg  *registry.Registry
	conn *stdio.Conn
}

func main() {
	s := &server{
		docs: newDocs(),
		// A registry with the standard library loaded and no database behind
		// it. That is the whole point of an offline server: it can compile and
		// diagnose without anything else running.
		reg: registry.New(registry.Config{}),
	}
	s.conn = stdio.NewConn(os.Stdin, os.Stdout, langserver.NewBaseSession("dtl-lsp", ""))

	if err := s.conn.Serve(s.methods()); err != nil {
		fmt.Fprintf(os.Stderr, "dtl-lsp: %v\n", err)
		os.Exit(1)
	}
}

func (s *server) methods() map[string]langserver.Handler {
	return map[string]langserver.Handler{
		"initialize":              s.initialize,
		"initialized":             noop,
		"shutdown":                func(langserver.Session, json.RawMessage) (any, *langserver.RPCError) { return nil, nil },
		"exit":                    noop,
		"textDocument/didOpen":    s.didOpen,
		"textDocument/didChange":  s.didChange,
		"textDocument/didClose":   s.didClose,
		"textDocument/completion": s.completion,
		"textDocument/hover":      s.hover,
	}
}

func noop(langserver.Session, json.RawMessage) (any, *langserver.RPCError) { return nil, nil }

// --- lifecycle ---

func (s *server) initialize(_ langserver.Session, _ json.RawMessage) (any, *langserver.RPCError) {
	return map[string]any{
		"capabilities": map[string]any{
			// 1 = full document sync. DTL functions are small and incremental
			// sync buys nothing but a class of state-drift bugs.
			"textDocumentSync": 1,
			"completionProvider": map[string]any{
				"triggerCharacters": []string{".", ":"},
			},
			"hoverProvider": true,
		},
		"serverInfo": map[string]any{"name": "dtl-lsp"},
	}, nil
}

// --- document sync ---

type didOpenParams struct {
	TextDocument struct {
		URI  string `json:"uri"`
		Text string `json:"text"`
	} `json:"textDocument"`
}

func (s *server) didOpen(_ langserver.Session, raw json.RawMessage) (any, *langserver.RPCError) {
	var p didOpenParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, badParams(err)
	}
	s.docs.set(p.TextDocument.URI, p.TextDocument.Text)
	s.publishDiagnostics(p.TextDocument.URI, p.TextDocument.Text)
	return nil, nil
}

type didChangeParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	ContentChanges []struct {
		Text string `json:"text"`
	} `json:"contentChanges"`
}

func (s *server) didChange(_ langserver.Session, raw json.RawMessage) (any, *langserver.RPCError) {
	var p didChangeParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, badParams(err)
	}
	if len(p.ContentChanges) == 0 {
		return nil, nil
	}
	// Full sync: the last change carries the whole document.
	text := p.ContentChanges[len(p.ContentChanges)-1].Text
	s.docs.set(p.TextDocument.URI, text)
	s.publishDiagnostics(p.TextDocument.URI, text)
	return nil, nil
}

func (s *server) didClose(_ langserver.Session, raw json.RawMessage) (any, *langserver.RPCError) {
	var p didOpenParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, badParams(err)
	}
	s.docs.remove(p.TextDocument.URI)
	// Clear the editor's gutter: diagnostics persist until replaced.
	_ = s.conn.Notify("textDocument/publishDiagnostics", map[string]any{
		"uri": p.TextDocument.URI, "diagnostics": []any{},
	})
	return nil, nil
}

// --- features ---

type positionParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	Position lsp.Position `json:"position"`
}

func (s *server) completion(_ langserver.Session, raw json.RawMessage) (any, *langserver.RPCError) {
	text, pos, rpcErr := s.locate(raw)
	if rpcErr != nil {
		return nil, rpcErr
	}

	items := lang.Complete(text, pos, s.langContext())

	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		entry := map[string]any{"label": it.Label, "kind": lspKind(it.Kind)}
		if it.Detail != "" {
			entry["detail"] = it.Detail
		}
		if it.Doc != "" {
			entry["documentation"] = it.Doc
		}
		if it.InsertText != "" {
			entry["insertText"] = it.InsertText
		}
		out = append(out, entry)
	}
	return map[string]any{"isIncomplete": false, "items": out}, nil
}

func (s *server) hover(_ langserver.Session, raw json.RawMessage) (any, *langserver.RPCError) {
	text, pos, rpcErr := s.locate(raw)
	if rpcErr != nil {
		return nil, rpcErr
	}

	info := lang.Hover(text, pos, s.langContext())
	if info == nil {
		// null, not an empty hover — editors render an empty box for the latter.
		return nil, nil
	}
	return map[string]any{
		"contents": map[string]any{"kind": "markdown", "value": info.Doc},
	}, nil
}

// locate resolves a position request to document text and a byte offset.
//
// This is where LSP's UTF-16 character index becomes a Go byte offset; getting
// it wrong puts every completion on the neighbouring token in any document
// containing an emoji.
func (s *server) locate(raw json.RawMessage) (string, int, *langserver.RPCError) {
	var p positionParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", 0, badParams(err)
	}
	text, ok := s.docs.get(p.TextDocument.URI)
	if !ok {
		return "", 0, &langserver.RPCError{
			Code:    -32602,
			Message: "document is not open: " + p.TextDocument.URI,
		}
	}
	return text, lsp.OffsetFromPosition(text, p.Position), nil
}

// langContext tells the language what this server knows exists.
//
// Offline, that is the standard library and nothing else — there is no
// platform to ask about datasets or workspace functions. An empty-ish context
// still yields keywords, types and builtins, which is exactly the point.
func (s *server) langContext() lang.Context {
	names := s.reg.ListFunctionNames()
	fns := make([]lang.Function, 0, len(names))
	for _, n := range names {
		params, _ := s.reg.ResolveFunction(n)
		fns = append(fns, lang.Function{Name: n, Params: params, Builtin: true})
	}
	return lang.Context{Functions: fns}
}

// publishDiagnostics compiles the document and pushes the result.
//
// Diagnostics are a notification, not a response: the editor never asks for
// them, so they are sent whenever the text changes.
func (s *server) publishDiagnostics(uri, text string) {
	result := s.reg.Validate(text)

	diags := make([]map[string]any, 0, len(result.Errors))
	for _, e := range result.Errors {
		severity := 1 // error
		if e.Code == "match_not_exhaustive" {
			severity = 2 // warning
		}
		// LSP lines and characters are zero-based; the compiler counts from 1.
		line := e.Line - 1
		if line < 0 {
			line = 0
		}
		col := e.Column - 1
		if col < 0 {
			col = 0
		}
		diags = append(diags, map[string]any{
			"range": map[string]any{
				"start": map[string]any{"line": line, "character": col},
				"end":   map[string]any{"line": line, "character": col + 1},
			},
			"severity": severity,
			"code":     e.Code,
			"source":   "dtl",
			"message":  e.Message,
		})
	}

	_ = s.conn.Notify("textDocument/publishDiagnostics", map[string]any{
		"uri": uri, "diagnostics": diags,
	})
}

// lspKind maps the language's completion kinds onto LSP's numeric enum.
func lspKind(k lang.Kind) int {
	switch k {
	case lang.KindKeyword:
		return 14
	case lang.KindFunction:
		return 3
	case lang.KindType:
		return 7 // class — LSP has no "type" kind
	case lang.KindVariable:
		return 6
	case lang.KindProperty:
		return 10
	default:
		return 1 // text
	}
}

func badParams(err error) *langserver.RPCError {
	return &langserver.RPCError{Code: -32602, Message: "invalid params: " + err.Error()}
}

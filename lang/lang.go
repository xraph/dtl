// Package lang provides editor intelligence for DTL — completions,
// diagnostics, hover and signature help.
//
// Nothing here serves anything. Every entry point is a pure function: text and
// a cursor in, structured results out. Transport is the caller's business, so
// the same functions back an HTTP handler, a WebSocket session, an SSE stream
// or an LSP binary without knowing which is which.
//
// What the host knows and the language does not — which functions are
// registered, which datasets exist, which ambient values are in scope — is
// passed per call in a Context rather than read from a global. One process can
// therefore serve several workspaces without them seeing each other's names.
package lang

// Kind classifies a completion item for an editor's icon and ranking.
type Kind string

const (
	KindKeyword  Kind = "keyword"
	KindType     Kind = "type"
	KindFunction Kind = "function"
	KindVariable Kind = "variable"
	KindProperty Kind = "property"
)

// Item is one completion suggestion.
type Item struct {
	Label      string `json:"label"`
	Kind       Kind   `json:"kind"`
	Detail     string `json:"detail,omitempty"`
	InsertText string `json:"insertText,omitempty"`
	Doc        string `json:"documentation,omitempty"`
}

// Function is a callable the host has registered, builtin or user-defined.
type Function struct {
	Name string
	// Params is the parameter count, used to build the insert text.
	Params int
	// Builtin distinguishes language builtins from user-defined functions,
	// which rank differently and carry different detail text.
	Builtin bool
	Doc     string
}

// Global is an ambient object available to every program, and optionally the
// keys it exposes — environment names, secret names.
type Global struct {
	Name string
	Doc  string
	Keys []string
}

// Dataset is a named data source the host offers as a completion. Detail and
// InsertText are the host's, because how a dataset is referenced in source is
// the host's convention, not the language's — one platform writes
// query("name"), another writes a bare identifier.
type Dataset struct {
	Name       string
	Detail     string
	InsertText string
}

// Context carries host knowledge into a completion or hover request. Every
// field is optional: an empty Context yields language-only results — keywords,
// types and whatever the caller passed — which is exactly what an editor with
// no server reachable should still get.
type Context struct {
	Functions []Function
	Datasets  []Dataset
	Globals   []Global
}

// keywords and types are the language's own vocabulary, so they live here
// rather than arriving in a Context.
var keywords = []string{
	"fn", "let", "return", "if", "then", "else", "match", "when",
	"try", "catch", "and", "or", "not", "for", "in", "raise", "use",
	"true", "false", "null",
}

var types = []string{"int", "float", "string", "bool", "any", "object", "datetime"}

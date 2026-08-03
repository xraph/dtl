package lang

import (
	"sync"

	"github.com/xraph/dtl/executor"
	"github.com/xraph/dtl/stdlib"
)

// builtinDocs returns hover documentation for a builtin function.
//
// Standard library entries come from the library itself: every stdlib
// registration carries a Doc string, so this reads the real registration table
// rather than a transcription of it. That is deliberate — the hand-maintained
// table this replaced had drifted into documenting functions the language did
// not have while omitting dozens it did, and a copy can always drift again.
//
// A host's own builtins document themselves the same way, through the Doc
// field on the BuiltinFunc it registers, reaching this package as
// Function.Doc on the Context. Only the language-level ambient accessors below
// stay written out here, because no stdlib registration owns them.
func builtinDocs(name string) string {
	if doc, ok := stdlibDocs()[name]; ok {
		return doc
	}
	return ambientDocs[name]
}

// stdlibDocs builds the name-to-doc index once, from a throwaway registration
// of the standard library.
var stdlibDocs = sync.OnceValue(func() map[string]string {
	builtins := make(map[string]*executor.BuiltinFunc)
	stdlib.RegisterAll(builtins)

	docs := make(map[string]string, len(builtins))
	for name, b := range builtins {
		if b.Doc != "" {
			docs[name] = b.Doc
		}
	}
	return docs
})

// ambientDocs documents the ambient-value accessors. These are not stdlib
// registrations — a host supplies them alongside the global.env and
// global.secrets objects — but this package already offers dotted-path hover
// for those objects, so it documents the call form to match.
//
// #nosec G101 -- help text describing the secret accessors, not credentials.
var ambientDocs = map[string]string{
	"ENV":           `ENV(key) -> any -- Returns an environment variable value. Supports dot-paths: ENV("DB_CONFIG.host")`,
	"SECRET":        `SECRET(key) -> any -- Returns a decrypted secret value. Supports dot-paths: SECRET("CREDS.password")`,
	"ENV_EXISTS":    "ENV_EXISTS(key) -> bool -- Whether the environment variable exists",
	"SECRET_EXISTS": "SECRET_EXISTS(key) -> bool -- Whether the secret exists",
}

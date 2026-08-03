package stdlib

import (
	"strings"

	"github.com/xraph/dtl/executor"
)

// RegisterAll registers all standard library functions into the builtins map.
func RegisterAll(builtins map[string]*executor.BuiltinFunc) {
	registerCore(builtins)
	registerMath(builtins)
	registerText(builtins)
	registerTextCase(builtins)
	registerDatetime(builtins)
	registerStats(builtins)
	registerCollections(builtins)
	registerCasting(builtins)
	registerFormatting(builtins)
	registerObjects(builtins)
	registerPath(builtins)
	registerRegex(builtins)
	registerNamespaceHelpers(builtins)
}

// register adds a BuiltinFunc to the map. doc is mandatory: it is what the
// language server shows on hover and completion, and requiring it here is what
// keeps the documentation from drifting away from the implementation.
// Conventional shape: "name(args) -> type -- what it does".
func register(m map[string]*executor.BuiltinFunc, name string, minArgs, maxArgs int, fn func(args []any) (any, error), doc string) {
	m[name] = &executor.BuiltinFunc{Name: name, MinArgs: minArgs, MaxArgs: maxArgs, Fn: fn, Doc: doc}
}

// alias registers an already-registered builtin under an additional name,
// sharing its implementation and its documentation. Used for the legacy
// system::* namespace spellings, which must never describe themselves
// differently from the bare name they mirror.
func alias(m map[string]*executor.BuiltinFunc, name, existing string) {
	b, ok := m[existing]
	if !ok {
		panic("stdlib: alias " + name + " refers to unregistered builtin " + existing)
	}
	clone := *b
	clone.Name = name
	// Rewrite the leading signature name so hover on the alias shows the alias.
	// Derived from the target's doc rather than written out again, so the two
	// descriptions cannot say different things.
	clone.Doc = name + strings.TrimPrefix(b.Doc, existing)
	m[name] = &clone
}

package stdlib

import (
	"strings"
	"sync"

	"github.com/xraph/dtl/executor"
)

var (
	sharedOnce sync.Once
	sharedTbl  map[string]*executor.BuiltinFunc
)

// Shared returns the standard library's builtin table, built once per process
// and reused by every registry.
//
// Building it costs ~22us and several hundred allocations, and a registry that
// rebuilt it paid that on construction. Nothing about a BuiltinFunc is
// per-registry — they close over no registry state — so there is no reason for
// each one to own a copy. A host that creates a registry per tenant or per
// request was paying this repeatedly for an identical result.
//
// The returned map MUST NOT be mutated: it is shared by every registry in the
// process, and a write here would be visible to all of them (and would race
// with concurrent lookups). Registry keeps host-registered builtins in a
// separate overlay for exactly this reason — see Registry.RegisterBuiltin.
func Shared() map[string]*executor.BuiltinFunc {
	sharedOnce.Do(func() {
		tbl := make(map[string]*executor.BuiltinFunc, 512)
		RegisterAll(tbl)
		sharedTbl = tbl
	})
	return sharedTbl
}

// RegisterAll registers all standard library functions into the builtins map.
//
// Prefer Shared unless you need a table you own and intend to modify.
func RegisterAll(builtins map[string]*executor.BuiltinFunc) {
	registerCore(builtins)
	registerMath(builtins)
	registerMathExtra(builtins)
	registerText(builtins)
	registerTextCase(builtins)
	registerDatetime(builtins)
	registerDatetimeExtra(builtins)
	registerStats(builtins)
	registerStatsExtra(builtins)
	registerCollections(builtins)
	registerCollectionsExtra(builtins)
	registerCasting(builtins)
	registerFormatting(builtins)
	registerObjects(builtins)
	registerPath(builtins)
	registerRegex(builtins)
	registerJSON(builtins)
	registerEncoding(builtins)
	registerHash(builtins)
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

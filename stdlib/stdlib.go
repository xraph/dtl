package stdlib

import (
	"github.com/xraph/dtl/executor"
)

// RegisterAll registers all standard library functions into the builtins map.
func RegisterAll(builtins map[string]*executor.BuiltinFunc) {
	registerCore(builtins)
	registerMath(builtins)
	registerText(builtins)
	registerDatetime(builtins)
	registerStats(builtins)
	registerCollections(builtins)
	registerCasting(builtins)
	registerFormatting(builtins)
	registerObjects(builtins)
	registerNamespaceHelpers(builtins)
}

// register is a convenience for adding a BuiltinFunc to the map.
func register(m map[string]*executor.BuiltinFunc, name string, minArgs, maxArgs int, fn func(args []any) (any, error)) {
	m[name] = &executor.BuiltinFunc{Name: name, MinArgs: minArgs, MaxArgs: maxArgs, Fn: fn}
}

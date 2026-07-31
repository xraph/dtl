package stdlib

import "context"

// Global variables are values an embedder makes available to every DTL program
// in an execution — configuration, environment, secrets. They ride in the
// context rather than in the function's scope because they are ambient to the
// execution, not arguments to any one call.
//
// These helpers were originally declared alongside a set of host-specific
// builtins; they moved here when those builtins left the language, because
// "an execution carries ambient values" is a property of the language, not of
// any particular host.

// globalVarsKey is the context key for pre-built global variables.
type globalVarsKey struct{}

// WithGlobalVars attaches pre-built global variables to a context.
func WithGlobalVars(ctx context.Context, globals map[string]any) context.Context {
	return context.WithValue(ctx, globalVarsKey{}, globals)
}

// GlobalVarsFrom extracts global variables from a context. Returns nil if not set.
func GlobalVarsFrom(ctx context.Context) map[string]any {
	if gv, ok := ctx.Value(globalVarsKey{}).(map[string]any); ok {
		return gv
	}
	return nil
}

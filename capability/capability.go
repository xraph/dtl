// Package capability lets an embedder narrow what DTL code may do as
// execution crosses into a user-defined function.
//
// DTL itself has no notion of who owns a function or what it is allowed to
// touch — that is the embedder's model. So this package carries no policy: it
// provides the hook the executor calls, and the embedder supplies the meaning.
//
// Absence means unrestricted, deliberately. An embedder that installs no
// interceptor gets a plain interpreter.
package capability

import "context"

// Interceptor re-scopes ctx as execution enters functionName. Returning ctx
// unchanged leaves the caller's authority in place.
type Interceptor func(ctx context.Context, functionName string) context.Context

type interceptorKey struct{}

// WithInterceptor attaches an Interceptor to ctx. A nil Interceptor attaches
// nothing, so callers need not branch.
func WithInterceptor(ctx context.Context, fn Interceptor) context.Context {
	if fn == nil {
		return ctx
	}
	return context.WithValue(ctx, interceptorKey{}, fn)
}

// From returns the Interceptor in ctx, or nil.
func From(ctx context.Context) Interceptor {
	fn, _ := ctx.Value(interceptorKey{}).(Interceptor)
	return fn
}

// Enter is called by the executor immediately before running a user function.
// With no interceptor installed the gate is inert.
func Enter(ctx context.Context, functionName string) context.Context {
	if fn := From(ctx); fn != nil {
		return fn(ctx, functionName)
	}
	return ctx
}

package stdlib

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/xraph/dtl/executor"
	"github.com/xraph/dtl/internal/slug"
)

// registerNamespaceHelpers registers small, pure host functions under their
// namespaced names. These have no PlatformServices dependency: they're host
// helpers that compose well with the dataset / event / identity surfaces.
func registerNamespaceHelpers(m map[string]*executor.BuiltinFunc) {
	register(m, "time::now", 0, 0, fnTimeNow)
	register(m, "id::uuid", 0, 0, fnIDUUID)
	register(m, "id::slug", 1, 1, fnIDSlug)
}

func fnTimeNow(_ []any) (any, error) {
	return time.Now().UTC(), nil
}

func fnIDUUID(_ []any) (any, error) {
	return uuid.New().String(), nil
}

func fnIDSlug(args []any) (any, error) {
	s, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("id::slug: expected string argument, got %T", args[0])
	}
	// Pure deterministic normalization — no collision check, which is what
	// this builtin always did (it passed a nil existence function).
	return slug.Generate(s), nil
}

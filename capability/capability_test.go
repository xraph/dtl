package capability_test

import (
	"context"
	"testing"

	"github.com/xraph/dtl/capability"
)

type markerKey struct{}

func TestEnter_noInterceptor_returnsContextUnchanged(t *testing.T) {
	ctx := context.Background()
	if got := capability.Enter(ctx, "app:com.acme.pack::do_it"); got != ctx {
		t.Fatalf("Enter with no interceptor must return ctx unchanged")
	}
}

func TestEnter_invokesInterceptorWithFunctionName(t *testing.T) {
	var seen string
	ctx := capability.WithInterceptor(context.Background(),
		func(ctx context.Context, fn string) context.Context {
			seen = fn
			return context.WithValue(ctx, markerKey{}, "scoped")
		})

	got := capability.Enter(ctx, "app:com.acme.pack::do_it")

	if seen != "app:com.acme.pack::do_it" {
		t.Fatalf("interceptor got %q, want the function name", seen)
	}
	if got.Value(markerKey{}) != "scoped" {
		t.Fatalf("Enter must return the interceptor's context")
	}
}

func TestWithInterceptor_nilIsInert(t *testing.T) {
	ctx := context.Background()
	if got := capability.WithInterceptor(ctx, nil); got != ctx {
		t.Fatalf("nil interceptor must not modify ctx")
	}
}

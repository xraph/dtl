package registry

import (
	"context"
	"reflect"
	"testing"

	"github.com/xraph/dtl/executor"
)

type tenantKey struct{}

// A lambda runs in its caller's context. A host builtin called inside one,
// such as a dataset query reading the host's services from the context,
// used to get an empty context from map, filter and the rest, and failed.
func TestLambdasRunInTheCallersContext(t *testing.T) {
	r := newTestRegistry()
	r.RegisterBuiltin("tenant", &executor.BuiltinFunc{Name: "tenant", MinArgs: 0, MaxArgs: 0,
		CtxFn: func(ctx context.Context, _ []any) (any, error) {
			tenant, _ := ctx.Value(tenantKey{}).(string)
			return tenant, nil
		}})
	ctx := context.WithValue(context.Background(), tenantKey{}, "acme")
	for src, want := range map[string]any{
		`fn f(xs: any) -> any => map(xs, x => tenant())`:                        []any{"acme", "acme"},
		`fn f(xs: any) -> any => filter(xs, x => tenant() == "acme")`:           []any{int64(1), int64(2)},
		`fn f(xs: any) -> any => reduce(xs, "", (acc, x) => tenant())`:          "acme",
		`fn f(xs: any) -> any => map_values({a: 1}, v => tenant())`:             map[string]any{"a": "acme"},
		`fn f(xs: any) -> any => max_by(map(xs, x => {t: tenant()}), o => 1).t`: "acme",
	} {
		got, err := r.ExecuteInline(ctx, src, map[string]any{"xs": []any{int64(1), int64(2)}})
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		if !reflect.DeepEqual(got.Value, want) {
			t.Errorf("%s = %#v, want %#v", src, got.Value, want)
		}
	}
}

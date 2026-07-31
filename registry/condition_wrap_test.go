package registry

import (
	"context"
	"testing"
	"time"
)

// TestConditionWrapFormat documents and locks the DTL source format used by the
// page extension's server-side conditional-visibility evaluator
// (buildExprEvaluator/wrapCondition): a `fn _cond(<env keys>: any, ...) -> any`
// declaration whose body is the authored expression, evaluated with env as the
// argument map. If this format ever stops compiling/evaluating, server-side
// page gating silently fails open — so guard it here against the real engine.
func TestConditionWrapFormat(t *testing.T) {
	r := New(Config{DefaultTimeout: 5 * time.Second, MaxCallDepth: 100})
	ctx := context.Background()

	env := map[string]any{
		"roles":            []any{"admin", "member"},
		"user":             map[string]any{"id": "u1", "authenticated": true},
		"workspace":        map[string]any{"id": "ws1"},
		"is_authenticated": true,
	}

	cases := []struct {
		name string
		src  string
		want bool
	}{
		{
			name: "contains roles admin (true)",
			src:  `fn _cond(is_authenticated: any, roles: any, user: any, workspace: any) -> any => contains(roles, "admin")`,
			want: true,
		},
		{
			name: "contains roles owner (false)",
			src:  `fn _cond(is_authenticated: any, roles: any, user: any, workspace: any) -> any => contains(roles, "owner")`,
			want: false,
		},
		{
			name: "member access user.authenticated (true)",
			src:  `fn _cond(is_authenticated: any, roles: any, user: any, workspace: any) -> any => user.authenticated == true`,
			want: true,
		},
		{
			name: "workspace id match (true)",
			src:  `fn _cond(is_authenticated: any, roles: any, user: any, workspace: any) -> any => workspace.id == "ws1"`,
			want: true,
		},
		{
			name: "flat alias is_authenticated (true)",
			src:  `fn _cond(is_authenticated: any, roles: any, user: any, workspace: any) -> any => is_authenticated == true`,
			want: true,
		},
		{
			name: "is_blank builtin (true)",
			src:  `fn _cond(is_authenticated: any, roles: any, user: any, workspace: any) -> any => is_blank("")`,
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := r.ExecuteInline(ctx, tc.src, env)
			if err != nil {
				t.Fatalf("ExecuteInline error: %v", err)
			}
			got, ok := res.GetValue().(bool)
			if !ok {
				t.Fatalf("expected bool result, got %T (%v)", res.GetValue(), res.GetValue())
			}
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

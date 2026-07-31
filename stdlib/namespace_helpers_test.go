package stdlib

import (
	"strings"
	"testing"
	"time"
)

func TestFnTimeNow_returnsUTC(t *testing.T) {
	out, err := fnTimeNow(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tm, ok := out.(time.Time)
	if !ok {
		t.Fatalf("expected time.Time, got %T", out)
	}
	if tm.Location() != time.UTC {
		t.Errorf("expected UTC, got %v", tm.Location())
	}
}

func TestFnIDUUID_returnsValidUUID(t *testing.T) {
	out, err := fnIDUUID(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := out.(string)
	if !ok {
		t.Fatalf("expected string, got %T", out)
	}
	// UUID v4: 36 chars, hyphenated, e.g. xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx.
	if len(s) != 36 || strings.Count(s, "-") != 4 {
		t.Errorf("not a UUID: %q", s)
	}
}

func TestFnIDUUID_isUnique(t *testing.T) {
	a, _ := fnIDUUID(nil)
	b, _ := fnIDUUID(nil)
	if a == b {
		t.Error("two consecutive id::uuid calls returned identical values")
	}
}

func TestFnIDSlug_normalizes(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Hello World", "hello-world"},
		{"  Multiple   Spaces  ", "multiple-spaces"},
		{"Trailing-Dash-", "trailing-dash"},
		{"Numbers 123", "numbers-123"},
		{"Mixed_Case Words", "mixed-case-words"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			out, err := fnIDSlug([]any{tc.in})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out != tc.want {
				t.Errorf("got %q, want %q", out, tc.want)
			}
		})
	}
}

func TestFnIDSlug_emptyFallsBackToUntitled(t *testing.T) {
	out, err := fnIDSlug([]any{""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "untitled" {
		t.Errorf("expected fallback 'untitled', got %q", out)
	}
}

func TestFnIDSlug_rejectsNonString(t *testing.T) {
	if _, err := fnIDSlug([]any{42}); err == nil {
		t.Fatal("expected error for non-string argument")
	}
}

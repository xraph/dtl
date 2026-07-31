package stdlib

import "testing"

// range exists because DTL's only iteration constructs — for..in, map, filter
// — all need an array to walk, and a function that has to visit each day in a
// horizon has no array to start from. Without it, "iterate N times" is not
// expressible, which is what deferred calendar's next_working_time on the
// false premise that DTL cannot loop at all.

func TestRange_CountsFromZero(t *testing.T) {
	got, _ := callBuiltin(t, setup(), "range", 4).([]any)
	if len(got) != 4 {
		t.Fatalf("expected 4 elements, got %#v", got)
	}
	for i, v := range got {
		if n, ok := v.(int64); !ok || n != int64(i) {
			t.Errorf("element %d: expected %d, got %#v", i, i, v)
		}
	}
}

func TestRange_TwoArgsIsHalfOpen(t *testing.T) {
	got, _ := callBuiltin(t, setup(), "range", 2, 5).([]any)
	if len(got) != 3 {
		t.Fatalf("expected [2 3 4], got %#v", got)
	}
	if got[0] != int64(2) || got[2] != int64(4) {
		t.Errorf("expected the end excluded, got %#v", got)
	}
}

// An empty range is the natural answer for "no days to walk", and it keeps
// callers from having to special-case a zero horizon.
func TestRange_EmptyWhenNothingToCount(t *testing.T) {
	for _, args := range [][]any{{0}, {-3}, {5, 5}, {5, 2}} {
		got, _ := callBuiltin(t, setup(), "range", args...).([]any)
		if len(got) != 0 {
			t.Errorf("range%v: expected empty, got %#v", args, got)
		}
	}
}

// A runaway range would allocate until the process died, and the DTL executor's
// depth and timeout limits do not cover a single builtin call.
func TestRange_RefusesToAllocateAnUnboundedSequence(t *testing.T) {
	if err := callBuiltinError(t, setup(), "range", 10_000_001); err == nil {
		t.Error("expected an oversized range to be refused")
	}
}

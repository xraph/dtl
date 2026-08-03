package stdlib

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSliceHandlesNegativeIndices(t *testing.T) {
	arr := []any{"a", "b", "c", "d", "e"}

	tests := []struct {
		name string
		args []any
		want []any
	}{
		{"from start", []any{arr, 1}, []any{"b", "c", "d", "e"}},
		{"bounded", []any{arr, 1, 3}, []any{"b", "c"}},
		{"negative start counts from the end", []any{arr, -2}, []any{"d", "e"}},
		{"negative end", []any{arr, 0, -1}, []any{"a", "b", "c", "d"}},
		{"both negative", []any{arr, -3, -1}, []any{"c", "d"}},
		{"start past end yields empty", []any{arr, 3, 1}, []any{}},
		{"beyond length clamps", []any{arr, 0, 99}, []any{"a", "b", "c", "d", "e"}},
		{"negative beyond length clamps", []any{arr, -99}, []any{"a", "b", "c", "d", "e"}},
		{"empty range", []any{arr, 2, 2}, []any{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fnSlice(tt.args)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

// slice must not alias its input, or writing through the result would corrupt
// the original array.
func TestSliceDoesNotAliasItsInput(t *testing.T) {
	original := []any{"a", "b", "c"}
	got, err := fnSlice([]any{original, 0, 2})
	if err != nil {
		t.Fatal(err)
	}
	out, _ := got.([]any)
	out[0] = "changed"

	if original[0] != "a" {
		t.Errorf("input was aliased: original[0] is now %v", original[0])
	}
}

func TestSetOperations(t *testing.T) {
	a := []any{int64(1), int64(2), int64(2), int64(3)}
	b := []any{int64(2), int64(3), int64(4)}

	tests := []struct {
		name string
		fn   func([]any) (any, error)
		want []any
	}{
		{"intersection dedupes and follows a's order", fnIntersection, []any{int64(2), int64(3)}},
		{"union is first-seen order", fnUnion, []any{int64(1), int64(2), int64(3), int64(4)}},
		{"difference", fnDifference, []any{int64(1)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fn([]any{a, b})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

// Set membership compares type as well as value, so the int 1 and the string
// "1" are different elements rather than accidentally equal.
func TestSetOperationsDistinguishTypes(t *testing.T) {
	got, err := fnIntersection([]any{[]any{int64(1)}, []any{"1"}})
	if err != nil {
		t.Fatal(err)
	}
	if arr, _ := got.([]any); len(arr) != 0 {
		t.Errorf("int 1 and string \"1\" should not intersect, got %#v", got)
	}
}

func TestWindowsProducesOverlappingRuns(t *testing.T) {
	arr := []any{int64(1), int64(2), int64(3), int64(4)}

	tests := []struct {
		name string
		size int
		want []any
	}{
		{"size 2", 2, []any{
			[]any{int64(1), int64(2)},
			[]any{int64(2), int64(3)},
			[]any{int64(3), int64(4)},
		}},
		{"size equal to length", 4, []any{[]any{int64(1), int64(2), int64(3), int64(4)}}},
		// A window larger than the array yields nothing rather than one short
		// window, so the size guarantee holds for rolling statistics.
		{"larger than array", 5, []any{}},
		{"zero", 0, []any{}},
		{"negative", -1, []any{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fnWindows([]any{arr, tt.size})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestUnzipInvertsZip(t *testing.T) {
	a := []any{int64(1), int64(2), int64(3)}
	b := []any{"x", "y", "z"}

	zipped, err := fnZip([]any{a, b})
	if err != nil {
		t.Fatal(err)
	}
	got, err := fnUnzip([]any{zipped})
	if err != nil {
		t.Fatal(err)
	}

	want := []any{a, b}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestCompactAndPluck(t *testing.T) {
	t.Run("compact removes nulls but keeps other falsy values", func(t *testing.T) {
		got, err := fnCompact([]any{[]any{int64(1), nil, "", nil, false, int64(0)}})
		if err != nil {
			t.Fatal(err)
		}
		want := []any{int64(1), "", false, int64(0)}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %#v, want %#v", got, want)
		}
	})

	t.Run("pluck skips elements lacking the key", func(t *testing.T) {
		got, err := fnPluck([]any{[]any{
			map[string]any{"id": int64(1)},
			map[string]any{"other": int64(9)},
			"not an object",
			map[string]any{"id": nil},
		}, "id"})
		if err != nil {
			t.Fatal(err)
		}
		// A present key holding null is kept; an absent key is skipped.
		want := []any{int64(1), nil}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %#v, want %#v", got, want)
		}
	})
}

func TestConcat(t *testing.T) {
	got, err := fnConcat([]any{
		[]any{int64(1)},
		[]any{int64(2), int64(3)},
		[]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []any{int64(1), int64(2), int64(3)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}

	if _, err := fnConcat([]any{[]any{}, "not an array"}); err == nil {
		t.Error("expected an error for a non-array argument")
	}
}

func TestMathExtras(t *testing.T) {
	tests := []struct {
		name string
		got  func() (any, error)
		want any
	}{
		{"exp(0)", func() (any, error) { return fnExp([]any{0}) }, 1.0},
		{"log2(8)", func() (any, error) { return fnLog2([]any{8}) }, 3.0},
		{"mod positive", func() (any, error) { return fnMod([]any{7, 3}) }, 1.0},
		// Go's Mod takes the sign of the dividend, which the doc states.
		{"mod negative dividend", func() (any, error) { return fnMod([]any{-7, 3}) }, -1.0},
		{"trunc positive", func() (any, error) { return fnTrunc([]any{2.9}) }, 2.0},
		{"trunc negative rounds toward zero", func() (any, error) { return fnTrunc([]any{-2.9}) }, -2.0},
		{"gcd", func() (any, error) { return fnGCD([]any{12, 18}) }, int64(6)},
		{"gcd with zero", func() (any, error) { return fnGCD([]any{0, 5}) }, int64(5)},
		{"gcd negative", func() (any, error) { return fnGCD([]any{-12, 18}) }, int64(6)},
		{"lcm", func() (any, error) { return fnLCM([]any{4, 6}) }, int64(12)},
		{"lcm with zero", func() (any, error) { return fnLCM([]any{0, 5}) }, int64(0)},
		{"hypot", func() (any, error) { return fnHypot([]any{3, 4}) }, 5.0},
		{"atan2", func() (any, error) { return fnAtan2([]any{0, 1}) }, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.got()
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

// mod and log2 error rather than returning NaN or -Inf, which would flow
// silently through the rest of a transformation.
func TestMathExtrasRejectUndefinedInput(t *testing.T) {
	if _, err := fnMod([]any{1, 0}); err == nil {
		t.Error("mod by zero should error rather than return NaN")
	}
	if _, err := fnLog2([]any{0}); err == nil {
		t.Error("log2(0) should error rather than return -Inf")
	}
	if _, err := fnLog2([]any{-1}); err == nil {
		t.Error("log2 of a negative should error")
	}
}

// These inspect the stored value rather than coercing, since ToFloat would turn
// a string or null into a perfectly finite 0 and make the answer meaningless.
func TestIsNaNAndIsFinite(t *testing.T) {
	tests := []struct {
		name            string
		value           any
		isNaN, isFinite bool
	}{
		{"int", int64(1), false, true},
		{"float", 1.5, false, true},
		{"nan", math.NaN(), true, false},
		{"positive infinity", math.Inf(1), false, false},
		{"negative infinity", math.Inf(-1), false, false},
		{"string", "x", false, false},
		{"null", nil, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := fnIsNaN([]any{tt.value})
			if got != tt.isNaN {
				t.Errorf("is_nan = %v, want %v", got, tt.isNaN)
			}
			got, _ = fnIsFinite([]any{tt.value})
			if got != tt.isFinite {
				t.Errorf("is_finite = %v, want %v", got, tt.isFinite)
			}
		})
	}
}

func TestDatetimeExtras(t *testing.T) {
	base := time.Date(2024, 3, 15, 12, 30, 0, 0, time.UTC)

	tests := []struct {
		name string
		got  func() (any, error)
		want any
	}{
		{"to_unix", func() (any, error) { return fnToUnix([]any{base}) }, base.Unix()},
		{"day_of_year", func() (any, error) { return fnDayOfYear([]any{base}) }, int64(75)},
		{"iso_week", func() (any, error) { return fnISOWeek([]any{base}) }, int64(11)},
		{"is_before true", func() (any, error) {
			return fnIsBefore([]any{base, base.Add(time.Hour)})
		}, true},
		{"is_before false when equal", func() (any, error) { return fnIsBefore([]any{base, base}) }, false},
		{"is_after true", func() (any, error) {
			return fnIsAfter([]any{base.Add(time.Hour), base})
		}, true},
		// Inclusive at both ends, which is what "between these dates" means.
		{"is_between at the start bound", func() (any, error) {
			return fnIsBetween([]any{base, base, base.Add(time.Hour)})
		}, true},
		{"is_between at the end bound", func() (any, error) {
			return fnIsBetween([]any{base.Add(time.Hour), base, base.Add(time.Hour)})
		}, true},
		{"is_between outside", func() (any, error) {
			return fnIsBetween([]any{base.Add(-time.Hour), base, base.Add(time.Hour)})
		}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.got()
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestUnixRoundTrip(t *testing.T) {
	base := time.Date(2024, 3, 15, 12, 30, 0, 0, time.UTC)

	unix, err := fnToUnix([]any{base})
	if err != nil {
		t.Fatal(err)
	}
	back, err := fnFromUnix([]any{unix})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := back.(time.Time)
	if !got.Equal(base) {
		t.Errorf("round trip produced %v, want %v", got, base)
	}
}

// duration_between is fractional where diff truncates. Both answers are wanted
// often enough to justify both functions, and this is where they differ.
func TestDurationBetweenIsFractionalWhereDiffTruncates(t *testing.T) {
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(36 * time.Hour) // a day and a half

	fractional, err := fnDurationBetween([]any{from, to, "days"})
	if err != nil {
		t.Fatal(err)
	}
	if fractional != 1.5 {
		t.Errorf("duration_between = %v, want 1.5", fractional)
	}

	truncated, err := fnDiff([]any{from, to, "days"})
	if err != nil {
		t.Fatal(err)
	}
	if truncated != int64(1) {
		t.Errorf("diff = %v, want 1", truncated)
	}
}

func TestDurationBetweenUnits(t *testing.T) {
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(2 * time.Hour)

	tests := []struct {
		unit string
		want float64
	}{
		{"hours", 2},
		{"hour", 2}, // singular and plural both accepted
		{"HOURS", 2},
		{"minutes", 120},
		{"seconds", 7200},
		{"milliseconds", 7200000},
		{"days", 2.0 / 24},
	}

	for _, tt := range tests {
		t.Run(tt.unit, func(t *testing.T) {
			got, err := fnDurationBetween([]any{from, to, tt.unit})
			if err != nil {
				t.Fatal(err)
			}
			if math.Abs(executorFloat(got)-tt.want) > 1e-9 {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}

	if _, err := fnDurationBetween([]any{from, to, "fortnights"}); err == nil {
		t.Error("expected an error for an unknown unit")
	}
}

func executorFloat(v any) float64 {
	f, _ := v.(float64)
	return f
}

func TestDurationBetweenIsNegativeWhenReversed(t *testing.T) {
	from := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	got, err := fnDurationBetween([]any{from, to, "days"})
	if err != nil {
		t.Fatal(err)
	}
	if got != -1.0 {
		t.Errorf("got %v, want -1", got)
	}
}

func TestStatsExtras(t *testing.T) {
	values := []any{int64(1), int64(2), int64(2), int64(3), int64(4)}

	t.Run("mode", func(t *testing.T) {
		got, err := fnMode([]any{values})
		if err != nil {
			t.Fatal(err)
		}
		if got != int64(2) {
			t.Errorf("got %#v, want 2", got)
		}
	})

	t.Run("mode of empty is null", func(t *testing.T) {
		got, err := fnMode([]any{[]any{}})
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Errorf("got %#v, want nil", got)
		}
	})

	// A tie must resolve the same way every run rather than following map
	// iteration order.
	t.Run("mode breaks ties deterministically", func(t *testing.T) {
		tied := []any{int64(1), int64(1), int64(2), int64(2)}
		first, err := fnMode([]any{tied})
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 50; i++ {
			again, _ := fnMode([]any{tied})
			if again != first {
				t.Fatalf("mode is not deterministic: %#v then %#v", first, again)
			}
		}
	})

	t.Run("quantile matches percentile on the same point", func(t *testing.T) {
		q, err := fnQuantile([]any{values, 0.5})
		if err != nil {
			t.Fatal(err)
		}
		p, err := fnPercentile([]any{values, 50})
		if err != nil {
			t.Fatal(err)
		}
		if q != p {
			t.Errorf("quantile(0.5) = %v but percentile(50) = %v", q, p)
		}
	})

	t.Run("quantile clamps out-of-range input", func(t *testing.T) {
		low, _ := fnQuantile([]any{values, -1})
		atZero, _ := fnQuantile([]any{values, 0})
		if low != atZero {
			t.Errorf("negative q should clamp to 0: got %v, want %v", low, atZero)
		}
		high, _ := fnQuantile([]any{values, 2})
		atOne, _ := fnQuantile([]any{values, 1})
		if high != atOne {
			t.Errorf("q above 1 should clamp: got %v, want %v", high, atOne)
		}
	})

	t.Run("cv of a zero mean is zero rather than infinite", func(t *testing.T) {
		got, err := fnCV([]any{[]any{int64(-1), int64(1)}})
		if err != nil {
			t.Fatal(err)
		}
		if got != 0.0 {
			t.Errorf("got %#v, want 0", got)
		}
	})

	t.Run("empty inputs return neutral values", func(t *testing.T) {
		empty := []any{[]any{}}
		for name, fn := range map[string]func([]any) (any, error){
			"quantile":    func(a []any) (any, error) { return fnQuantile([]any{a[0], 0.5}) },
			"cv":          fnCV,
			"sum_squares": fnSumSquares,
		} {
			got, err := fn(empty)
			if err != nil {
				t.Errorf("%s errored on empty input: %v", name, err)
			}
			if got != 0.0 {
				t.Errorf("%s of empty = %#v, want 0", name, got)
			}
		}
	})
}

// A perfect line must come back with the exact slope and intercept, and an r2
// of 1.
func TestLinregOnAPerfectLine(t *testing.T) {
	xs := []any{int64(1), int64(2), int64(3), int64(4)}
	ys := []any{int64(3), int64(5), int64(7), int64(9)} // y = 2x + 1

	got, err := fnLinreg([]any{xs, ys})
	if err != nil {
		t.Fatal(err)
	}
	obj, _ := got.(map[string]any)

	for field, want := range map[string]float64{"slope": 2, "intercept": 1, "r2": 1} {
		if math.Abs(executorFloat(obj[field])-want) > 1e-9 {
			t.Errorf("%s = %v, want %v", field, obj[field], want)
		}
	}
}

// A vertical line has no least-squares slope. The result keeps its shape so
// callers reading fields off it do not have to special-case the failure.
func TestLinregOnAVerticalLineKeepsItsShape(t *testing.T) {
	xs := []any{int64(1), int64(1), int64(1)}
	ys := []any{int64(1), int64(2), int64(3)}

	got, err := fnLinreg([]any{xs, ys})
	if err != nil {
		t.Fatal(err)
	}
	obj, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("got %T, want an object", got)
	}
	for _, field := range []string{"slope", "intercept", "r2"} {
		if _, present := obj[field]; !present {
			t.Errorf("result is missing %q", field)
		}
	}
}

func TestCovariance(t *testing.T) {
	// Deviations are (-1, 0, 1) and (-2, 0, 2), so the products sum to 4 and
	// the population covariance is 4/3.
	xs := []any{int64(1), int64(2), int64(3)}
	ys := []any{int64(2), int64(4), int64(6)}

	got, err := fnCovariance([]any{xs, ys})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(executorFloat(got)-4.0/3.0) > 1e-9 {
		t.Errorf("got %v, want %v", got, 4.0/3.0)
	}

	// Mismatched lengths compare up to the shorter, matching correlation.
	if _, err := fnCovariance([]any{xs, []any{int64(1)}}); err != nil {
		t.Errorf("mismatched lengths should not error: %v", err)
	}
}

// Unit names were normalised by trimming a trailing "s" before lowercasing,
// which left "DAYS" ending in a capital S that no case matched. dt_add survived
// by accident — it normalises, then passes the result to durationUnit, which
// normalises again — but diff normalises once and fell through to a default
// that silently returns seconds. Asking for a difference in "DAYS" produced
// 172800 instead of 2.
func TestUnitNamesAreCaseInsensitive(t *testing.T) {
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(48 * time.Hour)

	for _, unit := range []string{"day", "days", "Days", "DAYS", "  days  "} {
		t.Run(unit, func(t *testing.T) {
			got, err := fnDiff([]any{from, to, unit})
			if err != nil {
				t.Fatal(err)
			}
			if got != int64(2) {
				t.Errorf("diff in %q = %v, want 2", unit, got)
			}

			added, err := fnDtAdd([]any{from, 1, unit})
			if err != nil {
				t.Fatal(err)
			}
			tm, _ := added.(time.Time)
			if delta := tm.Sub(from); delta != 24*time.Hour {
				t.Errorf("dt_add 1 %q moved by %v, want 24h", unit, delta)
			}

			subtracted, err := fnDtSubtract([]any{to, 1, unit})
			if err != nil {
				t.Fatal(err)
			}
			tm, _ = subtracted.(time.Time)
			if delta := to.Sub(tm); delta != 24*time.Hour {
				t.Errorf("dt_subtract 1 %q moved by %v, want 24h", unit, delta)
			}
		})
	}
}

// An unrecognised unit is an error rather than a silent fallback.
//
// Each of these previously produced a plausible-looking wrong answer: diff
// returned seconds, dt_add and dt_subtract moved by one second, and start_of,
// end_of and time_bucket returned the input untouched. All reported success, so
// nothing downstream could tell the difference.
func TestUnknownUnitsAreRejected(t *testing.T) {
	base := time.Date(2024, 3, 15, 12, 30, 45, 0, time.UTC)
	later := base.Add(48 * time.Hour)

	tests := []struct {
		name string
		call func(unit string) (any, error)
	}{
		{"diff", func(u string) (any, error) { return fnDiff([]any{base, later, u}) }},
		{"dt_add", func(u string) (any, error) { return fnDtAdd([]any{base, 1, u}) }},
		{"dt_subtract", func(u string) (any, error) { return fnDtSubtract([]any{base, 1, u}) }},
		{"start_of", func(u string) (any, error) { return fnStartOf([]any{base, u}) }},
		{"end_of", func(u string) (any, error) { return fnEndOf([]any{base, u}) }},
		{"time_bucket", func(u string) (any, error) { return fnTimeBucket([]any{base, u}) }},
	}

	for _, tt := range tests {
		for _, unit := range []string{"fortnights", "", "decade", "dayz"} {
			t.Run(tt.name+"/"+unit, func(t *testing.T) {
				_, err := tt.call(unit)
				if err == nil {
					t.Fatalf("%s accepted unit %q instead of reporting it", tt.name, unit)
				}
				// The message must name the offending unit and what was
				// expected, so the fix is visible without reading the source.
				if !strings.Contains(err.Error(), "unknown unit") {
					t.Errorf("error %q should say the unit was unknown", err)
				}
				if !strings.Contains(err.Error(), "expected one of") {
					t.Errorf("error %q should list the accepted units", err)
				}
			})
		}
	}
}

// start_of and end_of accepted only exact singular lowercase, so start_of(dt,
// "days") silently returned the input while dt_add(dt, 1, "days") worked. An
// author who learned the plural from one got a no-op from the other.
func TestStartOfAndEndOfAcceptPluralAndMixedCase(t *testing.T) {
	base := time.Date(2024, 3, 15, 12, 30, 45, 0, time.UTC)

	for _, unit := range []string{"day", "days", "Days", "DAYS", " days "} {
		t.Run(unit, func(t *testing.T) {
			got, err := fnStartOf([]any{base, unit})
			if err != nil {
				t.Fatal(err)
			}
			tm, _ := got.(time.Time)
			want := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
			if !tm.Equal(want) {
				t.Errorf("start_of %q = %v, want %v", unit, tm, want)
			}

			got, err = fnEndOf([]any{base, unit})
			if err != nil {
				t.Fatal(err)
			}
			tm, _ = got.(time.Time)
			if tm.Hour() != 23 || tm.Minute() != 59 {
				t.Errorf("end_of %q = %v, want the last instant of the day", unit, tm)
			}
		})
	}
}

// end_of now handles the hour and minute that start_of always did, and that its
// documentation already claimed.
func TestEndOfHandlesHourAndMinute(t *testing.T) {
	base := time.Date(2024, 3, 15, 12, 30, 45, 0, time.UTC)

	got, err := fnEndOf([]any{base, "hour"})
	if err != nil {
		t.Fatal(err)
	}
	tm, _ := got.(time.Time)
	if tm.Hour() != 12 || tm.Minute() != 59 || tm.Second() != 59 {
		t.Errorf("end_of hour = %v, want 12:59:59", tm)
	}

	got, err = fnEndOf([]any{base, "minute"})
	if err != nil {
		t.Fatal(err)
	}
	tm, _ = got.(time.Time)
	if tm.Minute() != 30 || tm.Second() != 59 {
		t.Errorf("end_of minute = %v, want 12:30:59", tm)
	}
}

func TestDtParse(t *testing.T) {
	tests := []struct {
		name, input, format string
		want                time.Time
	}{
		{"date", "2024-03-15", "YYYY-MM-DD", time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)},
		{"datetime", "2024-03-15 12:30:45", "YYYY-MM-DD HH:mm:ss", time.Date(2024, 3, 15, 12, 30, 45, 0, time.UTC)},
		{"two-digit year", "24-03-15", "YY-MM-DD", time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)},
		{"slashes", "15/03/2024", "DD/MM/YYYY", time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fnDtParse([]any{tt.input, tt.format})
			if err != nil {
				t.Fatal(err)
			}
			tm, _ := got.(time.Time)
			if !tm.Equal(tt.want) {
				t.Errorf("got %v, want %v", tm, tt.want)
			}
		})
	}
}

// A mismatch is an error naming both the input and the format, since a silent
// zero datetime would flow onward looking like a real timestamp.
func TestDtParseRejectsMismatches(t *testing.T) {
	for _, tt := range []struct{ input, format string }{
		{"not a date", "YYYY-MM-DD"},
		{"2024-03-15", "DD/MM/YYYY"},
		{"", "YYYY-MM-DD"},
	} {
		t.Run(tt.input, func(t *testing.T) {
			_, err := fnDtParse([]any{tt.input, tt.format})
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), "dt_parse") {
				t.Errorf("error %q should name dt_parse", err)
			}
		})
	}
}

// dt_parse and dt_format are inverses, so a value written by one must be read
// back by the other.
func TestDtParseRoundTripsWithDtFormat(t *testing.T) {
	base := time.Date(2024, 3, 15, 12, 30, 45, 0, time.UTC)
	const format = "YYYY-MM-DD HH:mm:ss"

	formatted, err := fnDtFormat([]any{base, format})
	if err != nil {
		t.Fatal(err)
	}
	back, err := fnDtParse([]any{formatted, format})
	if err != nil {
		t.Fatalf("could not re-parse %q: %v", formatted, err)
	}
	tm, _ := back.(time.Time)
	if !tm.Equal(base) {
		t.Errorf("round trip gave %v, want %v", tm, base)
	}
}

// dt_in_zone depends on IANA zone data, which comes from the host or from an
// embedded copy and is absent in a scratch container. Both outcomes are
// acceptable; silently leaving the datetime in UTC is not.
func TestDtInZone(t *testing.T) {
	base := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)

	got, err := fnDtInZone([]any{base, "America/New_York"})
	if err != nil {
		if !strings.Contains(err.Error(), "zone data") {
			t.Fatalf("unexpected error: %v", err)
		}
		t.Skip("no IANA zone data on this host; the error explains how to embed it")
	}

	tm, _ := got.(time.Time)
	// The instant is unchanged; only the wall-clock reading moves.
	if !tm.Equal(base) {
		t.Errorf("dt_in_zone changed the instant: %v vs %v", tm, base)
	}
	if tm.Hour() == base.Hour() {
		t.Errorf("expected the wall-clock hour to differ from UTC, both are %d", tm.Hour())
	}
}

func TestDtInZoneRejectsUnknownZones(t *testing.T) {
	base := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)
	_, err := fnDtInZone([]any{base, "Mars/Olympus_Mons"})
	if err == nil {
		t.Fatal("expected an error for an unknown zone")
	}
	if !strings.Contains(err.Error(), "dt_in_zone") {
		t.Errorf("error %q should name dt_in_zone", err)
	}
}

func TestTrigonometry(t *testing.T) {
	tests := []struct {
		name string
		fn   func([]any) (any, error)
		in   float64
		want float64
	}{
		{"sin 0", fnSin, 0, 0},
		{"sin pi/2", fnSin, math.Pi / 2, 1},
		{"cos 0", fnCos, 0, 1},
		{"cos pi", fnCos, math.Pi, -1},
		{"tan 0", fnTan, 0, 0},
		{"tan pi/4", fnTan, math.Pi / 4, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fn([]any{tt.in})
			if err != nil {
				t.Fatal(err)
			}
			if math.Abs(executorFloat(got)-tt.want) > 1e-9 {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

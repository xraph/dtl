package stdlib

import (
	"sort"

	"github.com/xraph/dtl/executor"
)

// Descriptive statistics beyond the mean and spread the library already had.
//
// These follow the convention the existing stats functions set: a degenerate
// input returns a neutral value rather than an error, because a summary
// computed over a filtered-to-empty array is a normal thing to happen partway
// through a transformation and should not abort it.
func registerStatsExtra(m map[string]*executor.BuiltinFunc) {
	register(m, "mode", 1, 1, fnMode,
		"mode(values) -> any -- Most frequent value; the smallest of them when tied, null when empty")
	register(m, "quantile", 2, 2, fnQuantile,
		"quantile(values, q) -> float -- Value at quantile q, given as a fraction from 0 to 1")
	register(m, "covariance", 2, 2, fnCovariance,
		"covariance(xs, ys) -> float -- Population covariance, compared up to the shorter array's length")
	register(m, "cv", 1, 1, fnCV,
		"cv(values) -> float -- Coefficient of variation: standard deviation over the mean. 0 when the mean is 0")
	register(m, "sum_squares", 1, 1, fnSumSquares,
		"sum_squares(values) -> float -- Sum of squared deviations from the mean")
	register(m, "linreg", 2, 2, fnLinreg,
		"linreg(xs, ys) -> object -- Least-squares fit as {slope, intercept, r2}")
}

// fnMode breaks ties by taking the smallest value rather than whichever the map
// happened to yield first, so the result does not vary between runs.
func fnMode(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok || len(arr) == 0 {
		return nil, nil
	}

	counts := make(map[string]int, len(arr))
	values := make(map[string]any, len(arr))
	for _, v := range arr {
		k := setKey(v)
		counts[k]++
		values[k] = v
	}

	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	best, bestCount := keys[0], counts[keys[0]]
	for _, k := range keys[1:] {
		if counts[k] > bestCount {
			best, bestCount = k, counts[k]
		}
	}
	return values[best], nil
}

// fnQuantile takes a fraction, where the existing percentile takes 0-100. Both
// spellings are common and converting between them at the call site is the kind
// of arithmetic that invites off-by-a-factor-of-100 mistakes.
func fnQuantile(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok || len(arr) == 0 {
		return 0.0, nil
	}
	q := executor.ToFloat(args[1])
	if q < 0 {
		q = 0
	}
	if q > 1 {
		q = 1
	}
	return percentileFromSorted(sortedFloats(arr), q*100), nil
}

// pairedFloats aligns two numeric arrays to their common length, which is how
// the existing correlation treats mismatched inputs.
func pairedFloats(a, b any) ([]float64, []float64) {
	xArr, xOK := a.([]any)
	yArr, yOK := b.([]any)
	if !xOK || !yOK {
		return nil, nil
	}

	n := len(xArr)
	if len(yArr) < n {
		n = len(yArr)
	}
	if n == 0 {
		return nil, nil
	}

	xs, ys := make([]float64, n), make([]float64, n)
	for i := 0; i < n; i++ {
		xs[i] = executor.ToFloat(xArr[i])
		ys[i] = executor.ToFloat(yArr[i])
	}
	return xs, ys
}

func mean(vs []float64) float64 {
	if len(vs) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vs {
		sum += v
	}
	return sum / float64(len(vs))
}

func fnCovariance(args []any) (any, error) {
	xs, ys := pairedFloats(args[0], args[1])
	if len(xs) == 0 {
		return 0.0, nil
	}

	mx, my := mean(xs), mean(ys)
	sum := 0.0
	for i := range xs {
		sum += (xs[i] - mx) * (ys[i] - my)
	}
	return sum / float64(len(xs)), nil
}

// fnCV returns 0 rather than an infinity when the mean is 0, since the ratio is
// undefined there and an Inf would flow silently through the rest of a
// transformation.
func fnCV(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok || len(arr) == 0 {
		return 0.0, nil
	}

	m := arrayMean(arr)
	if m == 0 {
		return 0.0, nil
	}
	return arrayStdev(arr, m) / m, nil
}

func fnSumSquares(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok || len(arr) == 0 {
		return 0.0, nil
	}

	m := arrayMean(arr)
	sum := 0.0
	for _, v := range arr {
		d := executor.ToFloat(v) - m
		sum += d * d
	}
	return sum, nil
}

// fnLinreg returns slope, intercept and r2 together because computing them
// separately would traverse the data three times and recompute the same sums.
func fnLinreg(args []any) (any, error) {
	xs, ys := pairedFloats(args[0], args[1])
	zero := map[string]any{"slope": 0.0, "intercept": 0.0, "r2": 0.0}
	if len(xs) == 0 {
		return zero, nil
	}

	mx, my := mean(xs), mean(ys)

	var sxx, sxy float64
	for i := range xs {
		dx := xs[i] - mx
		sxx += dx * dx
		sxy += dx * (ys[i] - my)
	}
	// A vertical line has no least-squares slope. Returning zeros keeps the
	// shape of the result stable for callers reading fields off it.
	if sxx == 0 {
		return zero, nil
	}

	slope := sxy / sxx
	intercept := my - slope*mx

	var ssRes, ssTot float64
	for i := range xs {
		residual := ys[i] - (slope*xs[i] + intercept)
		deviation := ys[i] - my
		ssRes += residual * residual
		ssTot += deviation * deviation
	}
	r2 := 1.0
	if ssTot != 0 {
		r2 = 1 - ssRes/ssTot
	}

	return map[string]any{"slope": slope, "intercept": intercept, "r2": r2}, nil
}

package stdlib

import (
	"math"
	"sort"

	"github.com/xraph/dtl/executor"
)

func registerStats(m map[string]*executor.BuiltinFunc) {
	register(m, "z_score", 2, 2, fnZScore,
		"z_score(value, values) -> float -- Standard deviations between value and the mean of values; 0 when values is empty or has no spread")
	register(m, "outlier_bounds", 1, 2, fnOutlierBounds,
		"outlier_bounds(values, factor?) -> object -- Tukey fences as {lower, upper}, using factor x IQR (default 1.5)")
	register(m, "correlation", 2, 2, fnCorrelation,
		"correlation(xs, ys) -> float -- Pearson correlation coefficient. Compares up to the shorter array's length")

	// Legacy namespace spellings, aliased so they cannot drift from the bare names.
	alias(m, "system::stats::z_score", "z_score")
	alias(m, "system::stats::outlier_bounds", "outlier_bounds")
	alias(m, "system::stats::correlation", "correlation")
}

func fnZScore(args []any) (any, error) {
	value := executor.ToFloat(args[0])
	arr, ok := args[1].([]any)
	if !ok || len(arr) == 0 {
		return 0.0, nil
	}

	mean := arrayMean(arr)
	std := arrayStdev(arr, mean)
	if std == 0 {
		return 0.0, nil
	}
	return (value - mean) / std, nil
}

func fnOutlierBounds(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok || len(arr) == 0 {
		return map[string]any{"lower": 0.0, "upper": 0.0}, nil
	}
	factor := 1.5
	if len(args) > 1 {
		factor = executor.ToFloat(args[1])
	}

	sorted := sortedFloats(arr)
	q1 := percentileFromSorted(sorted, 25)
	q3 := percentileFromSorted(sorted, 75)
	iqr := q3 - q1

	return map[string]any{
		"lower": q1 - factor*iqr,
		"upper": q3 + factor*iqr,
	}, nil
}

func fnCorrelation(args []any) (any, error) {
	xArr, xOk := args[0].([]any)
	yArr, yOk := args[1].([]any)
	if !xOk || !yOk || len(xArr) == 0 || len(yArr) == 0 {
		return 0.0, nil
	}

	n := len(xArr)
	if len(yArr) < n {
		n = len(yArr)
	}

	var sumX, sumY, sumXY, sumX2, sumY2 float64
	nf := float64(n)
	for i := 0; i < n; i++ {
		x := executor.ToFloat(xArr[i])
		y := executor.ToFloat(yArr[i])
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
		sumY2 += y * y
	}

	denom := math.Sqrt((nf*sumX2 - sumX*sumX) * (nf*sumY2 - sumY*sumY))
	if denom == 0 {
		return 0.0, nil
	}
	return (nf*sumXY - sumX*sumY) / denom, nil
}

// --- Shared stats helpers (used by both stats and collections) ---

func arrayMean(arr []any) float64 {
	if len(arr) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range arr {
		sum += executor.ToFloat(v)
	}
	return sum / float64(len(arr))
}

func arrayStdev(arr []any, mean float64) float64 {
	if len(arr) < 2 {
		return 0
	}
	sumSq := 0.0
	for _, v := range arr {
		d := executor.ToFloat(v) - mean
		sumSq += d * d
	}
	return math.Sqrt(sumSq / float64(len(arr)))
}

func sortedFloats(arr []any) []float64 {
	fs := make([]float64, len(arr))
	for i, v := range arr {
		fs[i] = executor.ToFloat(v)
	}
	sort.Float64s(fs)
	return fs
}

func percentileFromSorted(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	rank := (p / 100.0) * float64(len(sorted)-1)
	lower := int(math.Floor(rank))
	upper := lower + 1
	if upper >= len(sorted) {
		return sorted[len(sorted)-1]
	}
	frac := rank - float64(lower)
	return sorted[lower] + frac*(sorted[upper]-sorted[lower])
}

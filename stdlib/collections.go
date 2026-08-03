package stdlib

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/xraph/dtl/executor"
)

func registerCollections(m map[string]*executor.BuiltinFunc) {
	register(m, "map", 2, 2, fnMap,
		"map(arr, fn) -> array -- Applies fn to every element")
	register(m, "filter", 2, 2, fnFilter,
		"filter(arr, fn) -> array -- Keeps the elements for which fn returns true")
	register(m, "reduce", 3, 3, fnReduce,
		"reduce(arr, init, fn) -> any -- Folds the array into a single value, starting from init")
	register(m, "sort", 1, 2, fnSort,
		"sort(arr, dir?) -> array -- Sorts ascending, or descending when dir is 'desc'")
	register(m, "sort_by", 2, 3, fnSortBy,
		"sort_by(arr, key, dir?) -> array -- Sorts objects by a key, descending when dir is 'desc'")
	register(m, "tail", 2, 2, fnTail,
		"tail(arr, n) -> array -- Last n elements")
	register(m, "head", 2, 2, fnHead,
		"head(arr, n) -> array -- First n elements")
	register(m, "unique", 1, 1, fnUnique,
		"unique(arr) -> array -- Removes duplicates, preserving first-seen order")
	register(m, "range", 1, 2, fnRange,
		"range(end) or range(start, end) -> int[] -- Integers from start (default 0) up to but excluding end")
	register(m, "flatten", 1, 1, fnFlatten,
		"flatten(arr) -> array -- Flattens one level of nesting")
	register(m, "zip", 2, 2, fnZip,
		"zip(a, b) -> array -- Pairs elements positionally, stopping at the shorter array")
	register(m, "group_by", 2, 2, fnGroupBy,
		"group_by(arr, fn) -> object -- Groups elements into arrays keyed by fn's result")
	register(m, "chunk", 2, 2, fnChunk,
		"chunk(arr, size) -> array -- Splits into consecutive chunks of at most size")
	register(m, "first", 1, 1, fnFirst,
		"first(arr) -> any -- First element, or null when empty")
	register(m, "last", 1, 1, fnLast,
		"last(arr) -> any -- Last element, or null when empty")
	register(m, "top_n", 2, 3, fnTopN,
		"top_n(arr, n, key?) -> array -- Highest n elements, ranked by key when the elements are objects")
	register(m, "histogram", 1, 2, fnHistogram,
		"histogram(values, bins?) -> array -- Buckets values into equal-width bins (default 10)")

	// Aggregation functions
	register(m, "sum", 1, 1, fnSum,
		"sum(arr) -> float -- Sum of all elements")
	register(m, "avg", 1, 1, fnAvg,
		"avg(arr) -> float -- Arithmetic mean")
	register(m, "min", 1, -1, fnMin,
		"min(arr) or min(a, b, ...) -> float -- Smallest value")
	register(m, "max", 1, -1, fnMax,
		"max(arr) or max(a, b, ...) -> float -- Largest value")
	register(m, "count", 1, 1, fnCount,
		"count(arr) -> int -- Number of elements")
	register(m, "stdev", 1, 1, fnStdev,
		"stdev(arr) -> float -- Standard deviation")
	register(m, "variance", 1, 1, fnVariance,
		"variance(arr) -> float -- Variance")
	register(m, "median", 1, 1, fnMedian,
		"median(arr) -> float -- Middle value, averaging the two middle values when the count is even")
	register(m, "percentile", 2, 2, fnPercentile,
		"percentile(arr, p) -> float -- Value at percentile p, clamped to 0..100")
	register(m, "count_where", 2, 2, fnCountWhere,
		"count_where(arr, fn) -> int -- Number of elements for which fn returns true")
	register(m, "sum_where", 2, 2, fnSumWhere,
		"sum_where(arr, fn) -> float -- Sum of the elements for which fn returns true")
	register(m, "find", 2, 2, fnFind,
		"find(arr, fn) -> any -- First element for which fn returns true, or null")
	register(m, "find_index", 2, 2, fnFindIndex,
		"find_index(arr, fn) -> int -- Index of the first match, or -1")
	register(m, "includes", 2, 2, fnIncludes,
		"includes(arr, value) -> bool -- Whether the value is present")
	register(m, "every", 2, 2, fnEvery,
		"every(arr, fn) -> bool -- Whether fn returns true for every element")
	register(m, "some", 2, 2, fnSome,
		"some(arr, fn) -> bool -- Whether fn returns true for any element")
	register(m, "reverse", 1, 1, fnReverseArr,
		"reverse(arr) -> array -- Reverses element order")
	register(m, "seq", 1, 3, fnSeq,
		"seq(end) or seq(start, end, step?) -> int[] -- Integer sequence; steps backwards automatically when start exceeds end")
	register(m, "take_while", 2, 2, fnTakeWhile,
		"take_while(arr, fn) -> array -- Leading elements while fn returns true")
	register(m, "drop_while", 2, 2, fnDropWhile,
		"drop_while(arr, fn) -> array -- Remaining elements after the leading run for which fn returns true")
	register(m, "distinct_by", 2, 2, fnDistinctBy,
		"distinct_by(arr, fn) -> array -- Keeps the first element for each distinct fn result")

	// Legacy namespace spellings, aliased so they cannot drift from the bare names.
	alias(m, "system::collections::top_n", "top_n")
	alias(m, "system::collections::histogram", "histogram")
	alias(m, "system::collections::find", "find")
	alias(m, "system::collections::find_index", "find_index")
	alias(m, "system::collections::includes", "includes")
	alias(m, "system::collections::every", "every")
	alias(m, "system::collections::some", "some")
	alias(m, "system::collections::seq", "seq")
}

func fnMap(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok {
		return nil, fmt.Errorf("map: first argument must be an array")
	}
	result := make([]any, 0, len(arr))
	for _, item := range arr {
		val, err := executor.CallLambda(context.Background(), args[1], []any{item}, time.Now(), 0)
		if err != nil {
			return nil, fmt.Errorf("map: %w", err)
		}
		result = append(result, val)
	}
	return result, nil
}

func fnFilter(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok {
		return nil, fmt.Errorf("filter: first argument must be an array")
	}
	result := make([]any, 0)
	for _, item := range arr {
		val, err := executor.CallLambda(context.Background(), args[1], []any{item}, time.Now(), 0)
		if err != nil {
			return nil, fmt.Errorf("filter: %w", err)
		}
		if executor.ToBool(val) {
			result = append(result, item)
		}
	}
	return result, nil
}

func fnReduce(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok {
		return nil, fmt.Errorf("reduce: first argument must be an array")
	}
	acc := args[1]
	for _, item := range arr {
		val, err := executor.CallLambda(context.Background(), args[2], []any{acc, item}, time.Now(), 0)
		if err != nil {
			return nil, fmt.Errorf("reduce: %w", err)
		}
		acc = val
	}
	return acc, nil
}

func fnSort(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok {
		return nil, fmt.Errorf("sort: first argument must be an array")
	}

	desc := false
	if len(args) > 1 {
		dir := executor.ToString(args[1])
		desc = dir == "desc"
	}

	sorted := make([]any, len(arr))
	copy(sorted, arr)

	sort.SliceStable(sorted, func(i, j int) bool {
		a := executor.ToFloat(sorted[i])
		b := executor.ToFloat(sorted[j])
		if desc {
			return a > b
		}
		return a < b
	})
	return sorted, nil
}

func fnSortBy(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok {
		return nil, fmt.Errorf("sort_by: first argument must be an array")
	}
	key := executor.ToString(args[1])
	desc := false
	if len(args) > 2 {
		dir := executor.ToString(args[2])
		desc = dir == "desc"
	}

	sorted := make([]any, len(arr))
	copy(sorted, arr)

	sort.SliceStable(sorted, func(i, j int) bool {
		ai := getField(sorted[i], key)
		bi := getField(sorted[j], key)
		a := executor.ToFloat(ai)
		b := executor.ToFloat(bi)
		if desc {
			return a > b
		}
		return a < b
	})
	return sorted, nil
}

func fnTail(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok {
		return nil, fmt.Errorf("tail: first argument must be an array")
	}
	n := int(executor.ToInt(args[1]))
	if n >= len(arr) {
		return arr, nil
	}
	if n <= 0 {
		return []any{}, nil
	}
	return arr[len(arr)-n:], nil
}

func fnHead(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok {
		return nil, fmt.Errorf("head: first argument must be an array")
	}
	n := int(executor.ToInt(args[1]))
	if n >= len(arr) {
		return arr, nil
	}
	if n <= 0 {
		return []any{}, nil
	}
	return arr[:n], nil
}

func fnUnique(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok {
		return nil, fmt.Errorf("unique: first argument must be an array")
	}
	seen := make(map[string]bool)
	result := make([]any, 0, len(arr))
	for _, v := range arr {
		key := fmt.Sprintf("%v", v)
		if !seen[key] {
			seen[key] = true
			result = append(result, v)
		}
	}
	return result, nil
}

// maxRange caps the sequence range will build. Every DTL iteration construct
// (for..in, map, filter) needs an array to walk, so range is how a function
// says "do this N times" — and an unbounded N would allocate until the process
// died, which the executor's depth and timeout limits do not catch inside a
// single builtin call.
const maxRange = 10_000_000

// fnRange builds [0, n) or [start, end). It is the sequence generator DTL
// otherwise lacks: walking a date horizon or a shift cycle has no existing
// array to start from.
func fnRange(args []any) (any, error) {
	start, end := int64(0), executor.ToInt(args[0])
	if len(args) == 2 {
		start, end = end, executor.ToInt(args[1])
	}
	if end <= start {
		return []any{}, nil
	}
	if end-start > maxRange {
		return nil, fmt.Errorf("range: refusing to build %d elements (limit %d)", end-start, maxRange)
	}
	out := make([]any, 0, end-start)
	for i := start; i < end; i++ {
		out = append(out, i)
	}
	return out, nil
}

func fnFlatten(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok {
		return nil, fmt.Errorf("flatten: first argument must be an array")
	}
	result := make([]any, 0, len(arr))
	for _, v := range arr {
		if inner, ok := v.([]any); ok {
			result = append(result, inner...)
		} else {
			result = append(result, v)
		}
	}
	return result, nil
}

func fnZip(args []any) (any, error) {
	a, aOk := args[0].([]any)
	b, bOk := args[1].([]any)
	if !aOk || !bOk {
		return nil, fmt.Errorf("zip: both arguments must be arrays")
	}
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	result := make([]any, 0, n)
	for i := 0; i < n; i++ {
		result = append(result, []any{a[i], b[i]})
	}
	return result, nil
}

func fnGroupBy(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok {
		return nil, fmt.Errorf("group_by: first argument must be an array")
	}
	groups := make(map[string][]any)
	for _, item := range arr {
		key, err := executor.CallLambda(context.Background(), args[1], []any{item}, time.Now(), 0)
		if err != nil {
			return nil, fmt.Errorf("group_by: %w", err)
		}
		k := executor.ToString(key)
		groups[k] = append(groups[k], item)
	}
	result := make(map[string]any, len(groups))
	for k, v := range groups {
		result[k] = v
	}
	return result, nil
}

func fnChunk(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok {
		return nil, fmt.Errorf("chunk: first argument must be an array")
	}
	size := int(executor.ToInt(args[1]))
	if size <= 0 {
		return []any{arr}, nil
	}

	result := make([]any, 0, (len(arr)+size-1)/size)
	for i := 0; i < len(arr); i += size {
		end := i + size
		if end > len(arr) {
			end = len(arr)
		}
		result = append(result, arr[i:end])
	}
	return result, nil
}

func fnFirst(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok || len(arr) == 0 {
		return nil, nil
	}
	return arr[0], nil
}

func fnLast(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok || len(arr) == 0 {
		return nil, nil
	}
	return arr[len(arr)-1], nil
}

func fnTopN(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok {
		return nil, fmt.Errorf("top_n: first argument must be an array")
	}
	n := int(executor.ToInt(args[1]))
	key := ""
	if len(args) > 2 {
		key = executor.ToString(args[2])
	}

	sorted := make([]any, len(arr))
	copy(sorted, arr)

	if key == "" {
		sort.SliceStable(sorted, func(i, j int) bool {
			return executor.ToFloat(sorted[i]) > executor.ToFloat(sorted[j])
		})
	} else {
		sort.SliceStable(sorted, func(i, j int) bool {
			return executor.ToFloat(getField(sorted[i], key)) > executor.ToFloat(getField(sorted[j], key))
		})
	}

	if n > len(sorted) {
		n = len(sorted)
	}
	return sorted[:n], nil
}

func fnHistogram(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok || len(arr) == 0 {
		return []any{}, nil
	}
	bins := 10
	if len(args) > 1 {
		bins = int(executor.ToInt(args[1]))
		if bins <= 0 {
			bins = 10
		}
	}

	floats := sortedFloats(arr)
	minVal := floats[0]
	maxVal := floats[len(floats)-1]
	binWidth := (maxVal - minVal) / float64(bins)
	if binWidth == 0 {
		binWidth = 1
	}

	counts := make([]int, bins)
	for _, v := range floats {
		idx := int((v - minVal) / binWidth)
		if idx >= bins {
			idx = bins - 1
		}
		if idx < 0 {
			idx = 0
		}
		counts[idx]++
	}

	result := make([]any, bins)
	for i := 0; i < bins; i++ {
		result[i] = map[string]any{
			"bin":         int64(i),
			"range_start": minVal + float64(i)*binWidth,
			"range_end":   minVal + float64(i+1)*binWidth,
			"count":       int64(counts[i]),
		}
	}
	return result, nil
}

// --- Aggregation Functions ---

func fnSum(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok {
		return executor.ToFloat(args[0]), nil
	}
	sum := 0.0
	for _, v := range arr {
		sum += executor.ToFloat(v)
	}
	return sum, nil
}

func fnAvg(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok || len(arr) == 0 {
		return 0.0, nil
	}
	return arrayMean(arr), nil
}

func fnMin(args []any) (any, error) {
	// Array aggregate: min([1, 2, 3])
	if arr, ok := args[0].([]any); ok && len(args) == 1 {
		if len(arr) == 0 {
			return nil, nil
		}
		result := executor.ToFloat(arr[0])
		for _, v := range arr[1:] {
			f := executor.ToFloat(v)
			if f < result {
				result = f
			}
		}
		return result, nil
	}
	// Scalar overload: min(3, 5, 1)
	if len(args) > 1 {
		result := executor.ToFloat(args[0])
		for _, a := range args[1:] {
			f := executor.ToFloat(a)
			if f < result {
				result = f
			}
		}
		return result, nil
	}
	return args[0], nil
}

func fnMax(args []any) (any, error) {
	// Array aggregate: max([1, 2, 3])
	if arr, ok := args[0].([]any); ok && len(args) == 1 {
		if len(arr) == 0 {
			return nil, nil
		}
		result := executor.ToFloat(arr[0])
		for _, v := range arr[1:] {
			f := executor.ToFloat(v)
			if f > result {
				result = f
			}
		}
		return result, nil
	}
	// Scalar overload: max(3, 5, 1)
	if len(args) > 1 {
		result := executor.ToFloat(args[0])
		for _, a := range args[1:] {
			f := executor.ToFloat(a)
			if f > result {
				result = f
			}
		}
		return result, nil
	}
	return args[0], nil
}

func fnCount(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok {
		if args[0] == nil {
			return int64(0), nil
		}
		return int64(1), nil
	}
	return int64(len(arr)), nil
}

func fnStdev(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok || len(arr) < 2 {
		return 0.0, nil
	}
	mean := arrayMean(arr)
	return arrayStdev(arr, mean), nil
}

func fnVariance(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok || len(arr) < 2 {
		return 0.0, nil
	}
	mean := arrayMean(arr)
	sumSq := 0.0
	for _, v := range arr {
		d := executor.ToFloat(v) - mean
		sumSq += d * d
	}
	return sumSq / float64(len(arr)), nil
}

func fnMedian(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok || len(arr) == 0 {
		return 0.0, nil
	}
	sorted := sortedFloats(arr)
	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2.0, nil
	}
	return sorted[n/2], nil
}

func fnPercentile(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok || len(arr) == 0 {
		return 0.0, nil
	}
	p := executor.ToFloat(args[1])
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	sorted := sortedFloats(arr)
	return percentileFromSorted(sorted, p), nil
}

func fnCountWhere(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok {
		return int64(0), nil
	}
	count := int64(0)
	for _, item := range arr {
		val, err := executor.CallLambda(context.Background(), args[1], []any{item}, time.Now(), 0)
		if err != nil {
			return nil, fmt.Errorf("count_where: %w", err)
		}
		if executor.ToBool(val) {
			count++
		}
	}
	return count, nil
}

func fnSumWhere(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok {
		return 0.0, nil
	}
	sum := 0.0
	for _, item := range arr {
		val, err := executor.CallLambda(context.Background(), args[1], []any{item}, time.Now(), 0)
		if err != nil {
			return nil, fmt.Errorf("sum_where: %w", err)
		}
		if executor.ToBool(val) {
			sum += executor.ToFloat(item)
		}
	}
	return sum, nil
}

// --- New Collection Functions ---

func fnFind(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok {
		return nil, nil
	}
	for _, item := range arr {
		val, err := executor.CallLambda(context.Background(), args[1], []any{item}, time.Now(), 0)
		if err != nil {
			return nil, fmt.Errorf("find: %w", err)
		}
		if executor.ToBool(val) {
			return item, nil
		}
	}
	return nil, nil
}

func fnFindIndex(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok {
		return int64(-1), nil
	}
	for i, item := range arr {
		val, err := executor.CallLambda(context.Background(), args[1], []any{item}, time.Now(), 0)
		if err != nil {
			return nil, fmt.Errorf("find_index: %w", err)
		}
		if executor.ToBool(val) {
			return int64(i), nil
		}
	}
	return int64(-1), nil
}

func fnIncludes(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok {
		return false, nil
	}
	target := fmt.Sprintf("%v", args[1])
	for _, item := range arr {
		if fmt.Sprintf("%v", item) == target {
			return true, nil
		}
	}
	return false, nil
}

func fnEvery(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok {
		return false, nil
	}
	for _, item := range arr {
		val, err := executor.CallLambda(context.Background(), args[1], []any{item}, time.Now(), 0)
		if err != nil {
			return nil, fmt.Errorf("every: %w", err)
		}
		if !executor.ToBool(val) {
			return false, nil
		}
	}
	return true, nil
}

func fnSome(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok {
		return false, nil
	}
	for _, item := range arr {
		val, err := executor.CallLambda(context.Background(), args[1], []any{item}, time.Now(), 0)
		if err != nil {
			return nil, fmt.Errorf("some: %w", err)
		}
		if executor.ToBool(val) {
			return true, nil
		}
	}
	return false, nil
}

func fnReverseArr(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok {
		return nil, fmt.Errorf("reverse: expected array")
	}
	result := make([]any, len(arr))
	for i, v := range arr {
		result[len(arr)-1-i] = v
	}
	return result, nil
}

func fnSeq(args []any) (any, error) {
	var start, end, step int64
	switch len(args) {
	case 1:
		start = 0
		end = executor.ToInt(args[0])
		step = 1
	case 2:
		start = executor.ToInt(args[0])
		end = executor.ToInt(args[1])
		step = 1
		if start > end {
			step = -1
		}
	default:
		start = executor.ToInt(args[0])
		end = executor.ToInt(args[1])
		step = executor.ToInt(args[2])
	}

	if step == 0 {
		return []any{}, nil
	}

	result := make([]any, 0)
	if step > 0 {
		for i := start; i < end; i += step {
			result = append(result, i)
			if len(result) > 10000 {
				break // safety limit
			}
		}
	} else {
		for i := start; i > end; i += step {
			result = append(result, i)
			if len(result) > 10000 {
				break
			}
		}
	}
	return result, nil
}

func fnTakeWhile(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok {
		return []any{}, nil
	}
	result := make([]any, 0)
	for _, item := range arr {
		val, err := executor.CallLambda(context.Background(), args[1], []any{item}, time.Now(), 0)
		if err != nil {
			return nil, fmt.Errorf("take_while: %w", err)
		}
		if !executor.ToBool(val) {
			break
		}
		result = append(result, item)
	}
	return result, nil
}

func fnDropWhile(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok {
		return []any{}, nil
	}
	dropping := true
	result := make([]any, 0)
	for _, item := range arr {
		if dropping {
			val, err := executor.CallLambda(context.Background(), args[1], []any{item}, time.Now(), 0)
			if err != nil {
				return nil, fmt.Errorf("drop_while: %w", err)
			}
			if executor.ToBool(val) {
				continue
			}
			dropping = false
		}
		result = append(result, item)
	}
	return result, nil
}

func fnDistinctBy(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok {
		return nil, fmt.Errorf("distinct_by: first argument must be an array")
	}
	seen := make(map[string]bool)
	result := make([]any, 0, len(arr))
	for _, item := range arr {
		key, err := executor.CallLambda(context.Background(), args[1], []any{item}, time.Now(), 0)
		if err != nil {
			return nil, fmt.Errorf("distinct_by: %w", err)
		}
		k := fmt.Sprintf("%v", key)
		if !seen[k] {
			seen[k] = true
			result = append(result, item)
		}
	}
	return result, nil
}

// --- Helpers ---

func getField(obj any, key string) any {
	if m, ok := obj.(map[string]any); ok {
		return m[key]
	}
	return nil
}

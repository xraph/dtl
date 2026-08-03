package stdlib

import (
	"context"
	"fmt"
	"time"

	"github.com/xraph/dtl/executor"
)

// Collection operations filling out the surface against jq, JSONata and CEL.
//
// The *_by family takes a lambda rather than a key name, unlike the older
// sort_by and group_by which take a string key. That is deliberate: a lambda
// covers the key-name case (r => r.field) and also computed keys, which a
// string cannot express. The existing string-key functions stay as they are.
func registerCollectionsExtra(m map[string]*executor.BuiltinFunc) {
	register(m, "flat_map", 2, 2, fnFlatMap,
		"flat_map(arr, fn) -> array -- Applies fn to each element and concatenates the resulting arrays")
	register(m, "partition", 2, 2, fnPartition,
		"partition(arr, fn) -> array -- Splits into [matching, not_matching] in one pass")
	register(m, "index_by", 2, 2, fnIndexBy,
		"index_by(arr, fn) -> object -- Keys each element by fn's result. Later elements win on collision")
	register(m, "count_by", 2, 2, fnCountBy,
		"count_by(arr, fn) -> object -- Counts elements per fn result")
	register(m, "sum_by", 2, 2, fnSumBy,
		"sum_by(arr, fn) -> float -- Sums fn's result over every element")
	register(m, "avg_by", 2, 2, fnAvgBy,
		"avg_by(arr, fn) -> float -- Averages fn's result over every element; 0 when empty")
	register(m, "min_by", 2, 2, fnMinBy,
		"min_by(arr, fn) -> any -- Element with the smallest fn result, or null when empty")
	register(m, "max_by", 2, 2, fnMaxBy,
		"max_by(arr, fn) -> any -- Element with the largest fn result, or null when empty")
	register(m, "compact", 1, 1, fnCompact,
		"compact(arr) -> array -- Removes null elements")
	register(m, "slice", 2, 3, fnSlice,
		"slice(arr, start, end?) -> array -- Elements from start up to but excluding end. Negative indices count from the end")
	register(m, "concat", 1, -1, fnConcat,
		"concat(a, b, ...) -> array -- Concatenates arrays end to end")
	register(m, "intersection", 2, 2, fnIntersection,
		"intersection(a, b) -> array -- Elements present in both, order following a, without duplicates")
	register(m, "union", 2, 2, fnUnion,
		"union(a, b) -> array -- Elements present in either, in first-seen order, without duplicates")
	register(m, "difference", 2, 2, fnDifference,
		"difference(a, b) -> array -- Elements of a that are not in b, without duplicates")
	register(m, "windows", 2, 2, fnWindows,
		"windows(arr, size) -> array -- Overlapping consecutive windows of the given size")
	register(m, "pluck", 2, 2, fnPluck,
		"pluck(arr, key) -> array -- Collects one key from every object, skipping elements that lack it")
	register(m, "unzip", 1, 1, fnUnzip,
		"unzip(arr) -> array -- Turns an array of pairs back into [firsts, seconds]. Inverse of zip")
}

// callLambda invokes a DTL lambda, labelling failures with the calling builtin
// so an error inside a callback points at the function the author called.
func callLambda(fnName string, lambda any, arg any) (any, error) {
	v, err := executor.CallLambda(context.Background(), lambda, []any{arg}, time.Time{}, 0)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fnName, err)
	}
	return v, nil
}

func arrayArg(fnName string, v any) ([]any, error) {
	arr, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("%s: first argument must be an array", fnName)
	}
	return arr, nil
}

func fnFlatMap(args []any) (any, error) {
	arr, err := arrayArg("flat_map", args[0])
	if err != nil {
		return nil, err
	}
	result := make([]any, 0, len(arr))
	for _, item := range arr {
		val, err := callLambda("flat_map", args[1], item)
		if err != nil {
			return nil, err
		}
		// A non-array result is appended as a single element rather than
		// dropped, so a callback that sometimes returns a bare value still
		// behaves sensibly.
		if nested, ok := val.([]any); ok {
			result = append(result, nested...)
			continue
		}
		result = append(result, val)
	}
	return result, nil
}

// fnPartition returns both halves from one traversal. Doing it as two filters
// runs the predicate twice per element, which matters when the predicate is a
// DTL lambda rather than a Go function.
func fnPartition(args []any) (any, error) {
	arr, err := arrayArg("partition", args[0])
	if err != nil {
		return nil, err
	}
	matching, rest := make([]any, 0), make([]any, 0)
	for _, item := range arr {
		val, err := callLambda("partition", args[1], item)
		if err != nil {
			return nil, err
		}
		// executor.ToBool, not a bool type assertion, so partition and filter
		// agree about what a predicate returning a non-bool means.
		if executor.ToBool(val) {
			matching = append(matching, item)
			continue
		}
		rest = append(rest, item)
	}
	return []any{matching, rest}, nil
}

func fnIndexBy(args []any) (any, error) {
	arr, err := arrayArg("index_by", args[0])
	if err != nil {
		return nil, err
	}
	result := make(map[string]any, len(arr))
	for _, item := range arr {
		key, err := callLambda("index_by", args[1], item)
		if err != nil {
			return nil, err
		}
		result[executor.ToString(key)] = item
	}
	return result, nil
}

func fnCountBy(args []any) (any, error) {
	arr, err := arrayArg("count_by", args[0])
	if err != nil {
		return nil, err
	}
	result := make(map[string]any)
	for _, item := range arr {
		key, err := callLambda("count_by", args[1], item)
		if err != nil {
			return nil, err
		}
		k := executor.ToString(key)
		prev, _ := result[k].(int64)
		result[k] = prev + 1
	}
	return result, nil
}

func fnSumBy(args []any) (any, error) {
	arr, err := arrayArg("sum_by", args[0])
	if err != nil {
		return nil, err
	}
	total := 0.0
	for _, item := range arr {
		val, err := callLambda("sum_by", args[1], item)
		if err != nil {
			return nil, err
		}
		total += executor.ToFloat(val)
	}
	return total, nil
}

func fnAvgBy(args []any) (any, error) {
	arr, err := arrayArg("avg_by", args[0])
	if err != nil {
		return nil, err
	}
	if len(arr) == 0 {
		return 0.0, nil
	}
	sum, err := fnSumBy(args)
	if err != nil {
		return nil, err
	}
	return executor.ToFloat(sum) / float64(len(arr)), nil
}

// fnMinBy and fnMaxBy return the element, not the projected value, which is
// what makes them different from min(map(arr, fn)).
func fnMinBy(args []any) (any, error) {
	return extremeBy("min_by", args, func(candidate, best float64) bool {
		return candidate < best
	})
}

func fnMaxBy(args []any) (any, error) {
	return extremeBy("max_by", args, func(candidate, best float64) bool {
		return candidate > best
	})
}

func extremeBy(fnName string, args []any, better func(candidate, best float64) bool) (any, error) {
	arr, err := arrayArg(fnName, args[0])
	if err != nil {
		return nil, err
	}
	if len(arr) == 0 {
		return nil, nil
	}

	var bestItem any
	var bestScore float64
	for i, item := range arr {
		val, err := callLambda(fnName, args[1], item)
		if err != nil {
			return nil, err
		}
		score := executor.ToFloat(val)
		if i == 0 || better(score, bestScore) {
			bestItem, bestScore = item, score
		}
	}
	return bestItem, nil
}

func fnCompact(args []any) (any, error) {
	arr, err := arrayArg("compact", args[0])
	if err != nil {
		return nil, err
	}
	result := make([]any, 0, len(arr))
	for _, item := range arr {
		if item != nil {
			result = append(result, item)
		}
	}
	return result, nil
}

// fnSlice accepts negative indices, counting from the end, which is what makes
// slice(arr, -3) a readable way to say "the last three".
func fnSlice(args []any) (any, error) {
	arr, err := arrayArg("slice", args[0])
	if err != nil {
		return nil, err
	}
	n := len(arr)

	start := resolveIndex(int(executor.ToInt(args[1])), n)
	end := n
	if len(args) > 2 {
		end = resolveIndex(int(executor.ToInt(args[2])), n)
	}
	if end < start {
		return []any{}, nil
	}

	out := make([]any, end-start)
	copy(out, arr[start:end])
	return out, nil
}

// resolveIndex maps a possibly-negative index into a clamped absolute position.
func resolveIndex(i, length int) int {
	if i < 0 {
		i += length
	}
	if i < 0 {
		return 0
	}
	if i > length {
		return length
	}
	return i
}

func fnConcat(args []any) (any, error) {
	result := make([]any, 0)
	for i, arg := range args {
		arr, ok := arg.([]any)
		if !ok {
			return nil, fmt.Errorf("concat: argument %d must be an array", i+1)
		}
		result = append(result, arr...)
	}
	return result, nil
}

// Set operations compare by the same string projection unique() uses, so they
// agree with it about what counts as a duplicate.
func setKey(v any) string {
	return fmt.Sprintf("%T:%v", v, v)
}

func fnIntersection(args []any) (any, error) {
	a, err := arrayArg("intersection", args[0])
	if err != nil {
		return nil, err
	}
	b, ok := args[1].([]any)
	if !ok {
		return nil, fmt.Errorf("intersection: second argument must be an array")
	}

	inB := make(map[string]bool, len(b))
	for _, v := range b {
		inB[setKey(v)] = true
	}

	seen := make(map[string]bool, len(a))
	result := make([]any, 0)
	for _, v := range a {
		k := setKey(v)
		if inB[k] && !seen[k] {
			seen[k] = true
			result = append(result, v)
		}
	}
	return result, nil
}

func fnUnion(args []any) (any, error) {
	a, err := arrayArg("union", args[0])
	if err != nil {
		return nil, err
	}
	b, ok := args[1].([]any)
	if !ok {
		return nil, fmt.Errorf("union: second argument must be an array")
	}

	seen := make(map[string]bool, len(a)+len(b))
	result := make([]any, 0, len(a)+len(b))
	for _, v := range append(append([]any{}, a...), b...) {
		k := setKey(v)
		if !seen[k] {
			seen[k] = true
			result = append(result, v)
		}
	}
	return result, nil
}

func fnDifference(args []any) (any, error) {
	a, err := arrayArg("difference", args[0])
	if err != nil {
		return nil, err
	}
	b, ok := args[1].([]any)
	if !ok {
		return nil, fmt.Errorf("difference: second argument must be an array")
	}

	inB := make(map[string]bool, len(b))
	for _, v := range b {
		inB[setKey(v)] = true
	}

	seen := make(map[string]bool, len(a))
	result := make([]any, 0)
	for _, v := range a {
		k := setKey(v)
		if !inB[k] && !seen[k] {
			seen[k] = true
			result = append(result, v)
		}
	}
	return result, nil
}

// fnWindows produces overlapping runs, unlike chunk which produces disjoint
// ones. A window larger than the array yields nothing rather than one short
// window, since a partial window would break the size guarantee callers rely
// on when computing rolling statistics.
func fnWindows(args []any) (any, error) {
	arr, err := arrayArg("windows", args[0])
	if err != nil {
		return nil, err
	}
	size := int(executor.ToInt(args[1]))
	if size <= 0 || size > len(arr) {
		return []any{}, nil
	}

	result := make([]any, 0, len(arr)-size+1)
	for i := 0; i+size <= len(arr); i++ {
		window := make([]any, size)
		copy(window, arr[i:i+size])
		result = append(result, window)
	}
	return result, nil
}

// fnPluck skips elements that lack the key rather than emitting null for them,
// so the result is the values that exist. compact is there for callers who want
// nulls dropped from an array they already have.
func fnPluck(args []any) (any, error) {
	arr, err := arrayArg("pluck", args[0])
	if err != nil {
		return nil, err
	}
	key := executor.ToString(args[1])

	result := make([]any, 0, len(arr))
	for _, item := range arr {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if v, exists := obj[key]; exists {
			result = append(result, v)
		}
	}
	return result, nil
}

func fnUnzip(args []any) (any, error) {
	arr, err := arrayArg("unzip", args[0])
	if err != nil {
		return nil, err
	}

	firsts, seconds := make([]any, 0, len(arr)), make([]any, 0, len(arr))
	for _, item := range arr {
		pair, ok := item.([]any)
		if !ok || len(pair) < 2 {
			continue
		}
		firsts = append(firsts, pair[0])
		seconds = append(seconds, pair[1])
	}
	return []any{firsts, seconds}, nil
}

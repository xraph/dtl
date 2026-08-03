package stdlib

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/xraph/dtl/executor"
)

func registerObjects(m map[string]*executor.BuiltinFunc) {
	register(m, "keys", 1, 1, fnKeys,
		"keys(obj) -> string[] -- Object keys, sorted")
	register(m, "values", 1, 1, fnValues,
		"values(obj) -> array -- Object values, ordered by sorted key")
	register(m, "entries", 1, 1, fnEntries,
		"entries(obj) -> array -- Object as [{key, value}] pairs, ordered by sorted key")
	register(m, "merge", 2, -1, fnMerge,
		"merge(a, b, ...) -> object -- Shallow merge; later values win. Errors when an argument is not an object")
	register(m, "pick", 2, 2, fnPick,
		"pick(obj, keys) -> object -- Keeps only the listed keys")
	register(m, "omit", 2, 2, fnOmit,
		"omit(obj, keys) -> object -- Removes the listed keys")
	register(m, "has_key", 2, 2, fnHasKey,
		"has_key(obj, key) -> bool -- Whether the key is present")
	register(m, "deep_merge", 2, -1, fnDeepMerge,
		"deep_merge(a, b, ...) -> object -- Recursive merge; later values win. Nested objects merge, arrays are replaced")
	register(m, "map_values", 2, 2, fnMapValues,
		"map_values(obj, fn) -> object -- Applies fn to every value, keeping the keys")
	register(m, "map_keys", 2, 2, fnMapKeys,
		"map_keys(obj, fn) -> object -- Applies fn to every key, keeping the values")
	register(m, "from_entries", 1, 1, fnFromEntries,
		"from_entries(arr) -> object -- Builds an object from [{key, value}] pairs. Inverse of entries")
	register(m, "invert", 1, 1, fnInvert,
		"invert(obj) -> object -- Swaps keys and values. Later keys win when values collide")
}

func fnKeys(args []any) (any, error) {
	obj, ok := args[0].(map[string]any)
	if !ok {
		return []any{}, nil
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	result := make([]any, len(keys))
	for i, k := range keys {
		result[i] = k
	}
	return result, nil
}

func fnValues(args []any) (any, error) {
	obj, ok := args[0].(map[string]any)
	if !ok {
		return []any{}, nil
	}
	// Sort keys for deterministic order
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	result := make([]any, len(keys))
	for i, k := range keys {
		result[i] = obj[k]
	}
	return result, nil
}

func fnEntries(args []any) (any, error) {
	obj, ok := args[0].(map[string]any)
	if !ok {
		return []any{}, nil
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	result := make([]any, len(keys))
	for i, k := range keys {
		result[i] = map[string]any{"key": k, "value": obj[k]}
	}
	return result, nil
}

func fnMerge(args []any) (any, error) {
	result := make(map[string]any)
	for _, arg := range args {
		obj, ok := arg.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("merge: all arguments must be objects")
		}
		for k, v := range obj {
			result[k] = v
		}
	}
	return result, nil
}

func fnPick(args []any) (any, error) {
	obj, ok := args[0].(map[string]any)
	if !ok {
		return map[string]any{}, nil
	}
	keyArr, ok := args[1].([]any)
	if !ok {
		return map[string]any{}, nil
	}
	result := make(map[string]any, len(keyArr))
	for _, k := range keyArr {
		key := executor.ToString(k)
		if v, exists := obj[key]; exists {
			result[key] = v
		}
	}
	return result, nil
}

func fnOmit(args []any) (any, error) {
	obj, ok := args[0].(map[string]any)
	if !ok {
		return map[string]any{}, nil
	}
	keyArr, ok := args[1].([]any)
	if !ok {
		// If not array, return copy of original
		result := make(map[string]any, len(obj))
		for k, v := range obj {
			result[k] = v
		}
		return result, nil
	}
	exclude := make(map[string]bool, len(keyArr))
	for _, k := range keyArr {
		exclude[executor.ToString(k)] = true
	}
	result := make(map[string]any)
	for k, v := range obj {
		if !exclude[k] {
			result[k] = v
		}
	}
	return result, nil
}

// fnDeepMerge merges recursively, unlike merge which overwrites at the top
// level. Arrays are replaced rather than concatenated: concatenating turns a
// merge into an append, which is almost never what a caller layering defaults
// under overrides wants, and an explicit concat is available when it is.
func fnDeepMerge(args []any) (any, error) {
	result := map[string]any{}
	for _, arg := range args {
		obj, ok := arg.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("deep_merge: all arguments must be objects")
		}
		result = mergeInto(result, obj)
	}
	return result, nil
}

func mergeInto(dst, src map[string]any) map[string]any {
	out := make(map[string]any, len(dst)+len(src))
	for k, v := range dst {
		out[k] = v
	}
	for k, v := range src {
		existing, hasExisting := out[k]
		if !hasExisting {
			out[k] = v
			continue
		}
		exObj, exOK := existing.(map[string]any)
		newObj, newOK := v.(map[string]any)
		if exOK && newOK {
			out[k] = mergeInto(exObj, newObj)
			continue
		}
		out[k] = v
	}
	return out
}

func fnMapValues(args []any) (any, error) {
	obj, ok := args[0].(map[string]any)
	if !ok {
		return map[string]any{}, nil
	}
	// Sorted so the callback runs in a stable order. Map iteration order is
	// random in Go, and a callback with an observable effect — DEBUG, say —
	// would otherwise behave differently between identical runs.
	keys := sortedKeys(obj)
	result := make(map[string]any, len(obj))
	for _, k := range keys {
		val, err := executor.CallLambda(context.Background(), args[1], []any{obj[k]}, time.Time{}, 0)
		if err != nil {
			return nil, fmt.Errorf("map_values: %w", err)
		}
		result[k] = val
	}
	return result, nil
}

func fnMapKeys(args []any) (any, error) {
	obj, ok := args[0].(map[string]any)
	if !ok {
		return map[string]any{}, nil
	}
	keys := sortedKeys(obj)
	result := make(map[string]any, len(obj))
	for _, k := range keys {
		val, err := executor.CallLambda(context.Background(), args[1], []any{k}, time.Time{}, 0)
		if err != nil {
			return nil, fmt.Errorf("map_keys: %w", err)
		}
		result[executor.ToString(val)] = obj[k]
	}
	return result, nil
}

// fnFromEntries accepts what entries produces: [{key, value}] objects. Entries
// missing a key are skipped rather than producing an "" key, since a pipeline
// that filtered entries down should not gain a blank one.
func fnFromEntries(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok {
		return map[string]any{}, nil
	}
	result := make(map[string]any, len(arr))
	for _, item := range arr {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		k, hasKey := entry["key"]
		if !hasKey {
			continue
		}
		result[executor.ToString(k)] = entry["value"]
	}
	return result, nil
}

func fnInvert(args []any) (any, error) {
	obj, ok := args[0].(map[string]any)
	if !ok {
		return map[string]any{}, nil
	}
	// Sorted so that when two keys share a value, which one wins is stable
	// rather than dependent on map iteration order.
	result := make(map[string]any, len(obj))
	for _, k := range sortedKeys(obj) {
		result[executor.ToString(obj[k])] = k
	}
	return result, nil
}

// sortedKeys returns an object's keys in sorted order. Several functions here
// depend on a deterministic traversal, so the sort lives in one place.
func sortedKeys(obj map[string]any) []string {
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func fnHasKey(args []any) (any, error) {
	obj, ok := args[0].(map[string]any)
	if !ok {
		return false, nil
	}
	key := executor.ToString(args[1])
	_, exists := obj[key]
	return exists, nil
}

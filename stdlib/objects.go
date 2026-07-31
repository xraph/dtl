package stdlib

import (
	"fmt"
	"sort"

	"github.com/xraph/dtl/executor"
)

func registerObjects(m map[string]*executor.BuiltinFunc) {
	register(m, "keys", 1, 1, fnKeys)
	register(m, "values", 1, 1, fnValues)
	register(m, "entries", 1, 1, fnEntries)
	register(m, "merge", 2, -1, fnMerge)
	register(m, "pick", 2, 2, fnPick)
	register(m, "omit", 2, 2, fnOmit)
	register(m, "has_key", 2, 2, fnHasKey)
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

func fnHasKey(args []any) (any, error) {
	obj, ok := args[0].(map[string]any)
	if !ok {
		return false, nil
	}
	key := executor.ToString(args[1])
	_, exists := obj[key]
	return exists, nil
}

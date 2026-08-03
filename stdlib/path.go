package stdlib

import (
	"sort"
	"strconv"
	"strings"

	"github.com/xraph/dtl/executor"
)

// Nested access by dotted path. Reshaping nested data is the language's whole
// job, and without these every level of a path costs an explicit guard.
//
// Path syntax is a plain dot-separated split. A segment that parses as a
// non-negative integer indexes an array, so "items.0.name" walks through one.
// There is no escape mechanism: a key that itself contains a dot cannot be
// addressed. That is a deliberate limit rather than an oversight — supporting
// it means a path grammar with quoting and bracket forms, and the data that
// needs it is rare enough that keeping the split trivial is the better trade.
// Such keys stay reachable through has_key, pick, and ordinary indexing.
func registerPath(m map[string]*executor.BuiltinFunc) {
	register(m, "path::get", 2, 3, fnPathGet,
		"path::get(obj, path, default?) -> any -- Value at a dotted path, else default, else null. Numeric segments index arrays")
	register(m, "path::has", 2, 2, fnPathHas,
		"path::has(obj, path) -> bool -- Whether the dotted path resolves to a value")
	register(m, "path::set", 3, 3, fnPathSet,
		"path::set(obj, path, value) -> object -- Copy of obj with the dotted path set, creating intermediate objects as needed")
	register(m, "path::delete", 2, 2, fnPathDelete,
		"path::delete(obj, path) -> object -- Copy of obj with the dotted path removed")
	register(m, "path::flatten", 1, 1, fnPathFlatten,
		"path::flatten(obj) -> object -- Flattens nested objects into a single level keyed by dotted path")
}

// splitPath breaks a dotted path into segments. An empty path yields no
// segments, which callers treat as addressing the container itself.
func splitPath(p string) []string {
	if p == "" {
		return nil
	}
	return strings.Split(p, ".")
}

// descend resolves one segment against one container, reporting whether the
// segment addressed anything at all. The bool matters: a path that resolves to
// a stored null is not the same as a path that does not resolve, and path::has
// and path::get's default both depend on telling them apart.
func descend(current any, seg string) (any, bool) {
	switch c := current.(type) {
	case map[string]any:
		v, ok := c[seg]
		return v, ok
	case []any:
		i, err := strconv.Atoi(seg)
		if err != nil || i < 0 || i >= len(c) {
			return nil, false
		}
		return c[i], true
	default:
		return nil, false
	}
}

func fnPathGet(args []any) (any, error) {
	var fallback any
	if len(args) > 2 {
		fallback = args[2]
	}

	current := args[0]
	for _, seg := range splitPath(executor.ToString(args[1])) {
		v, ok := descend(current, seg)
		if !ok {
			return fallback, nil
		}
		current = v
	}
	return current, nil
}

func fnPathHas(args []any) (any, error) {
	current := args[0]
	for _, seg := range splitPath(executor.ToString(args[1])) {
		v, ok := descend(current, seg)
		if !ok {
			return false, nil
		}
		current = v
	}
	return true, nil
}

// shallowCopy duplicates one level of a container. path::set and path::delete
// copy each container along the path and share everything off it, so the input
// is never mutated while untouched branches are not deep-copied either.
func shallowCopy(v any) any {
	switch c := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(c))
		for k, val := range c {
			out[k] = val
		}
		return out
	case []any:
		out := make([]any, len(c))
		copy(out, c)
		return out
	default:
		return v
	}
}

func fnPathSet(args []any) (any, error) {
	segs := splitPath(executor.ToString(args[1]))
	if len(segs) == 0 {
		return args[2], nil
	}
	return setPath(args[0], segs, args[2]), nil
}

// setPath rebuilds the containers along the path, replacing anything that is
// not a container with a fresh object. Writing "a.b" into a value where "a" is
// a number has to do something, and creating the structure the caller clearly
// intended beats failing or silently doing nothing.
func setPath(current any, segs []string, value any) any {
	seg := segs[0]

	if arr, ok := current.([]any); ok {
		i, err := strconv.Atoi(seg)
		if err == nil && i >= 0 && i < len(arr) {
			out, _ := shallowCopy(arr).([]any)
			if len(segs) == 1 {
				out[i] = value
			} else {
				out[i] = setPath(arr[i], segs[1:], value)
			}
			return out
		}
		// An out-of-range or non-numeric segment cannot address this array.
		// Fall through and replace it with an object, matching the behaviour
		// for any other non-addressable value.
	}

	obj, ok := current.(map[string]any)
	if !ok {
		obj = map[string]any{}
	}
	out, _ := shallowCopy(obj).(map[string]any)
	if len(segs) == 1 {
		out[seg] = value
	} else {
		out[seg] = setPath(obj[seg], segs[1:], value)
	}
	return out
}

func fnPathDelete(args []any) (any, error) {
	segs := splitPath(executor.ToString(args[1]))
	if len(segs) == 0 {
		return args[0], nil
	}
	return deletePath(args[0], segs), nil
}

func deletePath(current any, segs []string) any {
	seg := segs[0]

	switch c := current.(type) {
	case map[string]any:
		if _, exists := c[seg]; !exists {
			return current
		}
		out, _ := shallowCopy(c).(map[string]any)
		if len(segs) == 1 {
			delete(out, seg)
		} else {
			out[seg] = deletePath(c[seg], segs[1:])
		}
		return out

	case []any:
		i, err := strconv.Atoi(seg)
		if err != nil || i < 0 || i >= len(c) {
			return current
		}
		if len(segs) == 1 {
			// Removing an element shifts the ones after it, which is what a
			// caller deleting "items.0" means.
			out := make([]any, 0, len(c)-1)
			out = append(out, c[:i]...)
			out = append(out, c[i+1:]...)
			return out
		}
		out, _ := shallowCopy(c).([]any)
		out[i] = deletePath(c[i], segs[1:])
		return out

	default:
		return current
	}
}

func fnPathFlatten(args []any) (any, error) {
	out := map[string]any{}
	obj, ok := args[0].(map[string]any)
	if !ok {
		return out, nil
	}
	flattenInto(obj, "", out)
	return out, nil
}

// flattenInto walks nested objects only. Arrays are left as values rather than
// expanded into numeric segments, because flattening them loses the difference
// between a list and an object with numeric keys, and callers that want array
// paths can address them with path::get directly.
func flattenInto(obj map[string]any, prefix string, out map[string]any) {
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		full := k
		if prefix != "" {
			full = prefix + "." + k
		}
		if nested, ok := obj[k].(map[string]any); ok && len(nested) > 0 {
			flattenInto(nested, full, out)
			continue
		}
		out[full] = obj[k]
	}
}

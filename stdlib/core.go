package stdlib

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/xraph/dtl/executor"
)

func registerCore(m map[string]*executor.BuiltinFunc) {
	register(m, "len", 1, 1, fnLen,
		"len(x) -> int -- Length of a string, array, or object. Strings are measured in bytes")
	register(m, "type_of", 1, 1, fnTypeOf,
		"type_of(x) -> string -- Type name: 'null', 'bool', 'int', 'float', 'string', 'array', 'object', or 'datetime'")
	register(m, "is_null", 1, 1, fnIsNull,
		"is_null(x) -> bool -- True when x is null")
	register(m, "is_blank", 1, 1, fnIsBlank,
		"is_blank(x) -> bool -- True when x is null, an empty or whitespace-only string, or an empty array or object")
	register(m, "to_string", 1, 1, fnToString,
		"to_string(x) -> string -- String representation of any value")
	register(m, "abs", 1, 1, fnAbs,
		"abs(x) -> float -- Absolute value")
	register(m, "round", 1, 2, fnRound,
		"round(x, decimals?) -> float -- Rounds to the given number of decimal places (default 0)")
	register(m, "ceil", 1, 1, fnCeil,
		"ceil(x) -> float -- Rounds up to the nearest whole number")
	register(m, "floor", 1, 1, fnFloor,
		"floor(x) -> float -- Rounds down to the nearest whole number")
	register(m, "power", 2, 2, fnPower,
		"power(base, exp) -> float -- base raised to the power of exp")
	register(m, "sqrt", 1, 1, fnSqrt,
		"sqrt(x) -> float -- Square root. Errors on negative input")
	register(m, "log", 1, 1, fnLog,
		"log(x) -> float -- Natural logarithm. Errors on non-positive input")
	register(m, "log10", 1, 1, fnLog10,
		"log10(x) -> float -- Base-10 logarithm. Errors on non-positive input")
	register(m, "now", 0, 0, fnNow,
		"now() -> datetime -- Current UTC date and time")
	register(m, "today", 0, 0, fnToday,
		"today() -> datetime -- Today's date at UTC midnight")

	// DEBUG and PRINT are intercepted by the executor for output capture,
	// but registered here so the compiler recognizes them.
	register(m, "DEBUG", 1, -1, func(args []any) (any, error) {
		if len(args) > 0 {
			return args[len(args)-1], nil
		}
		return nil, nil
	}, "DEBUG(x, ...) -> any -- Records its arguments as debug output and returns the last one")
	register(m, "PRINT", 1, -1, func(args []any) (any, error) {
		if len(args) > 0 {
			return args[len(args)-1], nil
		}
		return nil, nil
	}, "PRINT(x, ...) -> any -- Records its arguments as output and returns the last one")
}

func fnLen(args []any) (any, error) {
	switch v := args[0].(type) {
	case string:
		return int64(len(v)), nil
	case []any:
		return int64(len(v)), nil
	case map[string]any:
		return int64(len(v)), nil
	case nil:
		return int64(0), nil
	default:
		return nil, fmt.Errorf("len: unsupported type %T", args[0])
	}
}

func fnTypeOf(args []any) (any, error) {
	switch args[0].(type) {
	case nil:
		return "null", nil
	case bool:
		return "bool", nil
	case int64:
		return "int", nil
	case float64:
		return "float", nil
	case string:
		return "string", nil
	case []any:
		return "array", nil
	case map[string]any:
		return "object", nil
	case time.Time:
		return "datetime", nil
	default:
		return "unknown", nil
	}
}

func fnIsNull(args []any) (any, error) {
	return args[0] == nil, nil
}

// fnIsBlank reports whether the argument is "blank": nil, an empty/whitespace
// string, or an empty array/object. Mirrors the client-side ISBLANK so
// authored conditions evaluate identically on both sides.
func fnIsBlank(args []any) (any, error) {
	switch v := args[0].(type) {
	case nil:
		return true, nil
	case string:
		return strings.TrimSpace(v) == "", nil
	case []any:
		return len(v) == 0, nil
	case map[string]any:
		return len(v) == 0, nil
	default:
		return false, nil
	}
}

func fnToString(args []any) (any, error) {
	return executor.ToString(args[0]), nil
}

func fnAbs(args []any) (any, error) {
	v := executor.ToFloat(args[0])
	return math.Abs(v), nil
}

func fnRound(args []any) (any, error) {
	v := executor.ToFloat(args[0])
	decimals := 0
	if len(args) > 1 {
		decimals = int(executor.ToInt(args[1]))
	}
	pow := math.Pow(10, float64(decimals))
	return math.Round(v*pow) / pow, nil
}

func fnCeil(args []any) (any, error) {
	return math.Ceil(executor.ToFloat(args[0])), nil
}

func fnFloor(args []any) (any, error) {
	return math.Floor(executor.ToFloat(args[0])), nil
}

func fnPower(args []any) (any, error) {
	base := executor.ToFloat(args[0])
	exp := executor.ToFloat(args[1])
	return math.Pow(base, exp), nil
}

func fnSqrt(args []any) (any, error) {
	v := executor.ToFloat(args[0])
	if v < 0 {
		return nil, fmt.Errorf("sqrt: negative input")
	}
	return math.Sqrt(v), nil
}

func fnLog(args []any) (any, error) {
	v := executor.ToFloat(args[0])
	if v <= 0 {
		return nil, fmt.Errorf("log: non-positive input")
	}
	return math.Log(v), nil
}

func fnLog10(args []any) (any, error) {
	v := executor.ToFloat(args[0])
	if v <= 0 {
		return nil, fmt.Errorf("log10: non-positive input")
	}
	return math.Log10(v), nil
}

func fnNow(args []any) (any, error) {
	_ = args
	return time.Now().UTC(), nil
}

func fnToday(args []any) (any, error) {
	_ = args
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), nil
}

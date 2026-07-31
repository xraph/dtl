package stdlib

import (
	"fmt"
	"strconv"
	"time"

	"github.com/xraph/dtl/executor"
)

func registerCasting(m map[string]*executor.BuiltinFunc) {
	register(m, "as_float", 1, 1, fnAsFloat)
	register(m, "as_int", 1, 1, fnAsInt)
	register(m, "as_string", 1, 1, fnAsString)
	register(m, "as_bool", 1, 1, fnAsBool)
	register(m, "as_datetime", 1, 2, fnAsDatetime)

	// Friendly aliases (non-devs expect "to" prefix)
	register(m, "to_int", 1, 1, fnAsInt)
	register(m, "to_float", 1, 1, fnAsFloat)
	register(m, "to_bool", 1, 1, fnAsBool)
	register(m, "to_date", 1, 2, fnAsDatetime)
	register(m, "to_datetime", 1, 2, fnAsDatetime)
}

func fnAsFloat(args []any) (any, error) {
	switch v := args[0].(type) {
	case float64:
		return v, nil
	case int64:
		return float64(v), nil
	case string:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, fmt.Errorf("as_float: cannot convert %q to float", v)
		}
		return f, nil
	case bool:
		if v {
			return 1.0, nil
		}
		return 0.0, nil
	case nil:
		return 0.0, nil
	default:
		return executor.ToFloat(args[0]), nil
	}
}

func fnAsInt(args []any) (any, error) {
	switch v := args[0].(type) {
	case int64:
		return v, nil
	case float64:
		return int64(v), nil
	case string:
		i, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			// Try parsing as float first
			f, ferr := strconv.ParseFloat(v, 64)
			if ferr != nil {
				return nil, fmt.Errorf("as_int: cannot convert %q to int", v)
			}
			return int64(f), nil
		}
		return i, nil
	case bool:
		if v {
			return int64(1), nil
		}
		return int64(0), nil
	case nil:
		return int64(0), nil
	default:
		return executor.ToInt(args[0]), nil
	}
}

func fnAsString(args []any) (any, error) {
	return executor.ToString(args[0]), nil
}

func fnAsBool(args []any) (any, error) {
	return executor.ToBool(args[0]), nil
}

func fnAsDatetime(args []any) (any, error) {
	switch v := args[0].(type) {
	case time.Time:
		return v, nil
	case string:
		// Try standard formats, plus custom format if provided
		layouts := []string{
			time.RFC3339,
			"2006-01-02T15:04:05",
			"2006-01-02",
		}
		if len(args) > 1 {
			custom := executor.ToString(args[1])
			goLayout := convertDateFormatForCasting(custom)
			layouts = append([]string{goLayout}, layouts...)
		}
		for _, layout := range layouts {
			if t, err := time.Parse(layout, v); err == nil {
				return t, nil
			}
		}
		return nil, fmt.Errorf("as_datetime: cannot parse %q as datetime", v)
	default:
		return nil, fmt.Errorf("as_datetime: unsupported type %T", args[0])
	}
}

func convertDateFormatForCasting(format string) string {
	return convertDateFormat(format)
}

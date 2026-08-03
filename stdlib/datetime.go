package stdlib

import (
	"fmt"
	"strings"
	"time"

	"github.com/xraph/dtl/executor"
)

func registerDatetime(m map[string]*executor.BuiltinFunc) {
	register(m, "dt_add", 3, 3, fnDtAdd,
		"dt_add(dt, amount, unit) -> datetime -- Adds amount of unit ('seconds', 'minutes', 'hours', 'days', 'weeks')")
	register(m, "dt_subtract", 3, 3, fnDtSubtract,
		"dt_subtract(dt, amount, unit) -> datetime -- Subtracts amount of unit ('seconds', 'minutes', 'hours', 'days', 'weeks')")
	register(m, "diff", 3, 3, fnDiff,
		"diff(from, to, unit) -> int -- Whole units from `from` to `to`; negative when `to` precedes `from`")
	register(m, "dt_format", 2, 2, fnDtFormat,
		"dt_format(dt, format) -> string -- Formats using YYYY, MM, DD, HH, mm, ss tokens")
	register(m, "year", 1, 1, fnYear,
		"year(dt) -> int -- Calendar year")
	register(m, "month", 1, 1, fnMonth,
		"month(dt) -> int -- Month, 1-12")
	register(m, "day", 1, 1, fnDay,
		"day(dt) -> int -- Day of the month, 1-31")
	register(m, "hour", 1, 1, fnHour,
		"hour(dt) -> int -- Hour, 0-23")
	register(m, "minute", 1, 1, fnMinute,
		"minute(dt) -> int -- Minute, 0-59")
	register(m, "second", 1, 1, fnSecond,
		"second(dt) -> int -- Second, 0-59")
	register(m, "day_of_week", 1, 1, fnDayOfWeek,
		"day_of_week(dt) -> int -- Day of the week, 0 for Sunday through 6 for Saturday")
	register(m, "start_of", 2, 2, fnStartOf,
		"start_of(dt, unit) -> datetime -- Truncates down to the start of the 'minute', 'hour', 'day', 'week', 'month', or 'year'")
	register(m, "end_of", 2, 2, fnEndOf,
		"end_of(dt, unit) -> datetime -- Last instant of the 'minute', 'hour', 'day', 'week', 'month', or 'year'")
	register(m, "business_days_between", 2, 2, fnBusinessDaysBetween,
		"business_days_between(from, to) -> int -- Weekdays between two datetimes, excluding weekends")
	register(m, "is_business_day", 1, 1, fnIsBusinessDay,
		"is_business_day(dt) -> bool -- Whether the date falls Monday to Friday")
	register(m, "time_bucket", 2, 2, fnTimeBucket,
		"time_bucket(dt, unit) -> datetime -- Buckets a datetime down to the given unit. Same behaviour as start_of")

	// Pipe-friendly aliases: "add" and "subtract" are registered under the
	// namespaced form to avoid collision with arithmetic. Users pipe like:
	//   dt | dt_add(7, "days")
	alias(m, "system::datetime::add", "dt_add")
	alias(m, "system::datetime::subtract", "dt_subtract")
	alias(m, "system::datetime::diff", "diff")
	alias(m, "system::datetime::format", "dt_format")
	alias(m, "system::datetime::business_days_between", "business_days_between")
	alias(m, "system::datetime::is_business_day", "is_business_day")
	alias(m, "system::datetime::time_bucket", "time_bucket")
}

func toTime(v any) (time.Time, error) {
	switch t := v.(type) {
	case time.Time:
		return t, nil
	case string:
		for _, layout := range []string{
			time.RFC3339,
			"2006-01-02T15:04:05",
			"2006-01-02",
			"2006-01-02 15:04:05",
			"01/02/2006",
			"02/01/2006",
			"02-Jan-2006",
			"Jan 2, 2006",
			"January 2, 2006",
		} {
			if parsed, err := time.Parse(layout, t); err == nil {
				return parsed, nil
			}
		}
		return time.Time{}, fmt.Errorf("cannot parse datetime: %q", t)
	default:
		return time.Time{}, fmt.Errorf("expected datetime, got %T", v)
	}
}

func durationUnit(amount int, unit string) time.Duration {
	unit = normalizeUnit(unit)
	switch unit {
	case "second":
		return time.Duration(amount) * time.Second
	case "minute":
		return time.Duration(amount) * time.Minute
	case "hour":
		return time.Duration(amount) * time.Hour
	case "day":
		return time.Duration(amount) * 24 * time.Hour
	case "week":
		return time.Duration(amount) * 7 * 24 * time.Hour
	default:
		return time.Duration(amount) * time.Second
	}
}

func fnDtAdd(args []any) (any, error) {
	dt, err := toTime(args[0])
	if err != nil {
		return nil, err
	}
	amount := int(executor.ToInt(args[1]))
	unit := executor.ToString(args[2])

	unit = normalizeUnit(unit)
	switch unit {
	case "month":
		return dt.AddDate(0, amount, 0), nil
	case "year":
		return dt.AddDate(amount, 0, 0), nil
	default:
		return dt.Add(durationUnit(amount, unit)), nil
	}
}

func fnDtSubtract(args []any) (any, error) {
	dt, err := toTime(args[0])
	if err != nil {
		return nil, err
	}
	amount := int(executor.ToInt(args[1]))
	unit := executor.ToString(args[2])

	unit = normalizeUnit(unit)
	switch unit {
	case "month":
		return dt.AddDate(0, -amount, 0), nil
	case "year":
		return dt.AddDate(-amount, 0, 0), nil
	default:
		return dt.Add(-durationUnit(amount, unit)), nil
	}
}

func fnDiff(args []any) (any, error) {
	dt1, err := toTime(args[0])
	if err != nil {
		return nil, err
	}
	dt2, err := toTime(args[1])
	if err != nil {
		return nil, err
	}
	unit := executor.ToString(args[2])
	diff := dt2.Sub(dt1)

	unit = normalizeUnit(unit)
	switch unit {
	case "second":
		return int64(diff.Seconds()), nil
	case "minute":
		return int64(diff.Minutes()), nil
	case "hour":
		return int64(diff.Hours()), nil
	case "day":
		return int64(diff.Hours() / 24), nil
	default:
		return int64(diff.Seconds()), nil
	}
}

func fnDtFormat(args []any) (any, error) {
	dt, err := toTime(args[0])
	if err != nil {
		return nil, err
	}
	format := executor.ToString(args[1])
	// Convert common format tokens to Go layout
	goLayout := convertDateFormat(format)
	return dt.Format(goLayout), nil
}

func fnYear(args []any) (any, error) {
	dt, err := toTime(args[0])
	if err != nil {
		return nil, err
	}
	return int64(dt.Year()), nil
}

func fnMonth(args []any) (any, error) {
	dt, err := toTime(args[0])
	if err != nil {
		return nil, err
	}
	return int64(dt.Month()), nil
}

func fnDay(args []any) (any, error) {
	dt, err := toTime(args[0])
	if err != nil {
		return nil, err
	}
	return int64(dt.Day()), nil
}

func fnHour(args []any) (any, error) {
	dt, err := toTime(args[0])
	if err != nil {
		return nil, err
	}
	return int64(dt.Hour()), nil
}

func fnMinute(args []any) (any, error) {
	dt, err := toTime(args[0])
	if err != nil {
		return nil, err
	}
	return int64(dt.Minute()), nil
}

func fnSecond(args []any) (any, error) {
	dt, err := toTime(args[0])
	if err != nil {
		return nil, err
	}
	return int64(dt.Second()), nil
}

func fnDayOfWeek(args []any) (any, error) {
	dt, err := toTime(args[0])
	if err != nil {
		return nil, err
	}
	return int64(dt.Weekday()), nil // 0=Sunday
}

func fnStartOf(args []any) (any, error) {
	dt, err := toTime(args[0])
	if err != nil {
		return nil, err
	}
	unit := strings.ToLower(executor.ToString(args[1]))
	switch unit {
	case "day":
		return time.Date(dt.Year(), dt.Month(), dt.Day(), 0, 0, 0, 0, dt.Location()), nil
	case "week":
		wd := int(dt.Weekday())
		return time.Date(dt.Year(), dt.Month(), dt.Day()-wd, 0, 0, 0, 0, dt.Location()), nil
	case "month":
		return time.Date(dt.Year(), dt.Month(), 1, 0, 0, 0, 0, dt.Location()), nil
	case "year":
		return time.Date(dt.Year(), 1, 1, 0, 0, 0, 0, dt.Location()), nil
	case "hour":
		return time.Date(dt.Year(), dt.Month(), dt.Day(), dt.Hour(), 0, 0, 0, dt.Location()), nil
	case "minute":
		return time.Date(dt.Year(), dt.Month(), dt.Day(), dt.Hour(), dt.Minute(), 0, 0, dt.Location()), nil
	default:
		return dt, nil
	}
}

func fnEndOf(args []any) (any, error) {
	dt, err := toTime(args[0])
	if err != nil {
		return nil, err
	}
	unit := strings.ToLower(executor.ToString(args[1]))
	switch unit {
	case "day":
		return time.Date(dt.Year(), dt.Month(), dt.Day(), 23, 59, 59, 999999999, dt.Location()), nil
	case "week":
		wd := int(dt.Weekday())
		saturday := 6 - wd
		return time.Date(dt.Year(), dt.Month(), dt.Day()+saturday, 23, 59, 59, 999999999, dt.Location()), nil
	case "month":
		y, m, _ := dt.Date()
		first := time.Date(y, m+1, 1, 0, 0, 0, 0, dt.Location())
		return first.Add(-time.Nanosecond), nil
	case "year":
		first := time.Date(dt.Year()+1, 1, 1, 0, 0, 0, 0, dt.Location())
		return first.Add(-time.Nanosecond), nil
	default:
		return dt, nil
	}
}

func fnBusinessDaysBetween(args []any) (any, error) {
	start, err := toTime(args[0])
	if err != nil {
		return nil, err
	}
	end, err := toTime(args[1])
	if err != nil {
		return nil, err
	}

	count := int64(0)
	current := start
	for current.Before(end) {
		wd := current.Weekday()
		if wd != time.Saturday && wd != time.Sunday {
			count++
		}
		current = current.AddDate(0, 0, 1)
	}
	return count, nil
}

func fnIsBusinessDay(args []any) (any, error) {
	dt, err := toTime(args[0])
	if err != nil {
		return nil, err
	}
	wd := dt.Weekday()
	return wd >= time.Monday && wd <= time.Friday, nil
}

func fnTimeBucket(args []any) (any, error) {
	return fnStartOf(args)
}

// convertDateFormat translates common format tokens to Go reference time layout.
func convertDateFormat(format string) string {
	r := strings.NewReplacer(
		"YYYY", "2006",
		"YY", "06",
		"MM", "01",
		"DD", "02",
		"HH", "15",
		"mm", "04",
		"ss", "05",
	)
	return r.Replace(format)
}

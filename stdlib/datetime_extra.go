package stdlib

import (
	"fmt"
	"strings"
	"time"

	"github.com/xraph/dtl/executor"
)

// Datetime operations completing the surface.
//
// Note what is absent: DTL has no duration value. durationUnit builds a
// time.Duration internally, but dt_add takes an amount and a unit string and
// returns a datetime, so no duration ever reaches the language and type_of has
// no case for one. duration_between therefore returns a number in a unit the
// caller names, rather than implying a type that does not exist.
func registerDatetimeExtra(m map[string]*executor.BuiltinFunc) {
	register(m, "dt_parse", 2, 2, fnDtParse,
		"dt_parse(s, format) -> datetime -- Parses a string using YYYY, MM, DD, HH, mm, ss tokens. Errors when it does not match")
	register(m, "to_unix", 1, 1, fnToUnix,
		"to_unix(dt) -> int -- Seconds since the Unix epoch")
	register(m, "from_unix", 1, 1, fnFromUnix,
		"from_unix(seconds) -> datetime -- Datetime from seconds since the Unix epoch, in UTC")
	register(m, "day_of_year", 1, 1, fnDayOfYear,
		"day_of_year(dt) -> int -- Day of the year, 1-366")
	register(m, "iso_week", 1, 1, fnISOWeek,
		"iso_week(dt) -> int -- ISO 8601 week number, 1-53")
	register(m, "dt_in_zone", 2, 2, fnDtInZone,
		"dt_in_zone(dt, zone) -> datetime -- Same instant expressed in an IANA zone such as 'America/New_York'")
	register(m, "is_before", 2, 2, fnIsBefore,
		"is_before(a, b) -> bool -- Whether a is strictly earlier than b")
	register(m, "is_after", 2, 2, fnIsAfter,
		"is_after(a, b) -> bool -- Whether a is strictly later than b")
	register(m, "is_between", 3, 3, fnIsBetween,
		"is_between(dt, start, end) -> bool -- Whether dt falls within [start, end], inclusive at both ends")
	register(m, "duration_between", 3, 3, fnDurationBetween,
		"duration_between(from, to, unit) -> float -- Elapsed time in the named unit, fractional. Negative when to precedes from")
}

func fnDtParse(args []any) (any, error) {
	s := executor.ToString(args[0])
	layout := convertDateFormat(executor.ToString(args[1]))

	parsed, err := time.Parse(layout, s)
	if err != nil {
		return nil, fmt.Errorf("dt_parse: %q does not match format %q", s, executor.ToString(args[1]))
	}
	return parsed, nil
}

func fnToUnix(args []any) (any, error) {
	dt, err := toTime(args[0])
	if err != nil {
		return nil, fmt.Errorf("to_unix: %w", err)
	}
	return dt.Unix(), nil
}

func fnFromUnix(args []any) (any, error) {
	return time.Unix(executor.ToInt(args[0]), 0).UTC(), nil
}

func fnDayOfYear(args []any) (any, error) {
	dt, err := toTime(args[0])
	if err != nil {
		return nil, fmt.Errorf("day_of_year: %w", err)
	}
	return int64(dt.YearDay()), nil
}

func fnISOWeek(args []any) (any, error) {
	dt, err := toTime(args[0])
	if err != nil {
		return nil, fmt.Errorf("iso_week: %w", err)
	}
	_, week := dt.ISOWeek()
	return int64(week), nil
}

// fnDtInZone reports plainly when zone data is unavailable rather than silently
// leaving the datetime in UTC. Go reads IANA zones from the host, or from an
// embedded copy if the binary imports time/tzdata; a scratch container has
// neither, and a caller converting to a named zone needs to know it did not
// happen rather than discover it in the output.
func fnDtInZone(args []any) (any, error) {
	dt, err := toTime(args[0])
	if err != nil {
		return nil, fmt.Errorf("dt_in_zone: %w", err)
	}
	name := executor.ToString(args[1])

	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("dt_in_zone: cannot load zone %q: %w "+
			"(the host may be missing IANA zone data; import _ \"time/tzdata\" to embed it)", name, err)
	}
	return dt.In(loc), nil
}

func fnIsBefore(args []any) (any, error) {
	a, b, err := twoTimes("is_before", args)
	if err != nil {
		return nil, err
	}
	return a.Before(b), nil
}

func fnIsAfter(args []any) (any, error) {
	a, b, err := twoTimes("is_after", args)
	if err != nil {
		return nil, err
	}
	return a.After(b), nil
}

// fnIsBetween is inclusive at both ends, which is what "between these dates"
// means when a human says it about a reporting period.
func fnIsBetween(args []any) (any, error) {
	dt, err := toTime(args[0])
	if err != nil {
		return nil, fmt.Errorf("is_between: %w", err)
	}
	start, err := toTime(args[1])
	if err != nil {
		return nil, fmt.Errorf("is_between: %w", err)
	}
	end, err := toTime(args[2])
	if err != nil {
		return nil, fmt.Errorf("is_between: %w", err)
	}
	return !dt.Before(start) && !dt.After(end), nil
}

// fnDurationBetween returns a fractional count, unlike diff which truncates to
// whole units. Half a day is 0.5 here and 0 there, and both answers are wanted
// often enough to justify both functions.
func fnDurationBetween(args []any) (any, error) {
	from, to, err := twoTimes("duration_between", args)
	if err != nil {
		return nil, err
	}
	elapsed := to.Sub(from)

	switch normalizeUnit(executor.ToString(args[2])) {
	case "millisecond":
		return float64(elapsed.Nanoseconds()) / 1e6, nil
	case "second":
		return elapsed.Seconds(), nil
	case "minute":
		return elapsed.Minutes(), nil
	case "hour":
		return elapsed.Hours(), nil
	case "day":
		return elapsed.Hours() / 24, nil
	case "week":
		return elapsed.Hours() / (24 * 7), nil
	default:
		return nil, fmt.Errorf("duration_between: unknown unit %q", executor.ToString(args[2]))
	}
}

// normalizeUnit lowercases a unit name and drops a trailing plural, so "Days",
// "days" and "day" all mean the same thing.
//
// The lowercasing has to come first. Trimming an "s" before lowercasing leaves
// "HOURS" ending in a capital S, which no suffix match catches — the bug diff
// has always had, where it lowercases the already-trimmed string.
func normalizeUnit(unit string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(unit)), "s")
}

func twoTimes(fnName string, args []any) (time.Time, time.Time, error) {
	a, err := toTime(args[0])
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("%s: %w", fnName, err)
	}
	b, err := toTime(args[1])
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("%s: %w", fnName, err)
	}
	return a, b, nil
}

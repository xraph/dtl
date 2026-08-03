package stdlib

import (
	"fmt"
	"math"

	"github.com/xraph/dtl/executor"
)

func registerMathExtra(m map[string]*executor.BuiltinFunc) {
	register(m, "exp", 1, 1, fnExp,
		"exp(x) -> float -- e raised to the power of x")
	register(m, "log2", 1, 1, fnLog2,
		"log2(x) -> float -- Base-2 logarithm. Errors on non-positive input")
	register(m, "mod", 2, 2, fnMod,
		"mod(a, b) -> float -- Remainder of a divided by b, taking the sign of a. Errors when b is zero")
	register(m, "trunc", 1, 1, fnTrunc,
		"trunc(x) -> float -- Discards the fractional part, rounding toward zero")
	register(m, "gcd", 2, 2, fnGCD,
		"gcd(a, b) -> int -- Greatest common divisor of two integers")
	register(m, "lcm", 2, 2, fnLCM,
		"lcm(a, b) -> int -- Least common multiple of two integers; 0 when either is 0")
	register(m, "hypot", 2, 2, fnHypot,
		"hypot(a, b) -> float -- Length of the hypotenuse, without intermediate overflow")
	register(m, "sin", 1, 1, fnSin,
		"sin(x) -> float -- Sine of x, in radians")
	register(m, "cos", 1, 1, fnCos,
		"cos(x) -> float -- Cosine of x, in radians")
	register(m, "tan", 1, 1, fnTan,
		"tan(x) -> float -- Tangent of x, in radians")
	register(m, "atan2", 2, 2, fnAtan2,
		"atan2(y, x) -> float -- Angle in radians from the positive x-axis to the point (x, y)")
	register(m, "is_nan", 1, 1, fnIsNaN,
		"is_nan(x) -> bool -- Whether the value is the floating-point not-a-number")
	register(m, "is_finite", 1, 1, fnIsFinite,
		"is_finite(x) -> bool -- Whether the value is neither infinite nor not-a-number")
}

func fnExp(args []any) (any, error) {
	return math.Exp(executor.ToFloat(args[0])), nil
}

// fnLog2 rejects non-positive input, matching log and log10 rather than
// returning the -Inf or NaN that math.Log2 would.
func fnLog2(args []any) (any, error) {
	v := executor.ToFloat(args[0])
	if v <= 0 {
		return nil, fmt.Errorf("log2: non-positive input")
	}
	return math.Log2(v), nil
}

// fnMod errors on a zero divisor rather than returning NaN, matching how the
// executor treats integer division by zero. A NaN would flow silently through
// the rest of a transformation.
func fnMod(args []any) (any, error) {
	b := executor.ToFloat(args[1])
	if b == 0 {
		return nil, fmt.Errorf("mod: division by zero")
	}
	return math.Mod(executor.ToFloat(args[0]), b), nil
}

func fnTrunc(args []any) (any, error) {
	return math.Trunc(executor.ToFloat(args[0])), nil
}

func fnGCD(args []any) (any, error) {
	a, b := absInt(executor.ToInt(args[0])), absInt(executor.ToInt(args[1]))
	for b != 0 {
		a, b = b, a%b
	}
	return a, nil
}

func fnLCM(args []any) (any, error) {
	a, b := absInt(executor.ToInt(args[0])), absInt(executor.ToInt(args[1]))
	if a == 0 || b == 0 {
		return int64(0), nil
	}
	gcd, err := fnGCD(args)
	if err != nil {
		return nil, err
	}
	g, _ := gcd.(int64)
	// Divide before multiplying, so a large pair does not overflow on the way
	// to a representable answer.
	return a / g * b, nil
}

// absInt avoids the float round trip math.Abs would impose, which would lose
// precision for integers beyond 2^53.
func absInt(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func fnHypot(args []any) (any, error) {
	return math.Hypot(executor.ToFloat(args[0]), executor.ToFloat(args[1])), nil
}

func fnSin(args []any) (any, error) {
	return math.Sin(executor.ToFloat(args[0])), nil
}

func fnCos(args []any) (any, error) {
	return math.Cos(executor.ToFloat(args[0])), nil
}

func fnTan(args []any) (any, error) {
	return math.Tan(executor.ToFloat(args[0])), nil
}

func fnAtan2(args []any) (any, error) {
	return math.Atan2(executor.ToFloat(args[0]), executor.ToFloat(args[1])), nil
}

// fnIsNaN and fnIsFinite check the actual stored value rather than coercing,
// since ToFloat would turn a string or null into a perfectly finite 0 and the
// answer would be meaningless.
func fnIsNaN(args []any) (any, error) {
	f, ok := args[0].(float64)
	if !ok {
		return false, nil
	}
	return math.IsNaN(f), nil
}

func fnIsFinite(args []any) (any, error) {
	switch v := args[0].(type) {
	case float64:
		return !math.IsNaN(v) && !math.IsInf(v, 0), nil
	case int64:
		return true, nil
	default:
		return false, nil
	}
}

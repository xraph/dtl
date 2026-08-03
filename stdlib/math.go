package stdlib

import (
	"math/rand/v2"

	"github.com/xraph/dtl/executor"
)

func registerMath(m map[string]*executor.BuiltinFunc) {
	register(m, "clamp", 3, 3, fnClamp,
		"clamp(value, min, max) -> float -- Restricts value to the range [min, max]")
	register(m, "lerp", 3, 3, fnLerp,
		"lerp(a, b, t) -> float -- Linear interpolation between a and b at position t")
	register(m, "normalize", 3, 3, fnNormalize,
		"normalize(value, min, max) -> float -- Scales value into 0..1 across the range; 0 when min equals max")
	register(m, "moving_avg", 2, 2, fnMovingAvg,
		"moving_avg(values, window) -> float -- Mean of the last `window` values; uses every value when the window exceeds the array")
	register(m, "ewma", 1, 2, fnEwma,
		"ewma(values, alpha?) -> float -- Exponentially weighted moving average (default alpha 0.3)")
	register(m, "sign", 1, 1, fnSign,
		"sign(x) -> float -- -1 when negative, 1 when positive, 0 when zero")
	register(m, "random", 0, 0, fnRandom,
		"random() -> float -- Random number in [0, 1). Not suitable for security purposes")
	register(m, "random_int", 2, 2, fnRandomInt,
		"random_int(min, max) -> int -- Random integer in [min, max]. Not suitable for security purposes")

	// Legacy namespace spellings. Registered as aliases so they can never
	// describe or behave differently from the bare names above.
	alias(m, "system::math::clamp", "clamp")
	alias(m, "system::math::lerp", "lerp")
	alias(m, "system::math::normalize", "normalize")
	alias(m, "system::math::moving_avg", "moving_avg")
	alias(m, "system::math::ewma", "ewma")
}

func fnClamp(args []any) (any, error) {
	val := executor.ToFloat(args[0])
	minV := executor.ToFloat(args[1])
	maxV := executor.ToFloat(args[2])
	if val < minV {
		return minV, nil
	}
	if val > maxV {
		return maxV, nil
	}
	return val, nil
}

func fnLerp(args []any) (any, error) {
	a := executor.ToFloat(args[0])
	b := executor.ToFloat(args[1])
	t := executor.ToFloat(args[2])
	return a + (b-a)*t, nil
}

func fnNormalize(args []any) (any, error) {
	val := executor.ToFloat(args[0])
	minV := executor.ToFloat(args[1])
	maxV := executor.ToFloat(args[2])
	if maxV == minV {
		return 0.0, nil
	}
	return (val - minV) / (maxV - minV), nil
}

func fnMovingAvg(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok || len(arr) == 0 {
		return 0.0, nil
	}
	window := int(executor.ToInt(args[1]))
	if window <= 0 || window > len(arr) {
		window = len(arr)
	}

	start := len(arr) - window
	sum := 0.0
	for i := start; i < len(arr); i++ {
		sum += executor.ToFloat(arr[i])
	}
	return sum / float64(window), nil
}

func fnEwma(args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok || len(arr) == 0 {
		return 0.0, nil
	}
	alpha := 0.3
	if len(args) > 1 {
		alpha = executor.ToFloat(args[1])
	}

	result := executor.ToFloat(arr[0])
	for i := 1; i < len(arr); i++ {
		v := executor.ToFloat(arr[i])
		result = alpha*v + (1-alpha)*result
	}
	return result, nil
}

func fnSign(args []any) (any, error) {
	v := executor.ToFloat(args[0])
	if v > 0 {
		return 1.0, nil
	}
	if v < 0 {
		return -1.0, nil
	}
	return 0.0, nil
}

func fnRandom(_ []any) (any, error) {
	return rand.Float64(), nil //#nosec G404 -- expression-language RNG, not used for security/crypto purposes
}

func fnRandomInt(args []any) (any, error) {
	min := executor.ToInt(args[0])
	max := executor.ToInt(args[1])
	if min >= max {
		return min, nil
	}
	return min + rand.Int64N(max-min+1), nil //#nosec G404 -- expression-language RNG, not used for security/crypto purposes
}

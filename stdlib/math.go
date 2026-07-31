package stdlib

import (
	"math/rand/v2"

	"github.com/xraph/dtl/executor"
)

func registerMath(m map[string]*executor.BuiltinFunc) {
	register(m, "clamp", 3, 3, fnClamp)
	register(m, "lerp", 3, 3, fnLerp)
	register(m, "normalize", 3, 3, fnNormalize)
	register(m, "moving_avg", 2, 2, fnMovingAvg)
	register(m, "ewma", 1, 2, fnEwma)
	register(m, "sign", 1, 1, fnSign)
	register(m, "random", 0, 0, fnRandom)
	register(m, "random_int", 2, 2, fnRandomInt)

	// Namespace aliases
	register(m, "system::math::clamp", 3, 3, fnClamp)
	register(m, "system::math::lerp", 3, 3, fnLerp)
	register(m, "system::math::normalize", 3, 3, fnNormalize)
	register(m, "system::math::moving_avg", 2, 2, fnMovingAvg)
	register(m, "system::math::ewma", 1, 2, fnEwma)
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

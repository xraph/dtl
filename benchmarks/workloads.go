// Package benchmarks compares DTL against other embeddable expression and
// scripting languages for Go. See README.md for methodology.
package benchmarks

// Unsupported marks a workload an engine cannot express without changing what
// is being measured. An empty source is reported as "n/a" rather than being
// bent into a near-equivalent, which would make the column meaningless.
const Unsupported = ""

// Workload is one semantic computation, spelled in each engine's own syntax.
// Every spelling must produce the same normalized result — enforced by
// TestParity, which is the guard that keeps these numbers comparable.
type Workload struct {
	Name string
	Desc string

	// Input is the variable environment. Numbers are float64 throughout so no
	// engine gets an unfair integer fast path that another one lacks.
	Input map[string]any

	// Want is the expected result after normalize().
	Want any

	DTL      string
	Expr     string
	CEL      string
	JS       string
	Lua      string
	Starlark string
}

func floats(n int) []any {
	xs := make([]any, n)
	for i := range xs {
		xs[i] = float64(i % 20) // deterministic, ~half over the threshold
	}
	return xs
}

// sumOver computes the expected result of the collection workload directly,
// so Want is derived rather than hand-copied.
func sumOver(xs []any, threshold float64) float64 {
	var t float64
	for _, x := range xs {
		if f := x.(float64); f > threshold {
			t += f
		}
	}
	return t
}

var collInput = floats(100)

// Workloads are ordered cheapest to most expensive. The first two are
// deliberately tiny: at that size per-call overhead dominates, which is
// exactly what an embedding host pays on every rule evaluation.
var Workloads = []*Workload{
	{
		Name:  "arith",
		Desc:  "a + b*2 - 1 — three float ops, the floor of per-call overhead",
		Input: map[string]any{"a": 10.0, "b": 5.0},
		Want:  19.0,

		DTL:      `fn f(a: float, b: float) -> float => a + b * 2 - 1`,
		Expr:     `a + b * 2 - 1`,
		CEL:      `a + b * 2.0 - 1.0`,
		JS:       `a + b * 2 - 1`,
		Lua:      `return a + b * 2 - 1`,
		Starlark: "def f(a, b):\n    return a + b * 2 - 1\n",
	},
	{
		Name:  "cond",
		Desc:  "two-branch numeric classification returning a string",
		Input: map[string]any{"x": 42.0},
		Want:  "mid",

		DTL:      `fn f(x: float) -> string => if x > 100 then "high" else if x > 10 then "mid" else "low"`,
		Expr:     `x > 100 ? "high" : (x > 10 ? "mid" : "low")`,
		CEL:      `x > 100.0 ? "high" : (x > 10.0 ? "mid" : "low")`,
		JS:       `x > 100 ? "high" : (x > 10 ? "mid" : "low")`,
		Lua:      `if x > 100 then return "high" elseif x > 10 then return "mid" else return "low" end`,
		Starlark: "def f(x):\n    return \"high\" if x > 100 else (\"mid\" if x > 10 else \"low\")\n",
	},
	{
		Name:  "string",
		Desc:  "uppercase one input and concatenate with another",
		Input: map[string]any{"a": "ada", "b": "lovelace"},
		Want:  "ADA lovelace",

		DTL:      `fn f(a: string, b: string) -> string => upper(a) + " " + b`,
		Expr:     `upper(a) + " " + b`,
		CEL:      `a.upperAscii() + " " + b`,
		JS:       `a.toUpperCase() + " " + b`,
		Lua:      `return string.upper(a) .. " " .. b`,
		Starlark: "def f(a, b):\n    return a.upper() + \" \" + b\n",
	},
	{
		Name: "field",
		Desc: "three-level nested map traversal",
		Input: map[string]any{"o": map[string]any{
			"user": map[string]any{"address": map[string]any{"city": "London"}},
		}},
		Want: "London",

		DTL:  `fn f(o: object) -> any => o.user.address.city`,
		Expr: `o.user.address.city`,
		CEL:  `o.user.address.city`,
		JS:   `o.user.address.city`,
		Lua:  `return o.user.address.city`,
		// Starlark dicts expose indexing, not attribute access. Indexing is the
		// idiomatic spelling, so this is a like-for-like traversal.
		Starlark: "def f(o):\n    return o[\"user\"][\"address\"][\"city\"]\n",
	},
	{
		Name:  "collection",
		Desc:  "filter a 100-element float list over a threshold, then sum",
		Input: map[string]any{"xs": collInput},
		Want:  sumOver(collInput, 10.0),

		DTL:  `fn f(xs: float[]) -> float => xs | filter((x) => x > 10.0) | sum`,
		Expr: `sum(filter(xs, # > 10.0))`,
		// Base CEL has filter but no fold/sum, and cel-go ships no standard sum
		// extension. Emulating one with a nested comprehension would measure a
		// different program than every other column, so this is left n/a.
		CEL:      Unsupported,
		JS:       `xs.filter(function (v) { return v > 10.0; }).reduce(function (a, b) { return a + b; }, 0)`,
		Lua:      `local t = 0 for i = 1, #xs do if xs[i] > 10.0 then t = t + xs[i] end end return t`,
		Starlark: "def f(xs):\n    t = 0.0\n    for x in xs:\n        if x > 10.0:\n            t += x\n    return t\n",
	},
}

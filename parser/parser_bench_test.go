package parser

import "testing"

func BenchmarkParse_Simple(b *testing.B) {
	src := "fn add(a: int, b: int) -> int => a + b"
	for i := 0; i < b.N; i++ {
		_, errs := Parse(src)
		if len(errs) > 0 {
			b.Fatal(errs)
		}
	}
}

func BenchmarkParse_Complex(b *testing.B) {
	src := `fn analyze(data: any[], threshold: float = 10.0) -> object:
  let filtered = filter(data, (x) => x > threshold)
  let total = sum(filtered)
  let average = avg(filtered)
  let result = if len(filtered) > 0 then {count: len(filtered), sum: total, avg: average} else {count: 0, sum: 0.0, avg: 0.0}
  return result`
	for i := 0; i < b.N; i++ {
		_, errs := Parse(src)
		if len(errs) > 0 {
			b.Fatal(errs)
		}
	}
}

func BenchmarkParseExpression_Arithmetic(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, errs := ParseExpression("(a + b) * c - d / e % f")
		if len(errs) > 0 {
			b.Fatal(errs)
		}
	}
}

func BenchmarkParseExpression_Pipe(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, errs := ParseExpression(`data | filter((x) => x > 0) | map((x) => x * 2) | sum`)
		if len(errs) > 0 {
			b.Fatal(errs)
		}
	}
}

func BenchmarkParseExpression_MatchExpr(b *testing.B) {
	src := `match status: when "active" => 1 when "pending" => 2 when "closed" => 3 when _ => 0`
	for i := 0; i < b.N; i++ {
		_, errs := ParseExpression(src)
		if len(errs) > 0 {
			b.Fatal(errs)
		}
	}
}

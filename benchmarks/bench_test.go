package benchmarks

import (
	"testing"
)

// BenchmarkEval measures steady-state evaluation: the program is compiled once,
// then evaluated b.N times. This is the number that matters for an embedding
// host, which compiles rules at load and evaluates them per request.
func BenchmarkEval(b *testing.B) {
	for _, w := range Workloads {
		b.Run(w.Name, func(b *testing.B) {
			for _, e := range Engines {
				if e.Source(w) == Unsupported {
					continue
				}
				b.Run(e.Name(), func(b *testing.B) {
					prog, err := e.Compile(w)
					if err != nil {
						b.Fatalf("compile: %v", err)
					}
					// Evaluate once outside the timed loop: a failure here is a
					// setup bug, and checking inside the loop would time the check.
					if _, err := prog.Eval(w.Input); err != nil {
						b.Fatalf("eval: %v", err)
					}

					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						if _, err := prog.Eval(w.Input); err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		})
	}
}

// BenchmarkCompile measures cold-start cost: parse, type-check and prepare a
// program for evaluation. This dominates when rules change often or are
// compiled per request rather than cached.
func BenchmarkCompile(b *testing.B) {
	for _, w := range Workloads {
		b.Run(w.Name, func(b *testing.B) {
			for _, e := range Engines {
				if e.Source(w) == Unsupported {
					continue
				}
				b.Run(e.Name(), func(b *testing.B) {
					if _, err := e.Compile(w); err != nil {
						b.Fatalf("compile: %v", err)
					}

					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						if _, err := e.Compile(w); err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		})
	}
}

package benchmarks

import (
	"math"
	"testing"
)

// TestParity is the precondition for every number this package reports.
// A speed comparison between programs that compute different things is
// meaningless, so each engine's spelling of a workload must produce the same
// normalized result before its timings are worth reading.
func TestParity(t *testing.T) {
	for _, w := range Workloads {
		t.Run(w.Name, func(t *testing.T) {
			var ran int
			for _, e := range Engines {
				if e.Source(w) == Unsupported {
					t.Logf("%-11s n/a — not expressible without changing the program", e.Name())
					continue
				}
				prog, err := e.Compile(w)
				if err != nil {
					t.Errorf("%-11s compile: %v", e.Name(), err)
					continue
				}
				got, err := prog.Eval(w.Input)
				if err != nil {
					t.Errorf("%-11s eval: %v", e.Name(), err)
					continue
				}
				ran++
				if n := normalize(got); !equal(n, w.Want) {
					t.Errorf("%-11s = %#v (%T), want %#v", e.Name(), n, n, w.Want)
				} else {
					t.Logf("%-11s = %#v", e.Name(), n)
				}
			}
			if ran < 2 {
				t.Errorf("only %d engine(s) ran — nothing to compare", ran)
			}
		})
	}
}

func equal(got, want any) bool {
	gf, gok := got.(float64)
	wf, wok := want.(float64)
	if gok && wok {
		return math.Abs(gf-wf) < 1e-9
	}
	return got == want
}

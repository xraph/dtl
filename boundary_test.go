// Package dtl_test holds the module's boundary guard.
//
// DTL is meant to be embeddable in any host. A dependency on one particular
// host would silently undo that without necessarily breaking a build — so
// this asserts the absence directly rather than trusting review.
package dtl_test

import (
	"os/exec"
	"strings"
	"testing"
)

// selfPrefix is the only module in its own namespace that DTL may depend on.
const selfPrefix = "github.com/xraph/dtl"

// siblingNamespace is where this project's other, host-side modules live.
// Naming them individually would mean this guard silently stops covering
// whichever one is written next, so the rule is stated the other way round:
// nothing from the namespace except DTL itself.
const siblingNamespace = "github.com/xraph/"

func TestModule_hasNoHostDependencies(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "./...").Output()
	if err != nil {
		t.Fatalf("go list -deps: %v", err)
	}

	for _, dep := range strings.Split(string(out), "\n") {
		dep = strings.TrimSpace(dep)
		if !strings.HasPrefix(dep, siblingNamespace) {
			continue
		}
		if dep == selfPrefix || strings.HasPrefix(dep, selfPrefix+"/") {
			continue
		}
		t.Errorf("dtl must not depend on %s — the language stays host-agnostic", dep)
	}
}

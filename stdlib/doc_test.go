package stdlib

import (
	"strings"
	"testing"

	"github.com/xraph/dtl/executor"
)

// TestEveryBuiltinIsDocumented is the guard that keeps the standard library and
// its editor documentation from drifting apart. Before Doc existed the two
// lived in separate files and diverged badly: the editor table documented
// functions the language did not have and omitted dozens it did. Requiring a
// doc at the registration site makes that particular drift impossible, and this
// test is what enforces the requirement.
func TestEveryBuiltinIsDocumented(t *testing.T) {
	builtins := make(map[string]*executor.BuiltinFunc)
	RegisterAll(builtins)

	if len(builtins) == 0 {
		t.Fatal("RegisterAll registered nothing")
	}

	for name, b := range builtins {
		if strings.TrimSpace(b.Doc) == "" {
			t.Errorf("%s: no Doc. Every registration must document itself", name)
			continue
		}
		// The conventional shape is "name(args) -> type -- description". The
		// separator is what an editor splits on to show signature and prose
		// apart, so a doc without it renders as one undifferentiated run.
		if !strings.Contains(b.Doc, " -- ") {
			t.Errorf("%s: Doc %q lacks the ' -- ' separator between signature and description", name, b.Doc)
		}
		if !strings.HasPrefix(b.Doc, name+"(") {
			t.Errorf("%s: Doc %q should open with the function's own name and its arguments", name, b.Doc)
		}
	}
}

// TestBuiltinNameMatchesKey catches a registration whose Name disagrees with
// the key it was filed under, which would make the executor and the editor
// disagree about what the function is called.
func TestBuiltinNameMatchesKey(t *testing.T) {
	builtins := make(map[string]*executor.BuiltinFunc)
	RegisterAll(builtins)

	for key, b := range builtins {
		if b.Name != key {
			t.Errorf("registered under %q but Name is %q", key, b.Name)
		}
	}
}

// TestAliasesShareTheirTargetsBehaviour pins the property the alias helper
// exists to guarantee: a legacy system::* spelling is the same function as the
// bare name, with the same arity, not a parallel registration that can be
// updated on one side only.
func TestAliasesShareTheirTargetsBehaviour(t *testing.T) {
	builtins := make(map[string]*executor.BuiltinFunc)
	RegisterAll(builtins)

	for name, b := range builtins {
		idx := strings.LastIndex(name, "::")
		if idx < 0 {
			continue
		}
		bare := name[idx+2:]
		target, ok := builtins[bare]
		if !ok {
			// Namespaces with no bare counterpart (time::now, id::uuid, and
			// the new topic namespaces) are not aliases. Nothing to compare.
			continue
		}
		if b.MinArgs != target.MinArgs || b.MaxArgs != target.MaxArgs {
			t.Errorf("%s has arity (%d,%d) but %s has (%d,%d)",
				name, b.MinArgs, b.MaxArgs, bare, target.MinArgs, target.MaxArgs)
		}
	}
}

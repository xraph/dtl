package stdlib

import (
	"reflect"
	"sort"
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
// exists to guarantee: an alternate spelling is the same function, with the
// same arity, not a parallel registration that can be updated on one side only.
//
// Aliases are identified by comparing implementation pointers rather than by
// matching names. Name matching would pair path::flatten with the unrelated
// collections flatten, and assert something neither function promises.
func TestAliasesShareTheirTargetsBehaviour(t *testing.T) {
	builtins := make(map[string]*executor.BuiltinFunc)
	RegisterAll(builtins)

	type arity struct{ min, max int }
	seen := map[uintptr]struct {
		name string
		ar   arity
	}{}

	names := make([]string, 0, len(builtins))
	for name := range builtins {
		names = append(names, name)
	}
	sort.Strings(names) // stable "first" so failures name the same pair each run

	for _, name := range names {
		b := builtins[name]
		if b.Fn == nil {
			continue
		}
		ptr := reflect.ValueOf(b.Fn).Pointer()
		this := arity{b.MinArgs, b.MaxArgs}

		first, ok := seen[ptr]
		if !ok {
			seen[ptr] = struct {
				name string
				ar   arity
			}{name, this}
			continue
		}
		if first.ar != this {
			t.Errorf("%s and %s share an implementation but declare different arities (%d,%d) vs (%d,%d)",
				first.name, name, first.ar.min, first.ar.max, this.min, this.max)
		}
	}
}

package parser

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestPackDTLSources_parseClean compiles every `.dtl` file under
// `packs/<id>/functions/` to lock in that pack-shipped DTL is
// syntactically valid before the install pipeline ever runs against a
// real workspace. Lives in the parser package so it has direct access
// to Parse without going through the public API surface.
//
// Catches typos in builtin names, malformed if/else, missing then,
// arity mismatches in the lexer, etc. Adds zero runtime cost — just
// reads + parses.
func TestPackDTLSources_parseClean(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Skip("runtime.Caller unavailable")
	}
	// Walk up from extensions/function/internal/parser/ to repo root.
	repoRoot := filepath.Join(filepath.Dir(file), "..", "..", "..", "..")
	packsDir := filepath.Join(repoRoot, "packs")

	packs, err := os.ReadDir(packsDir)
	if err != nil {
		t.Skipf("packs dir not readable: %v", err)
	}

	dtlFound := 0
	for _, p := range packs {
		if !p.IsDir() {
			continue
		}
		fnDir := filepath.Join(packsDir, p.Name(), "functions")
		entries, dirErr := os.ReadDir(fnDir)
		if dirErr != nil {
			continue // pack has no functions dir — fine
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".dtl") {
				continue
			}
			dtlFound++

			path := filepath.Join(fnDir, e.Name())
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Errorf("%s/%s: read: %v", p.Name(), e.Name(), readErr)
				continue
			}
			_, errs := Parse(string(data))
			if len(errs) > 0 {
				for _, perr := range errs {
					t.Errorf("%s/%s: parse error: %v", p.Name(), e.Name(), perr)
				}
			}
		}
	}

	if dtlFound == 0 {
		t.Skip("no .dtl files found in packs/")
	}
	t.Logf("parsed %d .dtl source files cleanly", dtlFound)
}

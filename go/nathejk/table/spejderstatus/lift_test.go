package spejderstatus

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// This package is written to shared-go's guidelines so that lifting it to
// shared-go/tables/spejderstatus (roadmap task 083) is a file move rather than a
// rewrite. Exactly one thing decides which of those it will be: whether anything
// here imports nathejk.dk/... — and nothing in the build complains if it does, so
// the discipline rots silently. Hence a test, copied from table/sos/lift_test.go
// (task 054), which exists for the same reason.
//
// The likeliest offender here is named, because it is a real temptation rather
// than a hypothetical: cmd/api/actor.go returns a sos.Actor, and importing
// nathejk.dk/nathejk/table/sos to reuse that one type would be the obvious
// shortcut. That is why this package declares its own Actor (task 070). The second
// likeliest is nathejk.dk/internal/requestctx — table/year/commands.go,
// table/checkgroup/commands.go and table/checkpoint/command.go all reach into it
// for the acting user, which is precisely what makes those packages awkward to
// move. Here the handler resolves the actor and passes it in.
//
// If this test ever fails, the fix is almost never "add the import to an
// allowlist" — it is to take what is needed as an argument or as a port declared
// in this package (see shared-go/tables/interfaces.go for that pattern).
const forbiddenPrefix = "nathejk.dk/"

func TestPackageDoesNotImportApplicationCode(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}

	fset := token.NewFileSet()
	checked := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		// Test files are not lifted, so they may import whatever they need. Only
		// the shipped files have to stay clean.
		if strings.HasSuffix(name, "_test.go") {
			continue
		}

		file, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		checked++

		for _, imp := range file.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if strings.HasPrefix(path, forbiddenPrefix) {
				t.Errorf("%s imports %q: this package must not depend on application code, "+
					"or lifting it to shared-go/tables/spejderstatus stops being a file move. "+
					"Take what you need as an argument or declare a port in this package.",
					name, path)
			}
		}
	}

	// Guard against the guard passing because it found nothing: a rename or a move
	// that left this test looking at an empty directory would otherwise be reported
	// as success.
	if checked == 0 {
		t.Fatal("no non-test .go files found — this check is not looking at the package")
	}
}

// The embedded schema must travel with the package too.
//
// table.sql is loaded with //go:embed, so a lift that moved the .go files and left
// the .sql behind would compile in the new location only if the file came along —
// but a lift that copied it and forgot to delete the original would leave two
// schemas drifting apart. Asserting it is here is cheap; asserting it is *only*
// here is not possible from inside the package, so this is a reminder in test form.
func TestSchemaIsPartOfThePackage(t *testing.T) {
	if _, err := os.Stat("table.sql"); err != nil {
		t.Fatalf("table.sql is not in the package directory: %v", err)
	}
	if tableSchema == "" {
		t.Error("embedded schema is empty; the projection would create no table")
	}
	// The columns PRD 006 added. A lift that picked up a stale copy of table.sql
	// would fail here rather than at 3am with "Unknown column".
	for _, col := range []string{"initialTeamId", "currentTeamId", "status", "updatedAt"} {
		if !strings.Contains(tableSchema, col) {
			t.Errorf("embedded schema is missing the %q column", col)
		}
	}
}

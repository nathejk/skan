package data

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/nathejk/shared-go/types"
)

// KortSheet is a map sheet as a scanner needs it: just enough to pick one from a list.
//
// Deliberately only id and title. A scanner is choosing which sheet they are handing
// over, so the name is all they need to recognise it — not its format, extents, or
// checkpoint list, the last of which is the scouts' information rather than theirs.
type KortSheet struct {
	ID   string
	Name string
}

// KortReader reads the map sheets a patrulje may be handed.
//
// # Why this is not kort.Maps()
//
// `nathejk/table/kort` is hq's package, copied in here, and its `Maps()` read resolves a
// sheet's checkpoint list and handout post against the `checkpoint` and `checkgroup`
// projections — neither of which this service runs. Calling it here fails with
// `Table 'skan.checkpoint' doesn't exist`.
//
// The alternatives were worse. Creating those tables empty would make `Maps()` succeed
// while silently reporting every sheet as having no checkpoints, which is a lie the read
// model would then repeat to anyone who looked. Editing `kort` is out: like `photo`, it
// must stay identical to its origin.
//
// So this asks the narrower question skan actually has — "which sheets may a patrulje be
// handed, and what are they called" — directly against the two tables the `kort`
// projection maintains. Reading another entity's tables has precedent here: `photocover`
// reads `photo` for the same reason, because the question spans both.
type KortReader struct {
	DB *sql.DB
}

// spejderSetFilter selects the sets whose sheets a patrulje may be handed.
//
// The "spejder set" is the set marked for the **patrulje** team type. There is no
// `spejder` team type in shared-go's vocabulary — the word names the scouts, the type
// names the team they form.
//
// An unmarked set is the crew set and is deliberately excluded rather than used as a
// fallback: handing a patrol a crew sheet would show the scouts checkpoints they are not
// meant to see yet. Several sets may carry the same team type, so this is an `IN`, not a
// lookup of "the" set.
const spejderSetFilter = `kortsaetId IN (
		SELECT id FROM kortsaet WHERE year = ? AND teamType = ?
	)`

// SpejderSheets returns the sheets a patrulje may be handed, in handout order.
//
// Empty means the year's patrol maps have not been drawn up yet. That is a setup error
// for the caller to report, not something to paper over: with no sheet there is nothing
// to hand over, and nothing to bind a QR code to.
func (r KortReader) SpejderSheets(ctx context.Context, year string) ([]KortSheet, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// sortOrder is handout order along the route, which is the order a scanner expects to
	// see them in.
	query := `SELECT id, name FROM kort
		WHERE year = ? AND ` + spejderSetFilter + `
		ORDER BY sortOrder ASC, id ASC`

	rows, err := r.DB.QueryContext(ctx, query, year, year, string(types.TeamTypePatrulje))
	if err != nil {
		return nil, fmt.Errorf("reading spejder map sheets: %w", err)
	}
	defer func() { _ = rows.Close() }()

	sheets := []KortSheet{}
	for rows.Next() {
		var s KortSheet
		if err := rows.Scan(&s.ID, &s.Name); err != nil {
			return nil, fmt.Errorf("scanning map sheet: %w", err)
		}
		sheets = append(sheets, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading spejder map sheets: %w", err)
	}
	return sheets, nil
}

// IsSpejderSheet reports whether a sheet is one a patrulje may be handed.
//
// A query rather than a check against a list the caller already has: whether a sheet is
// eligible is a fact about the read model, and the value being checked arrives from a
// form that can be posted directly.
func (r KortReader) IsSpejderSheet(ctx context.Context, year, id string) (bool, error) {
	if id == "" || year == "" {
		return false, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	query := `SELECT COUNT(*) FROM kort
		WHERE id = ? AND year = ? AND ` + spejderSetFilter

	var n int
	err := r.DB.QueryRowContext(ctx, query, id, year, year, string(types.TeamTypePatrulje)).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("checking map sheet %q: %w", id, err)
	}
	return n > 0, nil
}

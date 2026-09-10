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

// qrCodeFilter selects the sheets that carry a QR code, and so can be bound to a patrulje.
//
// A `skitse` is "a hand-drawn slip with no QR code, and usually no extent, whose only trace in
// the system is its checkpoint list" (kort's own table.sql). There is no sticker on it, so it
// can never be the sheet whose code a scanner has just scanned — it has no relevance to this
// UI at all, and listing it only invites a mis-pick.
//
// Deliberately **not** a filter on `handoutCheckgroupId`. An earlier version of this excluded
// sheets handed out at a post, on the reading that only "at the QR scan" sheets are handed
// over here. That confused two different things: where a sheet is given out, and whether it
// carries a code. A sheet handed over at a post still has a sticker, and that sticker still
// has to be bound to the patrulje — which is what a scanner manning that post is doing (task
// 019). Only the absence of a code makes a sheet irrelevant.
//
// `andet` is deliberately kept: nothing says it has no code, and guessing would hide a sheet a
// scanner is holding in their hand.
const qrCodeFilter = `format <> 'skitse'`

// SpejderSheets returns the sheets a patrulje may be handed, in handout order.
//
// Restricted to sheets that carry a QR code — see qrCodeFilter. A sketch has no sticker, so
// there is nothing on it to bind.
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
		WHERE year = ? AND ` + qrCodeFilter + ` AND ` + spejderSetFilter + `
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

// SheetForScanner returns the sheet handed out at the post this scanner is manning.
//
// # Why this is worth doing
//
// A post that hands out sheet 3 hands out sheet 3 all night. Making the crew member pick it
// from a list every time is friction at exactly the wrong moment — in the dark, with scouts
// waiting — and every pick is a chance to choose the wrong one, which binds a patrol's code
// to a map they were not given.
//
// The plan already holds the answer: `kort.handoutCheckgroupId` is "the checkgroup whose post
// gives this sheet to the team", and `checkpersonnel` says which checkpoint a scanner mans.
//
// # One query rather than three
//
// The chain is scanner → checkpoint → checkgroup → sheet, and walking it in Go would mean
// three round trips plus filtering, on a page a scanner is waiting for. The copied packages'
// own queriers cannot answer it anyway: `checkpersonnel.Filter` has no user field, so finding
// a scanner's assignment through them would mean reading every assignment and filtering here.
//
// # found=false is the ordinary case, not a failure
//
// Most scanners are not manning a handout post — bandits never are — and then the picker is
// the right behaviour. So is more than one match: a post configured to hand out several
// sheets has no single answer, and guessing between them would be worse than asking. A post
// that hands out only sketches is another: they carry no QR code, so there is nothing for this
// page to suggest.
func (r KortReader) SheetForScanner(ctx context.Context, year, userID string, at time.Time) (KortSheet, bool, error) {
	if year == "" || userID == "" {
		return KortSheet{}, false, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// startUts/endUts of 0 mean "unbounded", which is how every assignment on the stream
	// currently looks: nothing has ever published a time range, or a removal. So this
	// tolerates open-ended shifts while still honouring one when it is set — without the
	// year filter, a crew member who manned a post last year would match tonight.
	query := `SELECT k.id, k.name
		FROM checkpersonnel cp
		JOIN checkpoint c ON c.id = cp.checkpointId AND c.year = cp.year
		JOIN kort k ON k.handoutCheckgroupId = c.checkgroupId AND k.year = cp.year
		WHERE cp.userId = ? AND cp.year = ?
		  AND (cp.startUts = 0 OR cp.startUts <= ?)
		  AND (cp.endUts = 0 OR cp.endUts >= ?)
		  AND k.` + spejderSetFilter + `
		  AND k.` + qrCodeFilter + `
		ORDER BY k.sortOrder ASC, k.id ASC
		LIMIT 2`

	uts := at.Unix()
	rows, err := r.DB.QueryContext(ctx, query, userID, year, uts, uts, year, string(types.TeamTypePatrulje))
	if err != nil {
		return KortSheet{}, false, fmt.Errorf("reading the sheet for scanner %q: %w", userID, err)
	}
	defer func() { _ = rows.Close() }()

	// LIMIT 2 so "exactly one" can be told from "several" without reading the rest.
	sheets := []KortSheet{}
	for rows.Next() {
		var s KortSheet
		if err := rows.Scan(&s.ID, &s.Name); err != nil {
			return KortSheet{}, false, fmt.Errorf("scanning suggested sheet: %w", err)
		}
		sheets = append(sheets, s)
	}
	if err := rows.Err(); err != nil {
		return KortSheet{}, false, fmt.Errorf("reading the sheet for scanner %q: %w", userID, err)
	}
	if len(sheets) != 1 {
		return KortSheet{}, false, nil
	}
	return sheets[0], true, nil
}

// IsSpejderSheet reports whether a sheet is one a patrulje may be handed.
//
// A query rather than a check against a list the caller already has: whether a sheet is
// eligible is a fact about the read model, and the value being checked arrives from a
// form that can be posted directly.
//
// Deliberately the same two filters as SpejderSheets, so what is validated on submit is
// exactly what was offered. Any sheet excluded from the list must be refused here too,
// or the restriction is decoration — the form can be posted by hand.
func (r KortReader) IsSpejderSheet(ctx context.Context, year, id string) (bool, error) {
	if id == "" || year == "" {
		return false, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	query := `SELECT COUNT(*) FROM kort
		WHERE id = ? AND year = ? AND ` + qrCodeFilter + ` AND ` + spejderSetFilter

	var n int
	err := r.DB.QueryRowContext(ctx, query, id, year, year, string(types.TeamTypePatrulje)).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("checking map sheet %q: %w", id, err)
	}
	return n > 0, nil
}

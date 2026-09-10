package qr

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/nathejk/shared-go/types"
	tables "nathejk.dk/nathejk/table"
)

type querier struct {
	db *sql.DB
}

// GetByID resolves a QR id within one event year.
//
// The year is not optional. Ids restart at 1 each event and the stickers themselves
// may be reused from year to year, so an id alone does not identify a map.
func (q *querier) GetByID(ctx context.Context, yearSlug string, qrID types.QrID) (*QR, error) {
	if yearSlug == "" {
		return nil, tables.ErrRecordNotFound
	}
	query := `SELECT id, teamNumber, mapCreatedBy, mapCreatedAt, mapId
		FROM qr
		WHERE id = ? AND year = ?`
	var r QR
	var id int
	err := q.db.QueryRowContext(ctx, query, qrID, yearSlug).Scan(
		&id,
		&r.TeamNumber,
		&r.MapCreatedBy,
		&r.MapCreatedAt,
		&r.MapID,
	)
	r.ID = types.QrID(fmt.Sprintf("%d", id))
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, tables.ErrRecordNotFound
		default:
			return nil, err
		}
	}
	return &r, nil
}

// MapIDsByTeamNumber is the sheets a patrulje already has a QR code registered for.
//
// Used to hand maps out in order: a patrol may only be given the next sheet they do not
// yet have, so the handler needs to know what they hold.
//
// Keyed on teamNumber because that is what this projection stores — `qr` never learns the
// team's id. Returned as a set, since the order comes from `kort` and two codes for the
// same sheet is not a distinction worth carrying.
//
// Rows with an empty mapId are skipped: codes registered before a sheet was recorded, and
// codes only ever *found*. Neither says anything about which sheet a patrol holds.
func (q *querier) MapIDsByTeamNumber(ctx context.Context, yearSlug string, teamNumber int) (map[string]bool, error) {
	held := map[string]bool{}
	if yearSlug == "" || teamNumber == 0 {
		return held, nil
	}

	query := `SELECT DISTINCT mapId FROM qr
		WHERE year = ? AND teamNumber = ? AND mapId <> ''`

	rows, err := q.db.QueryContext(ctx, query, yearSlug, teamNumber)
	if err != nil {
		return nil, fmt.Errorf("reading registered sheets for team %d: %w", teamNumber, err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning registered sheet: %w", err)
		}
		held[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading registered sheets for team %d: %w", teamNumber, err)
	}
	return held, nil
}

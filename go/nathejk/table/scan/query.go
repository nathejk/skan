package scan

import (
	"context"
	"database/sql"
	"time"

	"github.com/nathejk/shared-go/types"
)

type querier struct {
	db *sql.DB
}

func (q *querier) GetAll(ctx context.Context, filters Filter) ([]*Scan, error) {
	// Create a context with a 3-second timeout.
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	query := `SELECT qrId, teamId, teamNumber, scannerId, scannerPhone, uts, latitude, longitude
		FROM scan
		WHERE (LOWER(year) = LOWER(?) OR ? = '')`
	args := []any{filters.YearSlug, filters.YearSlug}
	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	//totalRecords := 0
	scans := []*Scan{}
	for rows.Next() {
		var r Scan
		if err := rows.Scan(&r.QrID, &r.TeamID, &r.TeamNumber, &r.ScannerID, &r.ScannerPhone, &r.Uts, &r.Latitude, &r.Longitude); err != nil {
			return nil, err
		}
		scans = append(scans, &r)
	}
	// When the rows.Next() loop has finished, call rows.Err() to retrieve any error
	// that was encountered during the iteration.
	if err = rows.Err(); err != nil {
		return nil, err
	}
	//metadata := calculateMetadata(filters.Year, totalRecords, filters.Page, filters.PageSize)

	return scans, nil
}

// CountByTeam is every scan of a patrulje, by anyone.
//
// Crew-only information: it includes checkpoint and guide activity, which tells you
// how far through the race a patrol is. A bandit must never see it.
//
// Keyed on teamId alone, which is already year-unique — a team gets a fresh id each
// event — so no year filter is needed here.
func (q *querier) CountByTeam(ctx context.Context, teamID types.TeamID) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var n int
	err := q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM scan WHERE teamId = ?`, teamID).Scan(&n)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// CountCatchesByTeam is how many times bandits have caught a patrulje.
//
// A catch is a scan whose scanner is a senior — "senior" and "bandit" are the same
// people. This is the one count a bandit is allowed to know.
//
// A plain count of scans, not of distinct scanners: bandits legitimately catch the
// same patrol more than once, and a rescan counts. Guarding against *accidental*
// double scans is a separate concern, handled before the scan is recorded.
//
// EXISTS rather than a JOIN on purpose: the senior projection is keyed
// (year, memberId), so the same memberId can appear in several years and a join would
// multiply a single scan into several catches.
func (q *querier) CountCatchesByTeam(ctx context.Context, teamID types.TeamID) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	const query = `SELECT COUNT(*) FROM scan s
		WHERE s.teamId = ?
		  AND EXISTS (SELECT 1 FROM senior sr WHERE sr.memberId = s.scannerId)`

	var n int
	if err := q.db.QueryRowContext(ctx, query, teamID).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

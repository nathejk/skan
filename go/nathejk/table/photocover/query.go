package photocover

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// queryTimeout bounds a read. Reads happen on the synchronous request path.
const queryTimeout = 3 * time.Second

// Queries is what a handler needs from this entity. Declared here so callers
// depend on the reads rather than on the whole table.
type Queries interface {
	Ref(year, teamID string) (string, error)
	Covers(ctx context.Context, year string) ([]Cover, error)
}

// Ref returns the team's chosen cover ref, or "" when no choice has been made.
//
// "" for "none" rather than an error: no choice is the normal state — most teams
// never have one — and the caller's behaviour is the same either way, namely fall
// back to the newest photograph.
func (t *Table) Ref(year, teamID string) (string, error) {
	if t == nil || t.r == nil {
		return "", errors.New("photocover: no reader configured")
	}
	if year == "" || teamID == "" {
		return "", nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	var ref string
	err := t.r.QueryRowContext(ctx,
		`SELECT ref FROM photocover WHERE year = ? AND teamId = ?`, year, teamID,
	).Scan(&ref)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("photocover: ref: %w", err)
	}
	return ref, nil
}

// Cover is the one photograph that stands for a patrulje in a list.
type Cover struct {
	TeamID string `json:"teamId"`

	// Ref is the display image; ThumbRef the smallest rendition, empty when the
	// photograph has none (it predates renditions), in which case a client shows the
	// display image instead.
	Ref      string `json:"ref"`
	ThumbRef string `json:"thumbRef,omitempty"`

	Width  int `json:"width"`
	Height int `json:"height"`

	// Count is how many photographs the team has, so a list can hint that clicking
	// opens more than one without asking per row.
	Count int `json:"count"`

	// Chosen distinguishes "an organizer picked this" from "this is simply the
	// newest". The picker uses it to show which one is current.
	Chosen bool `json:"chosen"`
}

// Covers returns one photograph per team for a whole year: the chosen cover where
// there is one, otherwise the newest.
//
// One query for every team rather than a request per row. A patrol list is ~200
// rows, and a component that fetched its own photograph would turn one page into
// 200 requests.
//
// It reads the `photo` table, which another entity owns. Deliberate: this entity's
// question is "which photograph represents this team", and answering it needs the
// photographs. Copying rows here to avoid the join would create a second, staler
// record of the same facts.
func (t *Table) Covers(ctx context.Context, year string) ([]Cover, error) {
	if t == nil || t.r == nil {
		return nil, errors.New("photocover: no reader configured")
	}
	if year == "" {
		return []Cover{}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	// Window functions (MariaDB 10.2+), so the "one row per team" choice is made once
	// in the database rather than by reading every photograph into Go. The ordering is
	// the priority: the chosen ref first, then newest, then ref as a tiebreak so the
	// result is stable for two photographs sharing a timestamp.
	const query = `
		SELECT teamId, ref, thumbRef, width, height, cnt, chosen FROM (
			SELECT p.teamId, p.ref, p.thumbRef, p.width, p.height,
			       COUNT(*) OVER (PARTITION BY p.teamId) AS cnt,
			       (c.ref IS NOT NULL AND c.ref <> '' AND c.ref = p.ref) AS chosen,
			       ROW_NUMBER() OVER (
			           PARTITION BY p.teamId
			           ORDER BY (c.ref IS NOT NULL AND c.ref <> '' AND c.ref = p.ref) DESC,
			                    p.capturedAt IS NULL, p.capturedAt DESC, p.ref
			       ) AS rn
			FROM photo p
			LEFT JOIN photocover c ON c.year = p.year AND c.teamId = p.teamId
			WHERE p.year = ?
		) ranked
		WHERE rn = 1`

	rows, err := t.r.QueryContext(ctx, query, year)
	if err != nil {
		return nil, fmt.Errorf("photocover: covers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	covers := []Cover{}
	for rows.Next() {
		var c Cover
		if err := rows.Scan(&c.TeamID, &c.Ref, &c.ThumbRef, &c.Width, &c.Height, &c.Count, &c.Chosen); err != nil {
			return nil, fmt.Errorf("photocover: scan: %w", err)
		}
		covers = append(covers, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("photocover: rows: %w", err)
	}
	return covers, nil
}

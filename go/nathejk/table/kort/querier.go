package kort

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/nathejk/shared-go/types"
)

// ErrRecordNotFound is returned when a kort id names nothing.
var ErrRecordNotFound = errors.New("record not found")

type Queries interface {
	// Maps lists a year's sheets in handout order.
	Maps(context.Context, types.YearSlug) ([]Kort, error)
	// GetByID reads one sheet, which the commands need in order to dirty-check (task 124).
	GetByID(context.Context, types.YearSlug, KortID) (*Kort, error)
	// Sets lists a year's sets in their own order, without their sheets.
	Sets(context.Context, types.YearSlug) ([]Kortsaet, error)
	// GetSetByID reads one set, for the set commands' dirty-check.
	GetSetByID(context.Context, types.YearSlug, KortsaetID) (*Kortsaet, error)
	// CountMapsInSet reports how many sheets a set holds, which is what makes deleting a
	// non-empty set refusable.
	CountMapsInSet(context.Context, types.YearSlug, KortsaetID) (int, error)
}

type querier struct {
	db *sql.DB
	r  *goqu.Database
}

const selectKort = `SELECT id, year, version, kortsaetId, name, format, note, sortOrder,
	checkpointIds, extents, handoutCheckgroupId
	FROM kort`

// Maps lists a year's sheets, grouped by set and in handout order within it.
//
// One query for the whole year with no pagination and no filter beyond the year, because there
// are on the order of fifteen rows: `GET /api/kort` returns all of them, and the map view and the
// settings dialog share that single response (PRD 010 §8). A per-set query would be a second code
// path serving no caller.
//
// This endpoint serves HQ's own SPA only. Other services read the kort events off the stream and
// keep their own read model — see "How the maps leave HQ" in PRD 010.
//
// `id` is the tiebreak after sortOrder so that two sheets sharing a sort order — easy to produce
// mid-reorder — come back in a stable order rather than whatever the storage engine feels like.
// Two consecutive loads disagreeing about the order of the maps would look like a bug in the
// drag-and-drop.
//
// # Unknown checkpoint ids are filtered out here
//
// A second small query reads the year's checkpoint ids, and any id in a sheet's array that no
// longer resolves is dropped from the result. That is not belt-and-braces: it is where deleting a
// *checkgroup* is actually handled, because that event names only the group and its members cannot
// be cascaded out of the JSON array safely (see consumer.pruneCheckpoint for the two ways that
// fail, one of them a MariaDB bug).
//
// Note this fix does **not** travel over the stream: a consumer building its own projection from
// the kort events has to resolve `checkpointIds` against its own checkpoint projection the same
// way. Documented for them in roadmap/api/kort-events.md §4.
//
// Filtering on read rather than on write also self-heals every other cause of a stale id — a
// checkpoint deleted while the API was down, a half-finished replay — and it does so without
// depending on the order two independent projections happen to run in. The cost is one indexed
// query over a table with tens of rows.
//
// # A deleted handout checkgroup is resolved the same way
//
// `handoutCheckgroupId` naming a checkgroup that no longer exists comes back as "", which means the
// QR rule. That is the safe direction: the alternative is a sheet whose reveal is keyed to a post
// that will never be reached, so its checkpoints would never appear at all.
func (q *querier) Maps(ctx context.Context, year types.YearSlug) ([]Kort, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	known, err := q.knownCheckpointIDs(ctx, year)
	if err != nil {
		return nil, err
	}
	knownGroups, err := q.knownCheckgroupIDs(ctx, year)
	if err != nil {
		return nil, err
	}

	query := selectKort + `
		WHERE (year = ? OR ? = '')
		ORDER BY kortsaetId ASC, sortOrder ASC, id ASC`

	rows, err := q.db.QueryContext(ctx, query, string(year), string(year))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	maps := []Kort{}
	for rows.Next() {
		k, err := scanKort(rows)
		if err != nil {
			return nil, err
		}
		k.CheckpointIDs = filterKnown(k.CheckpointIDs, known)
		if k.HandoutCheckgroupID != "" && !knownGroups[k.HandoutCheckgroupID] {
			k.HandoutCheckgroupID = ""
		}
		maps = append(maps, *k)
	}
	return maps, rows.Err()
}

// knownCheckgroupIDs reads the year's checkgroup ids, for the same referential-integrity reason as
// knownCheckpointIDs — and with the same justification for reading another projection's table.
func (q *querier) knownCheckgroupIDs(ctx context.Context, year types.YearSlug) (map[types.CheckgroupID]bool, error) {
	rows, err := q.db.QueryContext(ctx,
		`SELECT id FROM checkgroup WHERE (year = ? OR ? = '')`,
		string(year), string(year))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	known := map[types.CheckgroupID]bool{}
	for rows.Next() {
		var id types.CheckgroupID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		known[id] = true
	}
	return known, rows.Err()
}

// knownCheckpointIDs reads the year's checkpoint ids.
//
// Reading another projection's table, which this package otherwise does not do. Justified because
// the question is about referential integrity between the two, and the alternative — teaching the
// checkpoint projection to publish per-checkpoint deletes so this one could cascade — would change
// an event contract that other consumers already read, to fix a problem only this table has.
func (q *querier) knownCheckpointIDs(ctx context.Context, year types.YearSlug) (map[types.CheckpointID]bool, error) {
	rows, err := q.db.QueryContext(ctx,
		`SELECT id FROM checkpoint WHERE (year = ? OR ? = '')`,
		string(year), string(year))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	known := map[types.CheckpointID]bool{}
	for rows.Next() {
		var id types.CheckpointID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		known[id] = true
	}
	return known, rows.Err()
}

// filterKnown drops ids that no longer resolve, preserving order.
//
// Returns a non-nil slice even when everything was dropped, so the JSON encoder still emits `[]`.
func filterKnown(ids []types.CheckpointID, known map[types.CheckpointID]bool) []types.CheckpointID {
	kept := make([]types.CheckpointID, 0, len(ids))
	for _, id := range ids {
		if known[id] {
			kept = append(kept, id)
		}
	}
	return kept
}

// GetByID reads one sheet.
//
// Year-scoped as well as keyed by id: ids are unique, but a request carrying the wrong year
// should not silently reach another year's map.
func (q *querier) GetByID(ctx context.Context, year types.YearSlug, id KortID) (*Kort, error) {
	if id == "" {
		return nil, ErrRecordNotFound
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	row := q.db.QueryRowContext(ctx,
		selectKort+` WHERE id = ? AND (year = ? OR ? = '')`,
		string(id), string(year), string(year))

	k, err := scanKort(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRecordNotFound
	}
	return k, err
}

// Nest groups sheets under their sets, and reports the ones whose set is unknown.
//
// The nesting is what makes `GET /api/kort` one round trip for a consumer that needs the sets'
// `teamType` markings *and* the sheets — which is every consumer, since a sheet means nothing
// without knowing who it is printed for.
//
// Orphans are returned rather than dropped. A sheet whose `kortsaetId` names no set is possible
// during replay (events arrive in stream order, so a sheet may precede its set) and after a bad
// edit, and silently omitting it would make a map invisible in the one screen that exists to find
// such mistakes. Normally empty.
func Nest(sets []Kortsaet, maps []Kort) (nested []Kortsaet, orphans []Kort) {
	bySet := map[KortsaetID][]Kort{}
	for _, m := range maps {
		bySet[m.KortsaetID] = append(bySet[m.KortsaetID], m)
	}

	nested = make([]Kortsaet, 0, len(sets))
	known := map[KortsaetID]bool{}
	for _, s := range sets {
		known[s.KortsaetID] = true
		s.Maps = bySet[s.KortsaetID]
		if s.Maps == nil {
			// `[]` rather than null: a set with no sheets yet is the ordinary state of a set an
			// operator has just created.
			s.Maps = []Kort{}
		}
		nested = append(nested, s)
	}

	orphans = []Kort{}
	for _, m := range maps {
		if !known[m.KortsaetID] {
			orphans = append(orphans, m)
		}
	}
	return nested, orphans
}

type scanner interface {
	Scan(dest ...any) error
}

const selectKortsaet = `SELECT id, year, version, name, teamType, sortOrder
	FROM kortsaet`

// Sets lists a year's sets in their own order.
//
// Without their sheets: nesting is done by the read path that serves them together (task 125),
// from one query of each table rather than a query per set.
func (q *querier) Sets(ctx context.Context, year types.YearSlug) ([]Kortsaet, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	rows, err := q.db.QueryContext(ctx,
		selectKortsaet+`
		WHERE (year = ? OR ? = '')
		ORDER BY sortOrder ASC, id ASC`,
		string(year), string(year))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sets := []Kortsaet{}
	for rows.Next() {
		s, err := scanKortsaet(rows)
		if err != nil {
			return nil, err
		}
		sets = append(sets, *s)
	}
	return sets, rows.Err()
}

// GetSetByID reads one set.
func (q *querier) GetSetByID(ctx context.Context, year types.YearSlug, id KortsaetID) (*Kortsaet, error) {
	if id == "" {
		return nil, ErrRecordNotFound
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	row := q.db.QueryRowContext(ctx,
		selectKortsaet+` WHERE id = ? AND (year = ? OR ? = '')`,
		string(id), string(year), string(year))

	s, err := scanKortsaet(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRecordNotFound
	}
	return s, err
}

// CountMapsInSet reports how many sheets a set holds.
//
// A count rather than a list, because the only caller asks a yes/no question: may this set be
// deleted? Reading the sheets to answer it would pull every checkpoint list off disk to look at
// len().
func (q *querier) CountMapsInSet(ctx context.Context, year types.YearSlug, id KortsaetID) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var count int
	err := q.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM kort WHERE kortsaetId = ? AND (year = ? OR ? = '')`,
		string(id), string(year), string(year)).Scan(&count)
	return count, err
}

// scanKortsaet reads one set row.
//
// teamType is read through sql.NullString and left nil when absent, never coerced to "": the
// difference between "this set is for patruljer" and "this set is the general one" is the whole
// point of the column.
func scanKortsaet(row scanner) (*Kortsaet, error) {
	var s Kortsaet
	var teamType sql.NullString
	if err := row.Scan(&s.KortsaetID, &s.YearSlug, &s.Version, &s.Name, &teamType, &s.SortOrder); err != nil {
		return nil, err
	}
	if teamType.Valid && teamType.String != "" {
		t := types.TeamType(teamType.String)
		s.TeamType = &t
	}
	return &s, nil
}

// scanKort reads one row, decoding the two JSON columns.
//
// A malformed JSON column yields an empty list rather than an error, and that is deliberate: a
// map whose extents cannot be parsed is still a map with a name and a checkpoint list, and
// failing the whole read would take the entire settings screen down over one bad row. The
// alternative — refusing to load — would also make the row impossible to fix from the UI.
func scanKort(row scanner) (*Kort, error) {
	var k Kort
	var checkpointIDs, extents sql.NullString
	err := row.Scan(
		&k.KortID,
		&k.YearSlug,
		&k.Version,
		&k.KortsaetID,
		&k.Name,
		&k.Format,
		&k.Note,
		&k.SortOrder,
		&checkpointIDs,
		&extents,
		&k.HandoutCheckgroupID,
	)
	if err != nil {
		return nil, err
	}

	k.CheckpointIDs = []types.CheckpointID{}
	if checkpointIDs.Valid && checkpointIDs.String != "" {
		_ = json.Unmarshal([]byte(checkpointIDs.String), &k.CheckpointIDs)
		if k.CheckpointIDs == nil {
			k.CheckpointIDs = []types.CheckpointID{}
		}
	}

	k.Extents = []Extent{}
	if extents.Valid && extents.String != "" {
		_ = json.Unmarshal([]byte(extents.String), &k.Extents)
		if k.Extents == nil {
			k.Extents = []Extent{}
		}
	}

	return &k, nil
}

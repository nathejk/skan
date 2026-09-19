package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// GeoScan is one positioned scan with everything the map export needs, already resolved.
//
// Scanner, Role and Lok are the *outcome* of the identity lookup rather than raw columns,
// because who a scanner is depends on which projection knows them — see resolveScanner.
type GeoScan struct {
	QrID             string
	TeamNumber       int
	TeamName         string
	Uts              int64
	Latitude         string
	Longitude        string
	LocationSource   string
	LocationAccuracy string

	Scanner string
	Role    string
	Lok     string
}

// GeoReader builds the whole /geo export in a single query.
//
// # Why this exists
//
// The handler used to walk the scans and ask four more questions about each one (patrulje,
// senior, that senior's klan, personnel). At 3.4k positioned scans that is well over ten
// thousand round trips for one request, each with its own context timeout, on an endpoint
// that exists to dump the entire race for a GIS tool. One join answers the same thing once.
//
// # Why it is a skan-side reader
//
// The question spans five projections, so it belongs to none of their queriers — the same
// reason KortReader and CheckpointReader exist. Nothing here writes, and the projections
// stay untouched and liftable to shared-go.
type GeoReader struct {
	DB *sql.DB
}

// geoQuery joins a positioned scan to everything that names it.
//
// Every join is LEFT: a scan is a fact on its own, and a missing patrulje or an unknown
// scanner must not drop it from the map. (That also removes a latent nil dereference — the
// old code read patrulje.Name without checking the lookup succeeded.)
//
// # The senior join is deliberately not matched on year
//
// `senior` is keyed (year, memberId), so the obvious `sr.memberId = s.scannerId AND
// sr.year = s.year` looks safer — and is wrong. A scanner's memberId does not track the
// scan's year: on live data Eskil's only senior row is 2026 while his 41 scans are 2025, and
// year-matching silently dropped the scanner's name from 107 rows. A person's name is not a
// per-year fact, so the lookup must not be either.
//
// But joining on memberId alone would risk the fan-out CountCatchesByTeam avoids with
// EXISTS: one row per year that member has existed. So `latest` reduces senior to exactly
// one row per memberId first — the most recent registration, coherent because every field
// comes from that one real row, and unique because (year, memberId) is the primary key.
// (Today no memberId appears in more than one year, so this matches the old per-scan lookup
// row for row; it is the future duplicate that it guards against.)
//
// The other three joins key on a year-unique teamId/userId and cannot fan out.
//
// klan hangs off the senior, not the scan: a lok is the bandit's camp, which is only
// meaningful once the scanner is known to be a senior.
const geoQuery = `SELECT
		s.qrId, s.teamNumber, s.uts, s.latitude, s.longitude, s.locationSource, s.locationAccuracy,
		p.name, sr.name, k.lok, pe.name, pe.additionals
	FROM scan s
	LEFT JOIN patrulje p ON p.teamId = s.teamId
	LEFT JOIN (
		SELECT one.memberId, one.teamId, one.name
		FROM senior one
		JOIN (
			SELECT memberId, MAX(year) AS year FROM senior GROUP BY memberId
		) latest ON latest.memberId = one.memberId AND latest.year = one.year
	) sr ON sr.memberId = s.scannerId
	LEFT JOIN klan k ON k.teamId = sr.teamId
	LEFT JOIN personnel pe ON pe.userId = s.scannerId
	WHERE s.latitude <> '' AND s.longitude <> ''
	ORDER BY s.uts, s.qrId`

// Scans returns every scan that has a position, across all years.
//
// Deliberately not year-scoped, which is what the endpoint did before: /geo is a full export
// for map and GIS tooling, and narrowing it here would quietly change what a consumer gets.
func (r GeoReader) Scans(ctx context.Context) ([]GeoScan, error) {
	// Generous next to the 3s used elsewhere: this is a bulk export of the whole race, not
	// a page a scanner is waiting for.
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	rows, err := r.DB.QueryContext(ctx, geoQuery)
	if err != nil {
		return nil, fmt.Errorf("reading the geo export: %w", err)
	}
	defer func() { _ = rows.Close() }()

	scans := []GeoScan{}
	for rows.Next() {
		var g GeoScan
		// NullString rather than COALESCE: "no row" and "a row with an empty value" are
		// different answers here, and the precedence rules below turn on which it was.
		var teamName, seniorName, klanLok, personName, personAdditionals sql.NullString
		if err := rows.Scan(
			&g.QrID, &g.TeamNumber, &g.Uts, &g.Latitude, &g.Longitude,
			&g.LocationSource, &g.LocationAccuracy,
			&teamName, &seniorName, &klanLok, &personName, &personAdditionals,
		); err != nil {
			return nil, fmt.Errorf("scanning a geo row: %w", err)
		}
		g.TeamName = teamName.String
		g.Scanner, g.Role, g.Lok = resolveScanner(seniorName, klanLok, personName, personAdditionals)
		scans = append(scans, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading the geo export: %w", err)
	}
	return scans, nil
}

// resolveScanner names the person behind a scan, and what they were doing.
//
// The precedence is the pre-existing behaviour, kept deliberately: a senior sets the name,
// the role "Bandit" and the lok, and a personnel row then **overrides** the name (and the
// role, when it states a department). Someone in both projections is a data error, but the
// export has always preferred the personnel name and this is not the place to change that.
//
// Presence, not emptiness, is what each step tests — a senior row with a blank name still
// makes the scanner a bandit, and a klan row with a blank lok still yields "LOK ", exactly
// as before.
func resolveScanner(seniorName, klanLok, personName, personAdditionals sql.NullString) (scanner, role, lok string) {
	if seniorName.Valid {
		scanner = seniorName.String
		// A senior signed up to a klan *is* a bandit; there is no per-person flag.
		role = "Bandit"
		if klanLok.Valid {
			lok = fmt.Sprintf("LOK %s", klanLok.String)
		}
	}
	if personName.Valid {
		scanner = personName.String
		if department, ok := departmentOf(personAdditionals.String); ok {
			role = department
		}
	}
	return scanner, role, lok
}

// departmentOf pulls the crew member's department out of personnel.additionals.
//
// The column holds whatever the signup form collected, so unparseable or absent is ordinary
// and answers false rather than erroring: a missing department is not a reason to drop a
// scan from the map.
func departmentOf(additionals string) (string, bool) {
	if additionals == "" {
		return "", false
	}
	var fields map[string]any
	if err := json.Unmarshal([]byte(additionals), &fields); err != nil {
		return "", false
	}
	department, ok := fields["department"].(string)
	return department, ok
}

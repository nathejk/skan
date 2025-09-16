package senior

import (
	"context"
	"database/sql"
	"errors"

	"github.com/nathejk/shared-go/types"
	tables "nathejk.dk/nathejk/table"
)

type querier struct {
	db *sql.DB
}

/*
	func (q *querier) query(filters Filter, query string, args []any) ([]types.TeamID, Metadata, error) {
		// Create a context with a 3-second timeout.
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		rows, err := q.db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, Metadata{}, err
		}
		defer rows.Close()

		totalRecords := 0
		teamIDs := []types.TeamID{}
		for rows.Next() {
			var teamID types.TeamID
			if err := rows.Scan(&teamID); err != nil {
				return nil, Metadata{}, err
			}
			teamIDs = append(teamIDs, teamID)
		}
		// When the rows.Next() loop has finished, call rows.Err() to retrieve any error
		// that was encountered during the iteration.
		if err = rows.Err(); err != nil {
			return nil, Metadata{}, err
		}
		metadata := calculateMetadata(filters.YearSlug, totalRecords, filters.Page, filters.PageSize)

		return teamIDs, metadata, nil
	}
*/
func (q *querier) GetAll(ctx context.Context, filter Filter) ([]Klan, error) {
	query := `SELECT t.teamId, t.name, t.groupName, t.korps, t.memberCount, t.signupStatus
		FROM klan t
		JOIN patruljestatus ts ON t.teamId = ts.teamId AND t.signupStatus != ''
		` //WHERE (LOWER(p.year) = LOWER(?) OR ? = '')`
	args := []any{} //filter.YearSlug, filter.YearSlug}
	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	//totalRecords := 0
	klans := []Klan{}
	for rows.Next() {
		var k Klan
		if err := rows.Scan(&k.ID, &k.Name, &k.Group, &k.Korps, &k.MemberCount, &k.Status); err != nil {
			//if err := rows.Scan(&klan.TeamID); err != nil {
			return nil, err
		}
		klans = append(klans, k)
	}
	// When the rows.Next() loop has finished, call rows.Err() to retrieve any error
	// that was encountered during the iteration.
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return klans, nil
}

func (q *querier) GetByID(ctx context.Context, teamID types.TeamID) (*Klan, error) {
	if len(teamID) == 0 {
		return nil, tables.ErrRecordNotFound
	}

	query := `SELECT t.teamId, t.name, t.groupName, t.korps, t.memberCount, t.signupStatus
		FROM klan t
		JOIN patruljestatus ts ON t.teamId = ts.teamID
		WHERE t.teamId = ?`
	var t Klan
	err := q.db.QueryRow(query, teamID).Scan(
		&t.ID,
		&t.Name,
		&t.Group,
		&t.Korps,
		&t.MemberCount,
		&t.Status,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, tables.ErrRecordNotFound
		default:
			return nil, err
		}
	}
	return &t, nil
	return nil, nil
}

/*
	func (m TeamModel) GetStartedTeamIDs(filters Filters) ([]types.TeamID, Metadata, error) {
		sql := `SELECT teamId FROM patruljestatus WHERE startedUts > 0 AND (LOWER(year) = LOWER(?) OR ? = '')`
		args := []any{filters.Year, filters.Year}
		return m.query(filters, sql, args)
	}

	func (m TeamModel) GetDiscontinuedTeamIDs(filters Filters) ([]types.TeamID, Metadata, error) {
		//sql := "SELECT teamId FROM patruljestatus WHERE startedUts > 0 AND (LOWER(year) = LOWER($1) OR $1 = '')"
		sql := `SELECT DISTINCT m.teamId FROM patruljemerged m JOIN patruljestatus s ON m.teamId = s.teamId WHERE s.startedUts > 0 AND (LOWER(year) = LOWER(?) OR ? = '')`
		args := []any{filters.Year, filters.Year}
		return m.query(filters, sql, args)
	}

	func (m TeamModel) RequestedSeniorCount() int {
		query := `SELECT COUNT(memberId) FROM senior WHERE year=%d`
		var count int
		_ = m.DB.QueryRow(query, 2024).Scan(&count)
		return count
	}

	func (m TeamModel) GetPatruljer(filters Filters) ([]*Patrulje, Metadata, error) {
		// Create a context with a 3-second timeout.
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		query := `SELECT p.teamId, p.teamNumber, p.name, p.groupName, p.korps, p.liga, p.memberCount, IF(pm.parentTeamId IS NOT NULL, "JOIN", IF(startedUts > 0, "STARTED",  signupStatus))
			FROM patrulje p
			JOIN patruljestatus ps ON p.teamId = ps.teamID AND (LOWER(p.year) = LOWER(?) OR ? = '')`
		args := []any{filters.Year, filters.Year}
		rows, err := m.DB.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, Metadata{}, err
		}
		defer rows.Close()

		totalRecords := 0
		patruljer := []*Patrulje{}
		for rows.Next() {
			var p Patrulje
			if err := rows.Scan(&p.ID, &p.Number, &p.Name, &p.Group, &p.Korps, &p.Liga, &p.MemberCount, &p.Status); err != nil {
				return nil, Metadata{}, err
			}
			patruljer = append(patruljer, &p)
		}
		// When the rows.Next() loop has finished, call rows.Err() to retrieve any error
		// that was encountered during the iteration.
		if err = rows.Err(); err != nil {
			return nil, Metadata{}, err
		}
		metadata := calculateMetadata(filters.Year, totalRecords, filters.Page, filters.PageSize)

		return patruljer, metadata, nil
	}
func (m TeamModel) GetPatrulje(teamID types.TeamID) (*Patrulje, error) {
	if len(teamID) == 0 {
		return nil, ErrRecordNotFound
	}

	query := `SELECT p.teamId, p.teamNumber, p.name, p.groupName, p.korps, p.liga, p.memberCount
		FROM patrulje p
		JOIN patruljestatus ps ON p.teamId = ps.teamID
		WHERE p.teamId = ?`
	var p Patrulje
	err := m.DB.QueryRow(query, teamID).Scan(
		&p.ID,
		&p.Number,
		&p.Name,
		&p.Group,
		&p.Korps,
		&p.Liga,
		&p.MemberCount,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}
	return &p, nil
}

func (m TeamModel) GetKlan(teamID types.TeamID) (*Klan, error) {
	if len(teamID) == 0 {
		return nil, ErrRecordNotFound
	}

	query := `SELECT t.teamId, t.name, t.groupName, t.korps, t.memberCount, t.signupStatus
		FROM klan t
		JOIN patruljestatus ts ON t.teamId = ts.teamID
		WHERE t.teamId = ?`
	var t Klan
	err := m.DB.QueryRow(query, teamID).Scan(
		&t.ID,
		&t.Name,
		&t.Group,
		&t.Korps,
		&t.MemberCount,
		&t.Status,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}
	return &t, nil
}

func (m TeamModel) GetContact(teamID types.TeamID) (*Contact, error) {
	if len(teamID) == 0 {
		return nil, ErrRecordNotFound
	}

	query := `SELECT p.contactName, p.contactPhone, p.contactEmail, p.contactRole
		FROM patrulje p
		JOIN patruljestatus ps ON p.teamId = ps.teamID
		WHERE p.teamId = ?`
	c := Contact{TeamID: teamID}
	err := m.DB.QueryRow(query, teamID).Scan(
		&c.Name,
		&c.Phone,
		&c.Email,
		&c.Role,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}
	return &c, nil
}
*/

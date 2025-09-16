package data

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/nathejk/shared-go/types"
	"nathejk.dk/internal/validator"
)

type Team struct {
}

func (p *Team) Validate(v validator.Validator) {
	//v.Check(p.Timestamp.IsZero(), "timestamp", "must be provided")
}

type TeamModel struct {
	DB *sql.DB
}

func (m *TeamModel) query(filters Filters, query string, args []any) ([]types.TeamID, Metadata, error) {
	// Create a context with a 3-second timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, args...)
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
	metadata := calculateMetadata(filters.Year, totalRecords, filters.Page, filters.PageSize)

	return teamIDs, metadata, nil
}

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

type Patrulje struct {
	ID          types.TeamID `json:"id"`
	Number      string       `json:"number"`
	Status      string       `json:"status"`
	Name        string       `json:"name"`
	Group       string       `json:"group"`
	Korps       string       `json:"korps"`
	Liga        string       `json:"liga"`
	MemberCount int          `json:"memberCount"`
}
type Klan struct {
	ID          types.TeamID       `json:"id"`
	Status      types.SignupStatus `json:"status"`
	Name        string             `json:"name"`
	Group       string             `json:"group"`
	Korps       string             `json:"korps"`
	MemberCount int                `json:"memberCount"`
}
type Contact struct {
	TeamID     types.TeamID       `json:"teamId"`
	Name       string             `json:"name"`
	Address    string             `json:"address"`
	PostalCode string             `json:"postal"`
	Email      types.EmailAddress `json:"email"`
	Phone      types.PhoneNumber  `json:"phone"`
	Role       string             `json:"role"`
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

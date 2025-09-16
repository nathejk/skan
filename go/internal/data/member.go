package data

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	"github.com/nathejk/shared-go/types"
	"nathejk.dk/internal/validator"
)

type Member struct {
}

func (p *Member) Validate(v validator.Validator) {
	//v.Check(p.Timestamp.IsZero(), "timestamp", "must be provided")
}

type MemberModel struct {
	DB *sql.DB
}

type Spejder struct {
	ID            types.MemberID     `json:"id"`
	MemberID      types.MemberID     `json:"memberId"`
	InitialTeamID types.TeamID       `json:"teamId"`
	CurrentTeamID types.TeamID       `json:"teamId"`
	Status        types.MemberStatus `json:"status"`
	Name          string             `json:"name"`
	Address       string             `json:"address"`
	PostalCode    string             `json:"postalCode"`
	City          string             `json:"city"`
	Email         string             `json:"email"`
	Phone         string             `json:"phone"`
	PhoneParent   string             `json:"phoneParent"`
	Birthday      types.Date         `json:"birthday"`
	Returning     bool               `json:"returning"`
	TShirtSize    string             `json:"tshirtSize"`
}

func (m MemberModel) GetSpejdere(filters Filters) ([]*Spejder, Metadata, error) {
	// Create a context with a 3-second timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `Select 
  s.memberId, 
  s.teamId, 
  IF(ss.status IS NULL, IF(ps.startedUts > 0, 'started', 'paid'), ss.status) AS status,
  name,
  address,
  postalCode,
  city,
  email,
  phone,
  phoneParent,
  birthday,
  ` + "`returning`" + `,
  tshirtsize
from spejder s
join patruljestatus ps on s.teamId = ps.teamId
left join spejderstatus ss on s.memberId = ss.id and s.year = ss.year
WHERE  (LOWER(s.year) = LOWER(?) OR ? = '') AND  (s.teamId = ? OR ? = '')`
	args := []any{filters.Year, filters.Year, filters.TeamID, filters.TeamID}
	rows, err := m.DB.QueryContext(ctx, query, args...)
	if err != nil {
		log.Print(err)
		return nil, Metadata{}, err
	}
	defer rows.Close()

	totalRecords := 0
	spejdere := []*Spejder{}
	for rows.Next() {
		var s Spejder
		if err := rows.Scan(&s.ID, &s.InitialTeamID, &s.Status, &s.Name, &s.Address, &s.PostalCode, &s.City, &s.Email, &s.Phone, &s.PhoneParent, &s.Birthday, &s.Returning, &s.TShirtSize); err != nil {
			log.Print(err)
			return nil, Metadata{}, err
		}
		s.MemberID = s.ID
		spejdere = append(spejdere, &s)
	}
	// When the rows.Next() loop has finished, call rows.Err() to retrieve any error
	// that was encountered during the iteration.
	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}
	metadata := calculateMetadata(filters.Year, totalRecords, filters.Page, filters.PageSize)

	return spejdere, metadata, nil
}

type Senior struct {
	ID         types.MemberID `json:"id"`
	MemberID   types.MemberID `json:"memberId"`
	TeamID     types.TeamID   `json:"teamId"`
	Name       string         `json:"name"`
	Address    string         `json:"address"`
	PostalCode string         `json:"postalCode"`
	City       string         `json:"city"`
	Email      string         `json:"email"`
	Phone      string         `json:"phone"`
	Birthday   types.Date     `json:"birthday"`
	Diet       string         `json:"diet"`
	TShirtSize string         `json:"tshirtSize"`
}

func (m MemberModel) GetSeniore(filters Filters) ([]*Senior, Metadata, error) {
	// Create a context with a 3-second timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `Select 
  s.memberId, 
  s.teamId, 
  name,
  address,
  postalCode,
  city,
  email,
  phone,
  birthday,
  diet,
  tshirtsize
from senior s
WHERE  s.teamId = ?`
	args := []any{filters.TeamID}
	rows, err := m.DB.QueryContext(ctx, query, args...)
	if err != nil {
		log.Print(err)
		return nil, Metadata{}, err
	}
	defer rows.Close()

	totalRecords := 0
	members := []*Senior{}
	for rows.Next() {
		var s Senior
		if err := rows.Scan(&s.ID, &s.TeamID, &s.Name, &s.Address, &s.PostalCode, &s.City, &s.Email, &s.Phone, &s.Birthday, &s.Diet, &s.TShirtSize); err != nil {
			log.Print(err)
			return nil, Metadata{}, err
		}
		s.MemberID = s.ID
		members = append(members, &s)
	}
	// When the rows.Next() loop has finished, call rows.Err() to retrieve any error
	// that was encountered during the iteration.
	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}
	metadata := calculateMetadata(filters.Year, totalRecords, filters.Page, filters.PageSize)

	return members, metadata, nil
}

type SpejderStatus struct {
	MemberID  types.MemberID
	TeamID    types.TeamID
	Status    types.MemberStatus
	Name      string
	TeamName  string
	UpdatedAt time.Time
}

func (m MemberModel) GetInactive(filters Filters) ([]*SpejderStatus, Metadata, error) {
	// Create a context with a 3-second timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
	select s.memberId, s.name, s.teamId, p.name, ss.status, ss.updatedAt
from spejder s
join patrulje p on s.teamId = p.teamId
join spejderstatus ss on s.memberId = ss.id and s.year = ss.year
WHERE (LOWER(s.year) = LOWER(?) OR ? = '')`

	args := []any{filters.Year, filters.Year, filters.TeamID, filters.TeamID}
	rows, err := m.DB.QueryContext(ctx, query, args...)
	if err != nil {
		log.Print(err)
		return nil, Metadata{}, err
	}
	defer rows.Close()

	totalRecords := 0
	sss := []*SpejderStatus{}
	for rows.Next() {
		var s SpejderStatus
		if err := rows.Scan(&s.MemberID, &s.Name, &s.TeamID, &s.TeamName, &s.Status, &s.UpdatedAt); err != nil {
			return nil, Metadata{}, err
		}
		sss = append(sss, &s)
	}
	// When the rows.Next() loop has finished, call rows.Err() to retrieve any error
	// that was encountered during the iteration.
	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}
	metadata := calculateMetadata(filters.Year, totalRecords, filters.Page, filters.PageSize)

	return sss, metadata, nil
}

func (m TeamModel) GetSpejder(teamID types.TeamID) (*Patrulje, error) {
	if len(teamID) == 0 {
		return nil, ErrRecordNotFound
	}

	query := `SELECT p.teamId, p.teamNumber, p.name, p.groupName, p.korps, p.memberCount, IF(pm.parentTeamId IS NOT NULL, "JOIN", IF(startedUts > 0, "STARTED",  signupStatus)) 
		FROM patrulje p 
		JOIN patruljestatus ps ON p.teamId = ps.teamID
		LEFT JOIN patruljemerged pm ON p.teamId = pm.teamId
		WHERE p.teamId = ?`
	var p Patrulje
	err := m.DB.QueryRow(query, teamID).Scan(
		&p.ID,
		&p.Number,
		&p.Name,
		&p.Group,
		&p.Korps,
		&p.MemberCount,
		&p.Status,
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

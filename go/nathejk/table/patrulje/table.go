package patrulje

import (
	"database/sql"
	"log"

	"github.com/jrgensen/cqrs"
	"github.com/nathejk/shared-go/types"

	_ "embed"
)

type Patrulje struct {
	TeamID       types.TeamID       `json:"teamId"`
	TeamNumber   string             `json:"teamNumber"`
	Year         string             `json:"year"`
	Name         string             `json:"name"`
	Group        string             `json:"group"`
	Korps        string             `json:"korps"`
	Liga         string             `json:"liga"`
	ContactName  string             `json:"contactName"`
	ContactPhone types.PhoneNumber  `json:"contactPhone"`
	ContactEmail types.EmailAddress `json:"contactEmail"`
	ContactRole  string             `json:"contactRole"`
	MemberCount  int                `json:"memberCount"`
	TshirtCount  int                `json:"tshirtCount"`
	SignupStatus types.SignupStatus `json:"signupStatus"`
	PaidAmount   int                `json:"paidAmount"`

	// MergedIntoTeamID is the team this one was merged into, or "" while it is still
	// running. A merge is how a patrol leaves the race.
	MergedIntoTeamID types.TeamID `json:"mergedIntoTeamId,omitempty"`
}

// Discontinued reports whether the patrol has left the race.
//
// A method so the rule is written once: "discontinued" is a merge into another team, not
// a signup status — the stream carries no patrulje status change for it. Its remaining
// members were reassigned, and they may well have taken their map with them.
func (p Patrulje) Discontinued() bool { return p.MergedIntoTeamID != "" }

type table struct {
	consumer
	querier
}

func New(w cqrs.Writer, r *sql.DB) *table {
	table := &table{consumer: consumer{w: w}, querier: querier{db: r}}
	if err := w.Consume(table.CreateTableSql()); err != nil {
		log.Fatalf("Error creating table %q", err)
	}
	return table
}

//go:embed table.sql
var tableSchema string

func (t *table) CreateTableSql() string {
	return tableSchema
}

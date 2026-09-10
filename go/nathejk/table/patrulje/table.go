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

	// ActiveMemberCount is how many members are still on the route, maintained by the
	// spejderstatus projection.
	ActiveMemberCount int `json:"activeMemberCount"`
}

// Discontinued reports whether the patrol has left the race.
//
// A started team with no active members left. Both halves matter: a team that has not
// started yet has no active members either, and calling that "discontinued" would treat
// every patrol in the hours before the start as having dropped out.
//
// A method so the rule is written once, and so the rule can change without every caller
// changing with it — it has already moved from the deprecated `patruljemerged` encoding to
// this one. There is deliberately no event for it: the count is recomputed from the member
// rows, so moving a member back in makes the team active again with no reverse event.
func (p Patrulje) Discontinued() bool {
	return p.SignupStatus == types.SignupStatusStarted && p.ActiveMemberCount == 0
}

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

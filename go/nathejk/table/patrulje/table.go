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

	// Remark is HQ's operational note about this patrol, shown to whoever scans it;
	// RemarkSeverity says how loudly. Both are written by hq — see messages.go.
	Remark         string `json:"remark"`
	RemarkSeverity string `json:"remarkSeverity"`
}

// RemarkInForce reports whether there is a note to show at all.
//
// An empty remark is off regardless of severity: there is nothing to display, and an empty
// box would be worse than no box. `inactive` is a note HQ deliberately stood down, so it is
// off too — the text is kept only so it can be put back without retyping.
func (p Patrulje) RemarkInForce() bool {
	if p.Remark == "" {
		return false
	}
	return p.RemarkSeverity == RemarkSeverityInformation || p.RemarkSeverity == RemarkSeverityStop
}

// RemarkStops reports whether the note is a "fuld stop": the patrol must not be sent on.
func (p Patrulje) RemarkStops() bool {
	return p.RemarkInForce() && p.RemarkSeverity == RemarkSeverityStop
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
	for _, stmt := range schemaMigrations {
		if err := w.Consume(stmt); err != nil {
			log.Fatalf("Error migrating patrulje table %q", err)
		}
	}
	return table
}

// schemaMigrations brings an existing database up to the current shape.
//
// CREATE TABLE IF NOT EXISTS is a no-op wherever the table already exists, so a column added
// to table.sql is silently absent from every database that has booted once — and the remark
// consumer's UPDATE would then fail with "Unknown column" on every note HQ writes. Entries
// run on every boot and must be idempotent; `IF NOT EXISTS` is MariaDB's and is what makes
// that safe.
var schemaMigrations = []string{
	`ALTER TABLE patrulje ADD COLUMN IF NOT EXISTS remark TEXT NOT NULL DEFAULT ""`,
	`ALTER TABLE patrulje ADD COLUMN IF NOT EXISTS remarkSeverity VARCHAR(20) NOT NULL DEFAULT ""`,
}

//go:embed table.sql
var tableSchema string

func (t *table) CreateTableSql() string {
	return tableSchema
}

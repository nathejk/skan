package spejderstatus

import (
	"database/sql"
	"log"
	"time"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/mysql"
	"github.com/jrgensen/cqrs"
	"github.com/jrgensen/stream"
	"github.com/nathejk/shared-go/types"

	_ "embed"
)

// SpejderStatus is one member's place in the lifecycle.
//
// InitialTeamID and CurrentTeamID are both kept, and the difference between them
// is the interesting part: a member moved to another patrol races on with a team
// that is not the one they signed up with, and both halves of that matter later —
// the current team for strength and discontinuation, the initial one for results
// and for answering "whose patrol was this?" months afterwards.
type SpejderStatus struct {
	MemberID      types.MemberID     `json:"memberId"`
	YearSlug      types.YearSlug     `json:"year"`
	InitialTeamID types.TeamID       `json:"initialTeamId"`
	CurrentTeamID types.TeamID       `json:"currentTeamId"`
	Status        types.MemberStatus `json:"status"`
	UpdatedAt     time.Time          `json:"updatedAt"`
}

// InOurCare reports whether Nathejk is currently responsible for this member's
// whereabouts. Delegates to shared-go rather than comparing statuses here, so the
// set has exactly one definition.
func (s SpejderStatus) InOurCare() bool { return s.Status.InOurCare() }

type table struct {
	commander
	consumer
	querier
}

// New builds the projection.
//
// It takes the publisher for the commander's sake; the writer is the consumer's and
// the *sql.DB the querier's.
func New(p stream.Publisher, w cqrs.Writer, r *sql.DB) *table {
	q := querier{db: r, r: goqu.New("mysql", r)}
	t := &table{
		commander: commander{p: p, q: &q},
		consumer:  consumer{w: w},
		querier:   q,
	}
	if err := w.Consume(t.CreateTableSql()); err != nil {
		log.Printf("Error creating table %q", err)
	}
	for _, stmt := range schemaMigrations {
		if err := w.Consume(stmt); err != nil {
			log.Printf("Error migrating spejderstatus tables %q", err)
		}
	}
	return t
}

// schemaMigrations brings an existing database up to the current shape, because
// CREATE TABLE IF NOT EXISTS is a no-op wherever the table already exists — so editing
// table.sql alone only ever affects databases that get cleared before deploy.
//
// The one entry here fixes a key that was wrong for a few minutes and would have stayed
// wrong forever. `spejderstatuslog` was first written with PRIMARY KEY (seq), and the dev
// container's hot-reload created it in the window before the member id was added to the
// key. The consequence was invisible and specific: **one event can concern many members**
// — a patrol starting puts its whole roster into `racing` from a single message — so with a
// seq-only key the first member written inserted and every other silently hit
// ON CONFLICT DO NOTHING. Members simply had no "startede løbet" line in their history,
// with nothing logged anywhere.
//
// Written as DROP + ADD in one statement so it is idempotent: run against the broken shape
// it fixes it, run against the correct shape it rebuilds the same key. That costs a key
// rebuild per boot on a table of a few thousand rows, which is cheaper than a schema that
// heals only on databases somebody remembered to drop.
var schemaMigrations = []string{
	`ALTER TABLE spejderstatuslog DROP PRIMARY KEY, ADD PRIMARY KEY (seq, id)`,
}

//go:embed table.sql
var tableSchema string

func (t *table) CreateTableSql() string { return tableSchema }

// Assert the consumer contract at compile time. The mux accepts anything shaped
// vaguely right, so a signature drifting out of step would otherwise surface as a
// projection that silently never runs — and a member projection that never runs
// looks exactly like an event with nobody in our care.
var _ cqrs.Consumer = (*table)(nil)

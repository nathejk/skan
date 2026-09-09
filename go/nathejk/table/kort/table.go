// Package kort is the maps we print and hand out during the event: what each sheet is called,
// which set it belongs to, what ground it shows, and which checkpoints are drawn on it (PRD 010).
//
// # Why a map is an entity
//
// The tempting shortcut is to say the reveal unit is the checkgroup, which HQ already models, and
// skip this package. It does not hold. A skitse shows a subset of one group; a double-sided A3
// spans two. The thing a patrol holds is a *sheet*, and the sheet is what a post hands over, so
// the sheet is what gets modelled. The checkgroup then stays what it is — a timing and staffing
// concept — instead of being quietly overloaded with a printing concern.
//
// Both are reveal units, and they do not nest: every checkpoint in a checkgroup is revealed at
// once, *and* the checkpoints on a sheet are revealed when its QR is linked to a team. This
// package makes both expressible and implements neither — the rule lives in the app that asks
// the question (PRD 010 §8).
//
// # What this package deliberately cannot answer
//
// "Which maps contain checkpoint X?" There is no index for it and no query for it, because
// checkpoints are a JSON array on the row rather than a join table. That is a decision, not an
// omission (see table.sql), and the one place it costs anything is the delete cascade.
//
// "Where was this sheet handed out?" Still nothing records that, and nothing should: what actually
// happened varies and is not reliably known. HandoutCheckgroupID is the *plan*, not the record —
// see its doc comment for why the distinction matters.
package kort

import (
	"database/sql"
	"log"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/mysql"
	"github.com/google/uuid"
	"github.com/jrgensen/cqrs"
	"github.com/jrgensen/stream"
	"github.com/nathejk/shared-go/types"

	_ "embed"
)

// KortID identifies one printed sheet.
//
// Minted by the server, like spejdernote's NoteID: a client cannot collide with an id it has not
// seen.
type KortID string

func NewKortID() KortID {
	return KortID("kort-" + uuid.New().String())
}

// KortsaetID identifies a set of sheets (task 122).
//
// Declared here rather than in the set's own file so that Created can name it without the two
// halves of this package having to be written in one go.
type KortsaetID string

func NewKortsaetID() KortsaetID {
	return KortsaetID("kortsaet-" + uuid.New().String())
}

// Kort is one printed sheet.
//
// CheckpointIDs and Extents are always non-nil after a read, so every caller — and the JSON
// encoder serving the SPA — sees `[]` rather than `null` for an empty one.
type Kort struct {
	KortID     KortID         `json:"id"`
	KortsaetID KortsaetID     `json:"kortsaetId"`
	YearSlug   types.YearSlug `json:"year"`
	Version    uint64         `json:"version"`

	Name      string `json:"name"`
	Format    Format `json:"format"`
	Note      string `json:"note"`
	SortOrder int    `json:"sortOrder"`

	// HandoutCheckgroupID is where the sheet is *planned* to be handed out: the checkgroup whose
	// post gives it to the team, or empty for "at the scan of this sheet's QR code".
	//
	// It is the reveal trigger a consuming app needs (PRD 010 §8): the sheet's checkpoints become
	// visible to the scout either when the group named here is reached, or when the QR is linked to
	// the team. Empty is the default and the pre-existing rule, so a sheet nobody has configured
	// behaves as it always did.
	//
	// Empty also means "no longer resolvable": a checkgroup deleted after a sheet pointed at it is
	// filtered to "" on read, the same way unknown checkpoint ids are (querier.Maps). A reader must
	// therefore treat empty as the QR rule, never as "unknown".
	HandoutCheckgroupID types.CheckgroupID `json:"handoutCheckgroupId"`

	// CheckpointIDs is what a consuming app is really here for: the checkpoints drawn on this
	// sheet, and therefore what may be revealed when the sheet is known to be in a team's hands.
	CheckpointIDs []types.CheckpointID `json:"checkpointIds"`

	// Extents is the ground the sheet shows — zero rectangles for a skitse, one for a normal
	// sheet, two for a double-sided one. Front and back are not distinguished; they are simply
	// two areas.
	Extents []Extent `json:"extents"`
}

// HasCheckpoint reports whether this sheet shows the given checkpoint.
//
// A method on the row rather than a query, which is the whole shape of this package: the
// question is only ever asked about maps already in hand, and answering it in SQL would need
// the index the JSON column deliberately does not have.
func (k Kort) HasCheckpoint(id types.CheckpointID) bool {
	for _, cp := range k.CheckpointIDs {
		if cp == id {
			return true
		}
	}
	return false
}

// ContainsAll reports whether every one of the given checkpoints is on this sheet.
//
// This is the primitive behind the split-checkgroup warning (task 133), and it is deliberately
// existential rather than partitioning: the warning fires when *no single* map contains a whole
// checkgroup, so two overlapping sheets that both contain it are fine. Overlap between adjacent
// sheets is designed in, so a test that treated it as an error would fire constantly and be
// ignored.
//
// An empty list is contained by every sheet, which is the right answer for a checkgroup with no
// checkpoints yet: it is not a coverage failure, it is an unfinished course.
func (k Kort) ContainsAll(ids []types.CheckpointID) bool {
	for _, id := range ids {
		if !k.HasCheckpoint(id) {
			return false
		}
	}
	return true
}

// Kortsaet is a set of sheets — most often "Patruljer" and one for everybody else.
type Kortsaet struct {
	KortsaetID KortsaetID     `json:"id"`
	YearSlug   types.YearSlug `json:"year"`
	Version    uint64         `json:"version"`

	Name      string `json:"name"`
	SortOrder int    `json:"sortOrder"`

	// TeamType is the team type this set is *specifically for*, or nil.
	//
	// Read it as "this set is for patruljer", not "only patruljer use it": an unmarked set is the
	// general one, and klaner normally draw from the crew set. So a typical year has one set
	// marked patrulje and one unmarked, and `klan` may never be used at all.
	//
	// The consequence for callers, worth stating because it is the obvious bug: filtering by
	// `klan` will usually return nothing, and that is **not** an error. Fall back to the unmarked
	// set rather than concluding a team type has no maps.
	TeamType *types.TeamType `json:"teamType"`

	// Maps is the set's sheets in handout order, filled in by Nest.
	//
	// No `omitempty`: a set with no sheets must serialize as `kort: []`, not vanish from the
	// payload. A set an operator has just created is exactly that, and a key that comes and goes
	// would make every client handle absence as well as emptiness.
	Maps []Kort `json:"kort"`
}

// ForTeamType reports whether this set is specifically for the given team type.
//
// A method so that the nil-means-general rule is written once. Every caller that reimplemented
// it would eventually reimplement it as `*s.TeamType == t` and panic on the crew set.
func (s Kortsaet) ForTeamType(t types.TeamType) bool {
	return s.TeamType != nil && *s.TeamType == t
}

type table struct {
	commander
	consumer
	querier
}

// New builds the projection.
//
// The publisher is only used by the commander, the writer only by the consumer and the *sql.DB
// only by the querier — and the querier is handed to the commander, which dirty-checks against
// it and refuses to delete a set that still holds sheets.
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
	if err := w.Consume(kortsaetSchema); err != nil {
		log.Printf("Error creating table %q", err)
	}
	for _, stmt := range schemaMigrations {
		if err := w.Consume(stmt); err != nil {
			log.Printf("Error migrating kort tables %q", err)
		}
	}
	return t
}

// schemaMigrations brings an existing database up to the current shape.
//
// Empty today and present from the start, because CREATE TABLE IF NOT EXISTS is a no-op wherever
// the table already exists: a column added to table.sql later is silently absent from every
// database that has already booted once (`.rules`, and the way spejderstatus learned it).
//
// This package will need it sooner than most — the set table arrives in task 122 and the columns
// it references are already declared here — so entries must be idempotent: they run on every
// boot, against both the old shape and the new.
var schemaMigrations = []string{
	// handoutCheckgroupId. `IF NOT EXISTS` is MariaDB's, and is what makes this safe to
	// run on every boot; the default matches table.sql, so a pre-existing row means "revealed at
	// the QR scan" — which is what it meant before the column existed.
	`ALTER TABLE kort ADD COLUMN IF NOT EXISTS handoutCheckgroupId VARCHAR(99) NOT NULL DEFAULT ""`,
}

//go:embed table.sql
var tableSchema string

//go:embed kortsaet.sql
var kortsaetSchema string

func (t *table) CreateTableSql() string { return tableSchema }

// Assert the consumer contract at compile time. The mux accepts anything shaped vaguely right,
// so a signature drifting out of step would surface as a projection that silently never runs —
// and an empty kort table looks exactly like a year nobody has drawn the maps for yet.
var _ cqrs.Consumer = (*table)(nil)

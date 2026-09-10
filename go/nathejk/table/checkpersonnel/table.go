package checkpersonnel

import (
	"database/sql"
	"log"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/jrgensen/cqrs"
	"github.com/jrgensen/stream"
	"github.com/nathejk/shared-go/types"

	_ "embed"
)

type CheckpointScan struct {
	ScanID types.ScanID `json:"scanId"`
	TeamID types.TeamID `json:"teamId"`
	Time   time.Time    `json:"time"`
}
type Checkpersonnel struct {
	ID           types.CheckpersonnelID `json:"id" db:"id"`
	Year         types.YearSlug         `json:"year" db:"year"`
	CheckgroupID types.CheckgroupID     `json:"checkgroupId" db:"checkgroupId"`
	CheckpointID types.CheckpointID     `json:"checkpointId" db:"checkpointId"`
	UserID       types.UserID           `json:"userId" db:"userId"`
	Start        time.Time              `json:"start" db:"startUts"`
	End          time.Time              `json:"end" db:"endUts"`
	Scans        []CheckpointScan       `json:"scans"`
}

// CheckPersonnel is an alias kept for backward compatibility with external references.
type CheckPersonnel = Checkpersonnel

type table struct {
	commander
	consumer
	querier
}

func New(p stream.Publisher, w cqrs.Writer, r *sql.DB) *table {
	q := querier{db: r, r: goqu.New("mysql", r)}
	table := &table{commander: commander{p: p, q: &q}, consumer: consumer{w: w}, querier: q}
	if err := w.Consume(table.CreateTableSql()); err != nil {
		log.Printf("Error creating table %q", err)
	}
	return table
}

//go:embed table.sql
var tableSchema string

func (t *table) CreateTableSql() string {
	return tableSchema
}

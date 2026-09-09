package scan

import (
	"database/sql"
	"log"

	"github.com/jrgensen/cqrs"
	"github.com/nathejk/shared-go/types"

	_ "embed"
)

type Scan struct {
	QrID         types.QrID   `json:"qrId"`
	TeamID       types.TeamID `json:"teamId"`
	TeamNumber   int          `json:"teamNumber"`
	ScannerID    string       `json:"scannerId"`
	ScannerPhone string       `json:"scannerPhone"`
	Uts          int64        `json:"uts"`
	Latitude     string       `json:"latitude"`
	Longitude    string       `json:"longitude"`

	// LocationSource is "gps", "manual", or "" for scans recorded before it was
	// tracked. Empty must not be presented as GPS.
	LocationSource string `json:"locationSource,omitempty"`

	// LocationAccuracy is the radius of confidence in metres, or "" when unknown.
	LocationAccuracy string `json:"locationAccuracy,omitempty"`
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

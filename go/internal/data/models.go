package data

import (
	"context"
	"errors"

	"github.com/nathejk/shared-go/types"
	"nathejk.dk/nathejk/table/klan"
	"nathejk.dk/nathejk/table/patrulje"
	"nathejk.dk/nathejk/table/personnel"
	"nathejk.dk/nathejk/table/photo"
	"nathejk.dk/nathejk/table/qr"
	"nathejk.dk/nathejk/table/scan"
	"nathejk.dk/nathejk/table/senior"
)

var (
	ErrRecordNotFound = errors.New("record not found")
	ErrEditConflict   = errors.New("edit conflict")
)

type KlanInterface interface {
	GetAll(context.Context, klan.Filter) ([]klan.Klan, error)
	GetByID(context.Context, types.TeamID) (*klan.Klan, error)
}
type PatruljeInterface interface {
	GetAll(context.Context, patrulje.Filter) ([]*patrulje.Patrulje, error)
	GetByID(context.Context, types.TeamID) (*patrulje.Patrulje, error)
	GetByNumber(ctx context.Context, yearSlug string, teamNumber int) (*patrulje.Patrulje, error)
}
type SeniorInterface interface {
	GetAll(context.Context, senior.Filter) ([]*senior.Senior, senior.Metadata, error)
	GetByID(context.Context, types.MemberID) (*senior.Senior, error)
	GetByPhone(ctx context.Context, yearSlug string, phone types.PhoneNumber) (*senior.Senior, error)
}
type PersonnelInterface interface {
	GetAll(context.Context, personnel.Filter) ([]*personnel.Person, error)
	GetByID(context.Context, types.UserID) (*personnel.Person, error)
	GetByPhone(ctx context.Context, yearSlug string, phone types.PhoneNumber) (*personnel.Person, error)
}
type QrInterface interface {
	GetByID(ctx context.Context, yearSlug string, qrID types.QrID) (*qr.QR, error)
}
type ScanInterface interface {
	GetAll(context.Context, scan.Filter) ([]*scan.Scan, error)
}

// PhotoInterface is the subset of the photo projection this service reads. Only
// the per-team list: skan never serves bytes (the foto service does) and must
// never touch originals, so Servable and OriginalRefs are deliberately absent.
type PhotoInterface interface {
	ByTeam(ctx context.Context, year, teamID string) ([]photo.Photo, error)
}

// PhotoCoverInterface is the organizer's explicit choice of cover photograph.
// Only the single-team read; the whole-year Covers query exists for hq's patrol
// lists, which this service does not render.
type PhotoCoverInterface interface {
	Ref(year, teamID string) (string, error)
}

// Models is the read side as handlers see it: one interface per projection this
// service actually reads.
type Models struct {
	Klan       KlanInterface
	Senior     SeniorInterface
	Patrulje   PatruljeInterface
	Personnel  PersonnelInterface
	QR         QrInterface
	Scan       ScanInterface
	Photo      PhotoInterface
	PhotoCover PhotoCoverInterface
}

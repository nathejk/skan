package data

import (
	"context"
	"errors"

	"github.com/nathejk/shared-go/types"
	"nathejk.dk/nathejk/table/klan"
	"nathejk.dk/nathejk/table/patrulje"
	"nathejk.dk/nathejk/table/personnel"
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
	GetByNumber(context.Context, int) (*patrulje.Patrulje, error)
}
type SeniorInterface interface {
	GetAll(context.Context, senior.Filter) ([]*senior.Senior, senior.Metadata, error)
	GetByID(context.Context, types.MemberID) (*senior.Senior, error)
	GetByPhone(context.Context, types.PhoneNumber) (*senior.Senior, error)
}
type PersonnelInterface interface {
	GetAll(context.Context, personnel.Filter) ([]*personnel.Person, error)
	GetByID(context.Context, types.UserID) (*personnel.Person, error)
	GetByPhone(context.Context, types.PhoneNumber) (*personnel.Person, error)
}
type QrInterface interface {
	GetByID(context.Context, types.QrID) (*qr.QR, error)
}
type ScanInterface interface {
	GetAll(context.Context, scan.Filter) ([]*scan.Scan, error)
}

// Models is the read side as handlers see it: one interface per projection this
// service actually reads.
type Models struct {
	Klan      KlanInterface
	Senior    SeniorInterface
	Patrulje  PatruljeInterface
	Personnel PersonnelInterface
	QR        QrInterface
	Scan      ScanInterface
}

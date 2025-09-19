package data

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/nathejk/shared-go/types"
	"nathejk.dk/nathejk/table/klan"
	"nathejk.dk/nathejk/table/patrulje"
	"nathejk.dk/nathejk/table/payment"
	"nathejk.dk/nathejk/table/personnel"
	"nathejk.dk/nathejk/table/qr"
	"nathejk.dk/nathejk/table/senior"
	"nathejk.dk/nathejk/table/spejder"
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
type PaymentInterface interface {
	GetAll(context.Context, types.TeamID) ([]*payment.Payment, error)
	GetByReference(context.Context, string) (*payment.Payment, error)
}
type SpejderInterface interface {
	GetAll(context.Context, spejder.Filter) ([]*spejder.Spejder, spejder.Metadata, error)
	GetByID(context.Context, types.MemberID) (*spejder.Spejder, error)
}
type QrInterface interface {
	GetByID(context.Context, types.QrID) (*qr.QR, error)
}

type Models struct {
	Teams interface {
		GetStartedTeamIDs(Filters) ([]types.TeamID, Metadata, error)
		GetDiscontinuedTeamIDs(Filters) ([]types.TeamID, Metadata, error)
		GetPatruljer(Filters) ([]*Patrulje, Metadata, error)
		GetPatrulje(types.TeamID) (*Patrulje, error)
		GetKlan(types.TeamID) (*Klan, error)
		GetContact(types.TeamID) (*Contact, error)
		RequestedSeniorCount() int
	}
	Members interface {
		GetSpejdere(Filters) ([]*Spejder, Metadata, error)
		GetSeniore(Filters) ([]*Senior, Metadata, error)
		GetInactive(Filters) ([]*SpejderStatus, Metadata, error)
	}
	Permissions interface {
		AddForUser(int64, ...string) error
		GetAllForUser(int64) (Permissions, error)
	}
	Tokens interface {
		New(userID int64, ttl time.Duration, scope string) (*Token, error)
		Insert(token *Token) error
		DeleteAllForUser(scope string, userID int64) error
	}
	Users interface {
		Insert(*User) error
		GetByEmail(string) (*User, error)
		Update(*User) error
		GetForToken(string, string) (*User, error)
	}
	Signup interface {
		GetByID(types.TeamID) (*Signup, error)
		ConfirmBySecret(string) (types.TeamID, error)
	}
	Klan      KlanInterface
	Senior    SeniorInterface
	Patrulje  PatruljeInterface
	Personnel PersonnelInterface
	Payment   PaymentInterface
	Spejder   SpejderInterface
	QR        QrInterface
}

func NewModels(db *sql.DB, klan KlanInterface, patrulje PatruljeInterface, personnel PersonnelInterface, payment PaymentInterface, spejder SpejderInterface) Models {
	return Models{
		Teams:       TeamModel{DB: db},
		Members:     MemberModel{DB: db},
		Permissions: PermissionModel{DB: db},
		Tokens:      TokenModel{DB: db},
		Users:       UserModel{DB: db},
		Signup:      SignupModel{DB: db},
		Klan:        klan,
		Patrulje:    patrulje,
		Personnel:   personnel,
		Payment:     payment,
		Spejder:     spejder,
	}
}

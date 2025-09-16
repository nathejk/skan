package commands

import (
	"github.com/nathejk/shared-go/messages"
	"github.com/nathejk/shared-go/types"
	"nathejk.dk/internal/data"
	"nathejk.dk/superfluids/streaminterface"
)

type Commands struct {
	Team interface {
		Signup(types.TeamType, *messages.NathejkTeamSignedUp) error
		UpdatePatrulje(types.TeamID, Patrulje, Contact, []Spejder) error
		StartPatrulje(types.TeamID, []StartPatruljeMember) error
		UpdateKlan(types.TeamID, Klan, []Senior) error
		AssignToLok(types.TeamID, string) error
	}
}

func New(stream streaminterface.Publisher, models data.Models) Commands {
	return Commands{
		Team: NewTeam(stream, models.Teams),
	}
}

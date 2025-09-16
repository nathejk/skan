package commands

import "github.com/nathejk/shared-go/types"

type Patrulje struct {
	TeamID          types.TeamID `json:"teamId"`
	Name            string       `json:"name"`
	Group           string       `json:"group"`
	Korps           string       `json:"korps"`
	AdventureLigaID string       `json:"liga"`
}

type Klan struct {
	TeamID      types.TeamID `json:"teamId"`
	Name        string       `json:"name"`
	Group       string       `json:"group"`
	Korps       string       `json:"korps"`
	MemberCount int          `json:"memberCount"`
}

type Contact struct {
	TeamID     types.TeamID       `json:"teamId"`
	Name       string             `json:"name"`
	Address    string             `json:"address"`
	PostalCode string             `json:"postal"`
	Email      types.EmailAddress `json:"email"`
	Phone      types.PhoneNumber  `json:"phone"`
	Role       string             `json:"role"`
}

type Spejder struct {
	MemberID     types.MemberID     `json:"memberId"`
	Deleted      bool               `json:"deleted"`
	Started      bool               `json:"started"`
	Name         string             `json:"name"`
	Address      string             `json:"address"`
	PostalCode   string             `json:"postalCode"`
	Email        types.EmailAddress `json:"email"`
	Phone        types.PhoneNumber  `json:"phone"`
	PhoneContact types.PhoneNumber  `json:"phoneCantact"`
	Birthday     types.Date         `json:"birthday"`
	TShirtSize   string             `json:"tshirtsize"`
}

type Senior struct {
	MemberID   types.MemberID     `json:"memberId"`
	Deleted    bool               `json:"deleted"`
	Name       string             `json:"name"`
	Address    string             `json:"address"`
	PostalCode string             `json:"postalCode"`
	Email      types.EmailAddress `json:"email"`
	Phone      types.PhoneNumber  `json:"phone"`
	Birthday   types.Date         `json:"birthday"`
	Diet       string             `json:"diet"`
	TShirtSize string             `json:"tshirtsize"`
}

package login

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/nathejk/shared-go/tables"
	"github.com/nathejk/shared-go/types"
	"nathejk.dk/internal/data"
)

var CookieName string = "user"

// Role is what a scanner is, for this event.
//
// Derived from the phone number alone. The pre-port PHP app let people hold several
// functions and choose one per session; that concept is gone and must not come back.
type Role string

const (
	// RoleCrew is event personnel: checkpoint staff, guides, samaritter. Not players,
	// and allowed to see everything.
	RoleCrew Role = "crew"

	// RoleBandit is a senior signed up to a klan. "Senior" and "bandit" are the same
	// people: the signup word and the race word. Bandits are players, so they may
	// only learn about bandits.
	RoleBandit Role = "bandit"
)

// IsBandit reports whether the scanner is a player, and so subject to the
// fair-game restrictions on what a scan result may reveal.
func (u User) IsBandit() bool { return u.Role == RoleBandit }

type User struct {
	ID    types.UserID
	Phone types.PhoneNumber
	Type  types.TeamType
	Role  Role
}

// Why logins are refused. Both are shown to the scanner in Danish, because both are
// things only HQ can resolve.
var (
	// ErrUnknownPhone means the number is not signed up for this year, as either crew
	// or senior.
	ErrUnknownPhone = errors.New("login: phone number is not registered for this year")

	// ErrAmbiguousRole means the number is registered as both crew and senior in the
	// same year. That is a data error upstream, not something to disambiguate here:
	// guessing would either hand a bandit the crew view of the race, or deny a crew
	// member information they need.
	ErrAmbiguousRole = errors.New("login: phone number is registered as both crew and senior")
)

// PageData is what the login page needs rendered.
type PageData struct {
	// Path is where to send the scanner after a successful login.
	Path string

	// Error is a Danish explanation, empty on a first visit.
	Error string
}

// Renderer draws the login page. Injected because the templates live in the main
// package, and this package must not grow a dependency on them.
type Renderer func(w http.ResponseWriter, r *http.Request, page PageData)

type auth struct {
	models data.Models
	render Renderer

	// yearSlug scopes every lookup. People sign up again each year, so a number
	// matches rows from several events and the role must be read from this one.
	yearSlug string
}

func New(models data.Models, yearSlug string, render Renderer) *auth {
	return &auth{models: models, render: render, yearSlug: yearSlug}
}

func UserFromRequest(r *http.Request) (*User, error) {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return nil, err
	}
	data, err := base64.StdEncoding.DecodeString(cookie.Value)
	if err != nil {
		return nil, err
	}
	var user User
	err = json.Unmarshal(data, &user)
	if err != nil {
		return nil, err
	}
	if len(user.ID) == 0 {
		err := errors.New("empty user cookie found")
		return nil, err
	}
	return &user, nil
}

// Authenticate renders the login page instead of next when there is no usable
// session.
//
// Any cookie problem counts as "not logged in", not as a server error. Earlier this
// returned 500 for anything other than a missing cookie, which meant an unusable
// cookie left a scanner staring at "server error" with no way out — and the login
// handler used to write exactly such a cookie for an unknown number. The cookie is
// cleared on the way past so a bad one cannot wedge the session.
func (a *auth) Authenticate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := UserFromRequest(r); err != nil {
			if !errors.Is(err, http.ErrNoCookie) {
				log.Printf("discarding unusable session cookie: %v", err)
				a.clearCookie(w)
			}
			a.render(w, r, PageData{Path: r.URL.Path})
			return
		}

		next.ServeHTTP(w, r)
	}
}

func (a *auth) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:    CookieName,
		Value:   "",
		Path:    "/",
		Expires: time.Unix(0, 0),
	})
}

func (a *auth) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	a.clearCookie(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// userByPhone resolves a phone number to exactly one scanner, or refuses.
//
// Both crew sources and the senior projection are consulted, always. Crew are personnel
// (gøgler, friend) **or** crew members (the 2026 crew pipeline) — either makes a scanner
// crew, so both are checked before deciding. The previous version returned on the first
// personnel hit and so could not notice a number registered as both crew and senior,
// silently giving a bandit the crew role.
//
// "Crew" and "senior" are still the only conflict. Being in both crew sources is no
// conflict at all — both mean crew — so the personnel identity is preferred when present
// and the crew-member id is the fallback.
func (a *auth) userByPhone(ctx context.Context, v string) (*User, error) {
	phone := types.PhoneNumber(v)
	if v == "" {
		return nil, ErrUnknownPhone
	}

	person, personErr := a.models.Personnel.GetByPhone(ctx, a.yearSlug, phone)
	senior, seniorErr := a.models.Senior.GetByPhone(ctx, a.yearSlug, phone)

	// The crew-member projection is optional wiring: guard the nil so a Models without it
	// (older callers, and tests that do not exercise crew members) still resolves.
	var crewMemberID types.UserID
	var crewMemberErr = tables.ErrRecordNotFound
	if a.models.CrewMember != nil {
		crewMemberID, crewMemberErr = a.models.CrewMember.GetByPhone(ctx, a.yearSlug, phone)
	}

	isPersonnel := personErr == nil && person != nil
	isCrewMember := crewMemberErr == nil && crewMemberID != ""
	crew := isPersonnel || isCrewMember
	bandit := seniorErr == nil && senior != nil

	switch {
	case crew && bandit:
		return nil, ErrAmbiguousRole

	case isPersonnel:
		return &User{
			ID:    person.ID,
			Phone: person.Phone,
			Type:  person.UserType,
			Role:  RoleCrew,
		}, nil

	case isCrewMember:
		return &User{
			ID:    crewMemberID,
			Phone: phone,
			Type:  types.TeamTypeCrew,
			Role:  RoleCrew,
		}, nil

	case bandit:
		// Presence in the senior projection is the whole bandit test: a senior signed
		// up to a klan is a bandit. There is no per-person flag to consult.
		return &User{
			ID:    types.UserID(senior.MemberID),
			Phone: senior.Phone,
			Type:  types.TeamTypeKlan,
			Role:  RoleBandit,
		}, nil

	default:
		return nil, ErrUnknownPhone
	}
}

// refusalMessage is the Danish text for a refused login, approved by HQ.
func refusalMessage(err error) string {
	switch {
	case errors.Is(err, ErrAmbiguousRole):
		return "Dit telefonnummer er registreret både som crew og som senior. " +
			"Ring til HQ, så de kan fjerne den ene registrering."
	case errors.Is(err, ErrUnknownPhone):
		return "Vi kender ikke det telefonnummer. Tjek at du har skrevet det rigtigt, " +
			"eller kontakt HQ."
	default:
		return "Der gik noget galt med login. Prøv igen, eller kontakt HQ."
	}
}

func (a *auth) LoginHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if r.FormValue("redir") != "" {
		path = r.FormValue("redir")
	}

	user, err := a.userByPhone(r.Context(), r.FormValue("phone"))
	if err != nil {
		// No cookie is written for a refused login. Writing one — which is what
		// marshalling a nil user used to do, storing the JSON literal `null` — made the
		// *next* request fail its cookie check and surface as "500 server error",
		// several steps away from the actual problem.
		if !errors.Is(err, ErrUnknownPhone) && !errors.Is(err, ErrAmbiguousRole) {
			log.Printf("login lookup failed: %v", err)
		}
		a.render(w, r, PageData{Path: path, Error: refusalMessage(err)})
		return
	}

	payload, err := json.Marshal(user)
	if err != nil {
		log.Printf("error encoding user %#v", err)
		a.render(w, r, PageData{Path: path, Error: refusalMessage(err)})
		return
	}
	cookie := http.Cookie{
		Name:   CookieName,
		Value:  base64.StdEncoding.EncodeToString(payload),
		Path:   "/",
		MaxAge: 3600 * 48,
		//HttpOnly: true,
		//Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}

	http.SetCookie(w, &cookie)
	http.Redirect(w, r, path, http.StatusSeeOther)
}

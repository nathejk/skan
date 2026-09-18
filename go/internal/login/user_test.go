package login

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/nathejk/shared-go/types"
	"nathejk.dk/internal/data"
	tables "nathejk.dk/nathejk/table"
	"nathejk.dk/nathejk/table/personnel"
	"nathejk.dk/nathejk/table/senior"
)

const testYear = "2026"

// stubPersonnel answers GetByPhone from a phone -> year map, so a test can express
// "this number is crew, but only in 2025".
type stubPersonnel struct{ byYear map[string]string } // phone -> year

func (s stubPersonnel) GetAll(context.Context, personnel.Filter) ([]*personnel.Person, error) {
	return nil, nil
}
func (s stubPersonnel) GetByID(context.Context, types.UserID) (*personnel.Person, error) {
	return nil, tables.ErrRecordNotFound
}
func (s stubPersonnel) GetByPhone(_ context.Context, year string, phone types.PhoneNumber) (*personnel.Person, error) {
	if s.byYear[string(phone)] != year {
		return nil, tables.ErrRecordNotFound
	}
	return &personnel.Person{ID: "crew-id", Phone: phone, UserType: types.TeamTypeBadut}, nil
}

type stubSenior struct{ byYear map[string]string }

func (s stubSenior) GetAll(context.Context, senior.Filter) ([]*senior.Senior, senior.Metadata, error) {
	return nil, senior.Metadata{}, nil
}
func (s stubSenior) GetByID(context.Context, types.MemberID) (*senior.Senior, error) {
	return nil, tables.ErrRecordNotFound
}
func (s stubSenior) GetByPhone(_ context.Context, year string, phone types.PhoneNumber) (*senior.Senior, error) {
	if s.byYear[string(phone)] != year {
		return nil, tables.ErrRecordNotFound
	}
	return &senior.Senior{MemberID: "senior-id", Phone: phone}, nil
}

// stubCrewMember answers GetByPhone from a phone -> year map, standing in for the 2026
// crew pipeline: a number that is crew but has no personnel (gøgler/friend) row.
type stubCrewMember struct{ byYear map[string]string }

func (s stubCrewMember) GetByPhone(_ context.Context, year string, phone types.PhoneNumber) (types.UserID, error) {
	if s.byYear[string(phone)] != year {
		return "", tables.ErrRecordNotFound
	}
	return "crewmember-id", nil
}

func newAuth(crew, crewMembers, seniors map[string]string) *auth {
	models := data.Models{
		Personnel:  stubPersonnel{byYear: crew},
		CrewMember: stubCrewMember{byYear: crewMembers},
		Senior:     stubSenior{byYear: seniors},
	}
	return New(models, testYear, func(http.ResponseWriter, *http.Request, PageData) {})
}

func TestUserByPhoneResolvesRole(t *testing.T) {
	tests := []struct {
		name        string
		crew        map[string]string
		crewMembers map[string]string
		seniors     map[string]string
		phone       string
		wantRole    Role
		wantErr     error
	}{
		{
			name:     "personnel only is crew",
			crew:     map[string]string{"11111111": testYear},
			phone:    "11111111",
			wantRole: RoleCrew,
		},
		{
			// The 2026 crew pipeline: crew with no gøgler/friend row must still be able to
			// log in and scan.
			name:        "crew member only is crew",
			crewMembers: map[string]string{"18181818": testYear},
			phone:       "18181818",
			wantRole:    RoleCrew,
		},
		{
			name:     "senior only is bandit",
			seniors:  map[string]string{"22222222": testYear},
			phone:    "22222222",
			wantRole: RoleBandit,
		},
		{
			// A data error upstream. Refusing beats guessing: one wrong guess hands a
			// player the crew view of the race.
			name:    "both in the same year is refused",
			crew:    map[string]string{"33333333": testYear},
			seniors: map[string]string{"33333333": testYear},
			phone:   "33333333",
			wantErr: ErrAmbiguousRole,
		},
		{
			// Crew via the crew pipeline and senior is the same conflict, and refused for
			// the same reason.
			name:        "crew member and senior is refused",
			crewMembers: map[string]string{"19191919": testYear},
			seniors:     map[string]string{"19191919": testYear},
			phone:       "19191919",
			wantErr:     ErrAmbiguousRole,
		},
		{
			// Personnel and crew member are both crew, so this is no conflict at all.
			name:        "personnel and crew member is crew",
			crew:        map[string]string{"20202020": testYear},
			crewMembers: map[string]string{"20202020": testYear},
			phone:       "20202020",
			wantRole:    RoleCrew,
		},
		{
			// The case that actually occurs: in the live data 12 numbers are crew in one
			// year and senior in another. Without the year filter these looked like
			// conflicts and would have locked real people out on race night.
			name:     "crew last year, senior this year is a bandit",
			crew:     map[string]string{"44444444": "2025"},
			seniors:  map[string]string{"44444444": testYear},
			phone:    "44444444",
			wantRole: RoleBandit,
		},
		{
			name:     "senior last year, crew this year is crew",
			crew:     map[string]string{"55555555": testYear},
			seniors:  map[string]string{"55555555": "2025"},
			phone:    "55555555",
			wantRole: RoleCrew,
		},
		{
			name:    "registered only in a previous year is unknown",
			crew:    map[string]string{"66666666": "2025"},
			phone:   "66666666",
			wantErr: ErrUnknownPhone,
		},
		{
			name:    "unknown number",
			phone:   "77777777",
			wantErr: ErrUnknownPhone,
		},
		{
			name:    "empty number",
			phone:   "",
			wantErr: ErrUnknownPhone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := newAuth(tt.crew, tt.crewMembers, tt.seniors).userByPhone(context.Background(), tt.phone)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("got error %v, want %v", err, tt.wantErr)
				}
				if user != nil {
					t.Fatalf("got user %+v, want none when refusing", user)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if user.Role != tt.wantRole {
				t.Fatalf("got role %q, want %q", user.Role, tt.wantRole)
			}
			if want := tt.wantRole == RoleBandit; user.IsBandit() != want {
				t.Fatalf("IsBandit() = %v, want %v", user.IsBandit(), want)
			}
		})
	}
}

// TestLoginHandlerWritesNoCookieWhenRefused guards the bug this replaced: a refused
// login used to store the JSON literal `null`, and the *next* request then failed its
// cookie check and surfaced as "500 server error" far from the cause.
func TestLoginHandlerWritesNoCookieWhenRefused(t *testing.T) {
	for _, phone := range []string{"00000000", ""} {
		var rendered PageData
		a := New(
			data.Models{
				Personnel: stubPersonnel{byYear: map[string]string{}},
				Senior:    stubSenior{byYear: map[string]string{}},
			},
			testYear,
			func(_ http.ResponseWriter, _ *http.Request, page PageData) { rendered = page },
		)

		req := httptest.NewRequest(http.MethodPost, "/login",
			strings.NewReader(url.Values{"phone": {phone}, "redir": {"/qr/7/123"}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		a.LoginHandler(w, req)

		if got := w.Result().Cookies(); len(got) != 0 {
			t.Fatalf("phone %q: got cookies %+v, want none", phone, got)
		}
		if rendered.Error == "" {
			t.Fatalf("phone %q: expected a Danish explanation on the login page", phone)
		}
		// The scanner must land where they were going once they do log in.
		if rendered.Path != "/qr/7/123" {
			t.Fatalf("phone %q: got redir path %q, want /qr/7/123", phone, rendered.Path)
		}
	}
}

func TestLoginHandlerStoresResolvedRole(t *testing.T) {
	a := newAuth(nil, nil, map[string]string{"22222222": testYear})

	req := httptest.NewRequest(http.MethodPost, "/login",
		strings.NewReader(url.Values{"phone": {"22222222"}, "redir": {"/"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	a.LoginHandler(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusSeeOther)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != CookieName {
		t.Fatalf("got cookies %+v, want one %q cookie", cookies, CookieName)
	}

	raw, err := base64.StdEncoding.DecodeString(cookies[0].Value)
	if err != nil {
		t.Fatalf("cookie is not base64: %v", err)
	}
	var user User
	if err := json.Unmarshal(raw, &user); err != nil {
		t.Fatalf("cookie is not a user: %v", err)
	}
	if user.Role != RoleBandit || !user.IsBandit() {
		t.Fatalf("got role %q, want %q", user.Role, RoleBandit)
	}
	if user.ID != "senior-id" {
		t.Fatalf("got id %q, want senior-id", user.ID)
	}
}

// TestAuthenticateRendersLoginForUnusableCookie covers the other half of the old 500:
// a cookie that cannot be decoded must send the scanner to the login page, not to an
// error page they cannot get out of.
func TestAuthenticateRendersLoginForUnusableCookie(t *testing.T) {
	for _, value := range []string{
		base64.StdEncoding.EncodeToString([]byte("null")), // what the old code wrote
		"not-base64-at-all",
		base64.StdEncoding.EncodeToString([]byte(`{"ID":""}`)),
	} {
		var rendered bool
		a := New(data.Models{}, testYear,
			func(http.ResponseWriter, *http.Request, PageData) { rendered = true })

		req := httptest.NewRequest(http.MethodGet, "/qr/7/123", nil)
		req.AddCookie(&http.Cookie{Name: CookieName, Value: value})
		w := httptest.NewRecorder()

		nextCalled := false
		a.Authenticate(func(http.ResponseWriter, *http.Request) { nextCalled = true })(w, req)

		if nextCalled {
			t.Fatalf("cookie %q: next handler ran without a valid session", value)
		}
		if !rendered {
			t.Fatalf("cookie %q: expected the login page to be rendered", value)
		}
		if w.Code >= 500 {
			t.Fatalf("cookie %q: got status %d, want no server error", value, w.Code)
		}
	}
}

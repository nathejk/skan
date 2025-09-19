package login

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/nathejk/shared-go/types"
	"nathejk.dk/internal/data"
)

var CookieName string = "user"

type User struct {
	ID    types.UserID
	Phone types.PhoneNumber
	Type  types.TeamType
}

type auth struct {
	models data.Models
}

func New(models data.Models) *auth {
	return &auth{models: models}
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
func (a *auth) Authenticate(next, login http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := UserFromRequest(r); err != nil {
			switch {
			case errors.Is(err, http.ErrNoCookie):
				login(w, r)
			default:
				log.Println(err)
				http.Error(w, "server error", http.StatusInternalServerError)
			}
			return
		}

		next.ServeHTTP(w, r)
	})
}
func (a *auth) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	c := &http.Cookie{
		Name:    CookieName,
		Value:   "",
		Path:    "/",
		Expires: time.Unix(0, 0),

		//HttpOnly: true,
	}

	http.SetCookie(w, c)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *auth) userByPhone(ctx context.Context, v string) *User {
	phone := types.PhoneNumber(v)
	//var user User
	if p, err := a.models.Personnel.GetByPhone(ctx, phone); err == nil {
		return &User{
			ID:    p.ID,
			Phone: p.Phone,
			Type:  p.UserType,
		}
	}
	if p, err := a.models.Senior.GetByPhone(ctx, phone); err == nil {
		return &User{
			ID:    types.UserID(p.MemberID),
			Phone: p.Phone,
			Type:  types.TeamTypeKlan,
		}
	}
	return nil
}
func (a *auth) LoginHandler(w http.ResponseWriter, r *http.Request) {
	user := a.userByPhone(r.Context(), r.FormValue("phone"))
	data, err := json.Marshal(user)
	if err != nil {
		log.Printf("error encoding user %#v", err)
	}
	cookie := http.Cookie{
		Name:   CookieName,
		Value:  base64.StdEncoding.EncodeToString(data),
		Path:   "/",
		MaxAge: 3600 * 48,
		//HttpOnly: true,
		//Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}

	path := r.URL.Path
	if r.FormValue("redir") != "" {
		path = r.FormValue("redir")
	}
	http.SetCookie(w, &cookie)
	http.Redirect(w, r, path, http.StatusSeeOther)
}

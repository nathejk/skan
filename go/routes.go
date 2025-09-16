package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/adler32"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

func (a *App) indexHandler(w http.ResponseWriter, r *http.Request) {
	ts, err := template.ParseFiles("templates/base.html", "templates/index.html")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if err := ts.ExecuteTemplate(w, "base", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *App) loginHandler(w http.ResponseWriter, r *http.Request) {
	ts, err := template.ParseFiles("templates/base.html", "templates/login.html")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if err := ts.ExecuteTemplate(w, "base", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *App) logoutHandler(w http.ResponseWriter, r *http.Request) {
	c := &http.Cookie{
		Name:    "exampleCookie",
		Value:   "",
		Path:    "/",
		Expires: time.Unix(0, 0),

		//HttpOnly: true,
	}

	http.SetCookie(w, c)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) doLoginHandler(w http.ResponseWriter, r *http.Request) {
	cookie := http.Cookie{
		Name:   "exampleCookie",
		Value:  "Hello world!",
		Path:   "/",
		MaxAge: 3600 * 48,
		//HttpOnly: true,
		//Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}

	http.SetCookie(w, &cookie)
	http.Redirect(w, r, r.URL.Path, http.StatusSeeOther)
}

func (a *App) scanHandler(w http.ResponseWriter, r *http.Request) {
	ts, err := template.ParseFiles("templates/base.html", "templates/coordinates.html")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"armNumber":  "10-4",
		"team":       "Vaskebjørne",
		"scanCount":  10,
		"catchCount": 1,
		"photo":      "/groupphoto.jpg",
		"remark":     "Patruljen har ondt i røven",
		"isBandit":   true,
	}
	if err := ts.ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *App) qrHandler(w http.ResponseWriter, r *http.Request) {
	max, _ := strconv.Atoi(r.URL.Query().Get("n"))
	w.Write([]byte("id,url\n"))
	for i := 1; i <= max; i++ {
		cs := adler32.Checksum([]byte(fmt.Sprintf("%d:%s", i, os.Getenv("SECRET"))))
		w.Write([]byte(fmt.Sprintf("%d,https://%s/qr/%d/%d\n", i, r.Host, i, cs)))
	}
}

func (a *App) aboutHandler(w http.ResponseWriter, r *http.Request) {
	ts, err := template.ParseFiles("templates/base.html", "templates/about.html")
	if err != nil {
		log.Print(err.Error())
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	err = ts.ExecuteTemplate(w, "base", nil)
	if err != nil {
		log.Print(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
func (a *App) routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/healthcheck", a.HealthcheckHandler)
	// Route for index page
	r.Get("/", a.indexHandler)

	// Route for about page
	r.Get("/about", a.aboutHandler)
	r.Get("/login", a.loginHandler)
	r.Get("/logout", a.logoutHandler)
	r.Get("/qr", a.qrHandler)
	r.Get("/qr/{id}/{cs}", a.authenticate(a.scanHandler))
	r.Post("/qr/{id}/{cs}", a.doLoginHandler)

	fileServer := http.FileServer(http.Dir("/webroot/"))

	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		http.StripPrefix("/", fileServer).ServeHTTP(w, r)
	})
	return r
}

func (a *App) authenticate(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := r.Cookie("exampleCookie")
		if err != nil {
			switch {
			case errors.Is(err, http.ErrNoCookie):
				//http.Error(w, "cookie not found", http.StatusBadRequest)
				a.loginHandler(w, r)
			default:
				log.Println(err)
				http.Error(w, "server error", http.StatusInternalServerError)
			}
			return
		}

		// Echo out the cookie value in the response body.
		//w.Write([]byte(cookie.Value))

		next.ServeHTTP(w, r)
	})
}

func (a *App) HealthcheckHandler(w http.ResponseWriter, r *http.Request) {
	env := Envelope{
		"status": "available",
		"system_info": map[string]string{
			"version": Version,
		},
	}
	err := a.writeJSON(w, http.StatusOK, env, nil)
	if err != nil {
		message := "the server encountered a problem and could not process your request"
		a.errorResponse(w, r, http.StatusInternalServerError, message)
	}
}

type Envelope map[string]any

func (a *App) writeJSON(w http.ResponseWriter, status int, data Envelope, headers http.Header) error {
	payload, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	for key, value := range headers {
		w.Header()[key] = value
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(payload)
	return nil
}

func (a *App) errorResponse(w http.ResponseWriter, r *http.Request, status int, message any) {
	env := Envelope{"error": message}
	err := a.writeJSON(w, status, env, nil)
	if err != nil {
		/*
			a.Logger.Error(err, map[string]string{
				"request_method": r.Method,
				"request_url":    r.URL.String(),
			})*/
		w.WriteHeader(500)
	}
}

package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/nathejk/shared-go/types"
	"nathejk.dk/internal/login"
)

//go:embed templates/*
var fs embed.FS

func (a *App) indexHandler(w http.ResponseWriter, r *http.Request) {
	ts, err := template.ParseFS(fs, "templates/base.html", "templates/index.html")
	if err != nil {
		http.Error(w, "Internal Server Error (index)", http.StatusInternalServerError)
		return
	}
	if err := ts.ExecuteTemplate(w, "base", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *App) doIndexHandler(w http.ResponseWriter, r *http.Request) {
	teamNumber, _ := strconv.Atoi(r.FormValue("number"))
	team, _ := a.models.Patrulje.GetByNumber(r.Context(), teamNumber)

	user, err := login.UserFromRequest(r)
	if user == nil {
		http.Error(w, fmt.Sprintf("No user %#v", err), http.StatusForbidden)
		return
	}
	if team != nil {
		if err := a.commands.QR.Scan("x", *team, *user, r.FormValue("latitude"), r.FormValue("longitude")); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}

	ts, err := template.ParseFS(fs, "templates/base.html", "templates/kvito.html")
	if err != nil {
		http.Error(w, "Internal Server Error (index)", http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"team":  team,
		"found": false,
	}
	if team != nil {
		data["found"] = true
		data["armNumber"] = fmt.Sprintf("%s-%d", team.TeamNumber, team.MemberCount)
	}
	if err := ts.ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *App) mapHandler(w http.ResponseWriter, r *http.Request) {
	ts, err := template.ParseFS(fs, "templates/base.html", "templates/map.html")
	if err != nil {
		http.Error(w, fmt.Sprintf("Internal Server Error (map) %#v", err), http.StatusInternalServerError)
		return
	}
	number, _ := strconv.Atoi(r.URL.Query().Get("number"))
	team, _ := a.models.Patrulje.GetByNumber(r.Context(), number)
	data := map[string]any{
		"qrid":     chi.URLParam(r, "id"),
		"checksum": chi.URLParam(r, "cs"),
		"confirm":  false,
		"team":     team,
		"photo":    "/groupphoto.jpg",
	}
	if team != nil {
		data["confirm"] = true
		data["armNumber"] = fmt.Sprintf("%s-%d", team.TeamNumber, team.MemberCount)
	}

	if err := ts.ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
func (a *App) doMapHandler(w http.ResponseWriter, r *http.Request) {
	user, _ := login.UserFromRequest(r)
	if user == nil {
		http.Error(w, "No user", http.StatusForbidden)
		return
	}
	qrID := types.QrID(chi.URLParam(r, "id"))
	cs, _ := strconv.Atoi(chi.URLParam(r, "cs"))
	if uint32(cs) != Checksum(qrID) {
		http.Error(w, "Malformed request", http.StatusExpectationFailed)
		return
	}
	teamNumber, _ := strconv.Atoi(r.FormValue("confirmed"))
	team, _ := a.models.Patrulje.GetByNumber(r.Context(), teamNumber)
	if team == nil {
		http.Error(w, "Patrulje not found", http.StatusNotFound)
		return
	}
	a.commands.QR.Register(qrID, *team, *user)
	http.Redirect(w, r, fmt.Sprintf("/qr/%s/%d", qrID, cs), http.StatusSeeOther)
}

func (a *App) loginHandler(w http.ResponseWriter, r *http.Request) {
	ts, err := template.ParseFS(fs, "templates/base.html", "templates/login.html")
	if err != nil {
		http.Error(w, "Internal Server Error (login)", http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"path": r.URL.Path,
	}
	if err := ts.ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *App) scanHandler(w http.ResponseWriter, r *http.Request) {
	user, _ := login.UserFromRequest(r)
	if user == nil {
		http.Error(w, "No user", http.StatusForbidden)
		return
	}
	qrID := types.QrID(chi.URLParam(r, "id"))
	cs, _ := strconv.Atoi(chi.URLParam(r, "cs"))
	if uint32(cs) != Checksum(qrID) {
		http.Error(w, "Malformed request", http.StatusExpectationFailed)
		return
	}
	qr, err := a.models.QR.GetByID(r.Context(), qrID)
	log.Printf("Scanned %s %#v %#v", qrID, qr, err)
	if err != nil {
		a.commands.QR.Found(qrID, *user)
		http.Redirect(w, r, fmt.Sprintf("/map/%s/%d", qrID, cs), http.StatusSeeOther)
		return
	}

	patrulje, err := a.models.Patrulje.GetByNumber(r.Context(), qr.TeamNumber)
	if err != nil {
		http.Error(w, fmt.Sprintf("No patrulje found %#v", err), http.StatusFailedDependency)
		return
	}

	ts, err := template.ParseFS(fs, "templates/base.html", "templates/coordinates.html")
	if err != nil {
		http.Error(w, "Internal Server Error (scan)", http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"qr":         qr,
		"armNumber":  fmt.Sprintf("%s-%d", patrulje.TeamNumber, patrulje.MemberCount),
		"team":       patrulje,
		"scanCount":  10,
		"catchCount": 1,
		"photo":      "/groupphoto.jpg",
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
		cs := Checksum(types.QrID(fmt.Sprintf("%d", i)))
		w.Write([]byte(fmt.Sprintf("%d,https://%s/qr/%d/%d\n", i, r.Host, i, cs)))
	}
}

func (a *App) aboutHandler(w http.ResponseWriter, r *http.Request) {
	ts, err := template.ParseFS(fs, "templates/base.html", "templates/about.html")
	if err != nil {
		log.Print(err.Error())
		http.Error(w, "Internal Server Error (about)", http.StatusInternalServerError)
		return
	}

	err = ts.ExecuteTemplate(w, "base", nil)
	if err != nil {
		log.Print(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
func (a *App) registerHandler(w http.ResponseWriter, r *http.Request) {
	type input struct {
		QrID       types.QrID `json:"qrId"`
		TeamNumber int        `json:"teamNumber"`
		Prompt     string     `json:"prompt"`
		Latitude   string     `json:"latitude"`
		Longitude  string     `json:"longitude"`
	}
	var in input
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	patrulje, err := a.models.Patrulje.GetByNumber(r.Context(), in.TeamNumber)
	if err != nil {
		http.Error(w, fmt.Sprintf("No patrulje found %#v", err), http.StatusFailedDependency)
		return
	}
	user, _ := login.UserFromRequest(r)
	if user == nil {
		http.Error(w, "No user", http.StatusForbidden)
		return
	}
	if err := a.commands.QR.Scan(in.QrID, *patrulje, *user, in.Latitude, in.Longitude); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(`{"status":"ok"}`))
}
func (a *App) routes() http.Handler {
	user := login.New(a.models)

	r := chi.NewRouter()
	r.Get("/healthcheck", a.HealthcheckHandler)
	// Route for index page
	r.Get("/", user.Authenticate(a.indexHandler, a.loginHandler))
	r.Post("/", a.doIndexHandler)

	// Route for about page
	r.Get("/about", a.aboutHandler)
	//r.Get("/login", a.loginHandler)
	r.Get("/logout", user.LogoutHandler)
	r.Post("/login", user.LoginHandler)
	r.Get("/qr", a.qrHandler)
	r.Get("/qr/{id}/{cs}", user.Authenticate(a.scanHandler, a.loginHandler))
	r.Post("/qr/{id}/{cs}", user.LoginHandler)
	r.Get("/map/{id}/{cs}", user.Authenticate(a.mapHandler, a.loginHandler))
	r.Post("/map/{id}/{cs}", user.Authenticate(a.doMapHandler, a.loginHandler))
	r.Put("/register", user.Authenticate(a.registerHandler, a.loginHandler))

	fileServer := http.FileServer(http.Dir("/webroot/"))

	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		http.StripPrefix("/", fileServer).ServeHTTP(w, r)
	})
	return r
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

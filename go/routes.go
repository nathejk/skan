package main

import (
	"crypto/subtle"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nathejk/shared-go/types"
	"nathejk.dk/internal/login"
	"nathejk.dk/nathejk/table/scan"
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

func (a *App) geoHandler(w http.ResponseWriter, r *http.Request) {
	type row struct {
		ID         int    `json:"id"`
		Lok        string `json:"lok"`
		TeamNumber string `json:"holdnummer"`
		TeamName   string `json:"patruljenavn"`
		Role       string `json:"role"`
		Timestamp  string `json:"tid"`
		Latitude   string `json:"lat"`
		Longitude  string `json:"lng"`
		Scanner    string `json:"scanner"`
	}
	scans, _ := a.models.Scan.GetAll(r.Context(), scan.Filter{})
	geo := []row{}
	for _, s := range scans {
		if (s.Latitude == "") || (s.Longitude == "") {
			continue
		}
		data := map[string]string{}
		patrulje, _ := a.models.Patrulje.GetByID(r.Context(), s.TeamID)
		senior, _ := a.models.Senior.GetByID(r.Context(), types.MemberID(s.ScannerID))
		if senior != nil {
			data["scanner"] = senior.Name
			data["role"] = "Bandit"
			klan, _ := a.models.Klan.GetByID(r.Context(), senior.TeamID)
			if klan != nil {
				data["lok"] = fmt.Sprintf("LOK %s", klan.Lok)
			}
		}
		person, _ := a.models.Personnel.GetByID(r.Context(), types.UserID(s.ScannerID))
		if person != nil {
			data["scanner"] = person.Name
			if v, ok := person.Additionals["department"].(string); ok {
				data["role"] = v
			}
		}
		qrID, _ := strconv.Atoi(string(s.QrID))
		ID, _ := strconv.Atoi(fmt.Sprintf("%d%05d", s.Uts, qrID))
		geo = append(geo, row{
			ID:         ID,
			TeamNumber: fmt.Sprintf("%d", s.TeamNumber),
			TeamName:   patrulje.Name,
			Timestamp:  time.Unix(s.Uts, 0).Format(time.RFC3339),
			Latitude:   s.Latitude,
			Longitude:  s.Longitude,
			Scanner:    data["scanner"],
			Lok:        data["lok"],
			Role:       data["role"],
		})
	}
	jsonstr, _ := json.Marshal(geo)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonstr)
}
func (a *App) doIndexHandler(w http.ResponseWriter, r *http.Request) {
	teamNumber, _ := strconv.Atoi(r.FormValue("number"))
	team, _ := a.models.Patrulje.GetByNumber(r.Context(), a.config.year, teamNumber)

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
	team, _ := a.models.Patrulje.GetByNumber(r.Context(), a.config.year, number)
	data := map[string]any{
		"qrid":     chi.URLParam(r, "id"),
		"checksum": chi.URLParam(r, "cs"),
		"confirm":  false,
		"team":     team,
		"photo":    "",
		"photoRef": "",
		"noPhoto":  false,
	}
	if team != nil {
		// The confirmation is only meaningful against the patrol's real photograph, so
		// the ref the scanner is shown is carried in the form and checked on POST.
		ref := a.coverPhotoRef(r.Context(), team.TeamID)
		data["armNumber"] = fmt.Sprintf("%s-%d", team.TeamNumber, team.MemberCount)
		data["photoRef"] = ref
		data["photo"] = a.coverPhotoThumbURL(r.Context(), team.TeamID)
		data["confirm"] = ref != ""
		data["noPhoto"] = ref == ""
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
	team, _ := a.models.Patrulje.GetByNumber(r.Context(), a.config.year, teamNumber)
	if team == nil {
		http.Error(w, "Patrulje not found", http.StatusNotFound)
		return
	}

	// Binding a code to a patrulje is the moment a mistake becomes permanent: every
	// later scan of that map is attributed to whoever is named here, for the rest of
	// the race. So the scanner must have confirmed the patrol against its photograph,
	// and that is checked here rather than trusted from the page.
	//
	// The check is on the ref, not a boolean: a confirmation is only worth anything
	// if it refers to the photograph actually shown. A stale or forged value fails.
	ref := a.coverPhotoRef(r.Context(), team.TeamID)
	if ref == "" {
		// A patrulje cannot start the race without being photographed, so this is an
		// error state rather than an unphotographed team. Refuse and send them to HQ.
		a.registrationRefused(w, r, "Denne patrulje har ikke noget billede, så identiteten kan ikke bekræftes. Kontakt HQ.")
		return
	}
	if r.FormValue("photoRef") != ref {
		a.registrationRefused(w, r, "Bekræftelsen passer ikke til patruljens billede. Prøv igen, og kontakt HQ hvis det bliver ved.")
		return
	}

	if err := a.commands.QR.Register(qrID, *team, *user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/qr/%s/%d", qrID, cs), http.StatusSeeOther)
}

// registrationRefused renders a Danish explanation instead of binding the code.
//
// Deliberately not http.Error: this is read by a scanner on a phone in a field, not
// by a developer, and "424 Failed Dependency" tells them nothing about what to do.
func (a *App) registrationRefused(w http.ResponseWriter, r *http.Request, message string) {
	ts, err := template.ParseFS(fs, "templates/base.html", "templates/refused.html")
	if err != nil {
		http.Error(w, message, http.StatusFailedDependency)
		return
	}
	w.WriteHeader(http.StatusFailedDependency)
	if err := ts.ExecuteTemplate(w, "base", map[string]any{"message": message}); err != nil {
		log.Print(err.Error())
	}
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
	qr, err := a.models.QR.GetByID(r.Context(), a.config.year, qrID)
	log.Printf("Scanned %s %#v %#v", qrID, qr, err)
	if err != nil {
		a.commands.QR.Found(qrID, *user)
		http.Redirect(w, r, fmt.Sprintf("/map/%s/%d", qrID, cs), http.StatusSeeOther)
		return
	}

	patrulje, err := a.models.Patrulje.GetByNumber(r.Context(), a.config.year, qr.TeamNumber)
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
		"photo":      a.coverPhotoThumbURL(r.Context(), patrulje.TeamID),
		"remark":     "",
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

	patrulje, err := a.models.Patrulje.GetByNumber(r.Context(), a.config.year, in.TeamNumber)
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

// requireExportToken guards the machine endpoints, /qr and /geo.
//
// These are not scanner pages: /qr feeds sticker printing and /geo feeds map/GIS
// export, so they sit behind a shared token in the query string rather than the
// phone login. Both are crew-grade information — /geo is a live map of the whole
// race, which in a bandit's hands would end the fair game outright — so the token
// must not be handed out to players.
//
// Three deliberate choices:
//
//   - The token is EXPORT_TOKEN, never SECRET. A token in a URL leaks into browser
//     history, proxy logs and Referer headers; leaking SECRET would let anyone
//     compute a valid checksum for every sticker id, printed or not, recoverable
//     only by reprinting the entire run.
//   - Comparison is constant-time, and an unset or empty configured token refuses
//     everything rather than failing open.
//   - The answer is 404, not 403, so the endpoints do not advertise their existence
//     to anyone poking at the host.
func (a *App) requireExportToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		supplied := r.URL.Query().Get("token")
		if a.config.exportToken == "" || supplied == "" ||
			subtle.ConstantTimeCompare([]byte(supplied), []byte(a.config.exportToken)) != 1 {
			http.NotFound(w, r)
			return
		}
		next(w, r)
	}
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
	r.Get("/qr", a.requireExportToken(a.qrHandler))
	r.Get("/geo", a.requireExportToken(a.geoHandler))
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

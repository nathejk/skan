package main

import (
	"context"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nathejk/shared-go/types"
	"nathejk.dk/internal/login"
	"nathejk.dk/nathejk/event"
	tables "nathejk.dk/nathejk/table"
	"nathejk.dk/nathejk/table/patrulje"
	"nathejk.dk/nathejk/table/qr"
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
		// Source is "gps", "manual", or "" for scans predating the distinction. Exported
		// because a hand-placed marker and a GPS fix are different qualities of fact, and
		// whoever draws the map should be able to tell which is which.
		Source string `json:"position"`
		// Accuracy is the radius of confidence in metres, or "" when unknown.
		Accuracy string `json:"accuracy"`
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
			Source:     s.LocationSource,
			Accuracy:   s.LocationAccuracy,
		})
	}
	jsonstr, _ := json.Marshal(geo)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonstr)
}

func (a *App) mapHandler(w http.ResponseWriter, r *http.Request) {
	ts, err := template.ParseFS(fs, "templates/base.html", "templates/map.html")
	if err != nil {
		http.Error(w, fmt.Sprintf("Internal Server Error (map) %#v", err), http.StatusInternalServerError)
		return
	}
	number, _ := strconv.Atoi(r.URL.Query().Get("number"))
	team, _ := a.models.Patrulje.GetByNumber(r.Context(), a.config.year, number)

	// The sheets a patrulje may be handed. Read even when no team is chosen yet, so a
	// year with no patrol maps drawn up says so on the first screen rather than after the
	// scanner has typed a number.
	spejderMaps, mapsErr := a.spejderSheets(r.Context())
	if mapsErr != nil {
		log.Printf("reading spejder map sheets: %v", mapsErr)
	}

	// Re-binding a code whose patrol has left the race. The sheet is the same physical
	// piece of paper — the scouts carried it to their new team — so it is carried over
	// rather than asked for again, and only the team number is in question.
	reassign := r.URL.Query().Get("reassign") != ""
	carriedMapID := ""
	if reassign {
		if existing, err := a.models.QR.GetByID(r.Context(), a.config.year, types.QrID(chi.URLParam(r, "id"))); err == nil {
			carriedMapID = existing.MapID
		}
	}

	// The sheet this scanner's post hands out, if they man one. Preselected rather than
	// applied silently: a wrong assumption would bind a patrol's code to a map they were
	// never given, and the scanner is the only one who can see that it is wrong.
	suggestedMapID := ""
	suggestedMapName := ""
	if user, err := login.UserFromRequest(r); err == nil && user != nil {
		sheet, found, err := a.models.Kort.SheetForScanner(r.Context(), a.config.year, string(user.ID), time.Now())
		if err != nil {
			log.Printf("reading the sheet for scanner %s: %v", user.ID, err)
		} else if found {
			suggestedMapID = sheet.ID
			suggestedMapName = sheet.Name
		}
	}

	data := map[string]any{
		"qrid":             chi.URLParam(r, "id"),
		"checksum":         chi.URLParam(r, "cs"),
		"confirm":          false,
		"team":             team,
		"photo":            "",
		"photoRef":         "",
		"noPhoto":          false,
		"discontinued":     false,
		"maps":             spejderMaps,
		"noMaps":           len(spejderMaps) == 0,
		"reassign":         reassign,
		"carriedMapId":     carriedMapID,
		"suggestedMapId":   suggestedMapID,
		"suggestedMapName": suggestedMapName,
	}
	if team != nil {
		// The confirmation is only meaningful against the patrol's real photograph, so
		// the ref the scanner is shown is carried in the form and checked on POST.
		ref := a.coverPhotoRef(r.Context(), team.TeamID)
		data["armNumber"] = fmt.Sprintf("%s-%d", team.TeamNumber, team.MemberCount)
		data["photoRef"] = ref
		data["photo"] = a.coverPhotoURL(r.Context(), team.TeamID)
		data["confirm"] = ref != "" && (len(spejderMaps) > 0 || carriedMapID != "")
		data["noPhoto"] = ref == ""

		// A patrulje that has left the race has no active members, so there is nobody in
		// front of the scanner to hand a map to. Offering the confirmation here would let
		// a slip of the finger record a sheet against a team that is out of the race —
		// and the likeliest slip is naming the team the scouts *left* rather than the one
		// they joined. Refuse, and say which number to use instead.
		//
		// Checked after the photograph so this takes precedence over it: "this team is
		// out" is the more useful answer, and it is true whether or not a photo exists.
		if team.Discontinued() {
			data["discontinued"] = true
			data["confirm"] = false
		}
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

	// Checked here as well as on the page, because the page cannot be trusted: a form
	// held open while the last member was moved off the team, or a hand-edited number,
	// would otherwise bind the sheet to a patrulje that has left the race.
	if team.Discontinued() {
		a.registrationRefused(w, r, "Patruljen er udgået af løbet, så den kan ikke få et kort. Er spejderne kommet med på et andet hold, skal du bruge holdnummeret på det hold.")
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

	// Which sheet is being handed over is part of the fact being recorded, so it is
	// required — and checked against the spejder set rather than trusted from the form,
	// which could otherwise name a crew sheet and show the scouts checkpoints they are
	// not meant to have yet.
	mapID := r.FormValue("mapId")
	if mapID == "" {
		a.registrationRefused(w, r, "Vælg hvilket kort patruljen får, før du tilknytter QR-koden.")
		return
	}
	// A carried-over sheet is accepted as it stands. On a re-bind the scanner is not
	// choosing a sheet — the scouts already hold it — so requiring it to still be in the
	// current spejder set would refuse a legitimate hand-over just because the sheet was
	// since retired from the set.
	carried := false
	if existing, err := a.models.QR.GetByID(r.Context(), a.config.year, qrID); err == nil {
		carried = existing.MapID != "" && existing.MapID == mapID
	}
	if !carried && !a.isSpejderSheet(r.Context(), mapID) {
		a.registrationRefused(w, r, "Det valgte kort hører ikke til spejdernes kortsæt. Prøv igen, og kontakt HQ hvis det bliver ved.")
		return
	}

	if err := a.commands.QR.Register(qrID, *team, *user, mapID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Wait for the projection to catch up before sending the scanner to a page that
	// reads it, so they are not bounced back here.
	if !a.waitForRegistration(r.Context(), qrID) {
		log.Printf("qr %s registered but not yet visible after %s", qrID, registrationVisibilityBudget)
	}
	http.Redirect(w, r, fmt.Sprintf("/qr/%s/%d", qrID, cs), http.StatusSeeOther)
}

// registrationVisibilityBudget bounds how long doMapHandler waits for the binding it
// just published to become readable.
//
// Short on purpose: it is a courtesy to the next page load, not a correctness
// mechanism. The registration is already durable in the stream before the wait starts.
const registrationVisibilityBudget = 2 * time.Second

// waitForRegistration blocks until a freshly published binding is visible in the read
// model, or the budget runs out.
//
// Why this exists: publishing an event and then redirecting to a page that *reads* the
// resulting projection is a read-after-write against an eventually consistent model. If
// the projection has not caught up, `scanHandler` sees an unknown code, publishes
// another `found`, and sends the scanner back to the registration page they just
// completed — which invites them to register the same code twice, and appends a
// spurious event to the log every time round.
//
// Polling rather than a stream signal: `stream/caughtup` announces "replay finished" at
// boot, not "this particular write has landed", so there is nothing to subscribe to for
// a single event. A bounded poll is the honest option, and it is cheap because the happy
// path returns on the first read.
//
// Returning false is not an error: the redirect happens either way. Waiting longer
// would be worse than a bounce.
func (a *App) waitForRegistration(ctx context.Context, qrID types.QrID) bool {
	deadline := time.Now().Add(registrationVisibilityBudget)
	for {
		if _, err := a.models.QR.GetByID(ctx, a.config.year, qrID); err == nil {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(25 * time.Millisecond):
		}
	}
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

// renderLogin draws the login page. Passed into the login package as its Renderer so
// that package stays free of template wiring.
func (a *App) renderLogin(w http.ResponseWriter, r *http.Request, page login.PageData) {
	ts, err := template.ParseFS(fs, "templates/base.html", "templates/login.html")
	if err != nil {
		http.Error(w, "Internal Server Error (login)", http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"path":  page.Path,
		"error": page.Error,
	}
	if err := ts.ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// scanResultData builds the template data for a scan result, per role.
//
// Extracted from the handler so the fair-game rule is testable, and stated once:
// **bandits may only learn about bandits.** A bandit is a player, so anything they
// see about a patrol's progress through the race is an unfair advantage. Crew are not
// players — checkpoint staff, guides, samaritter — and may see everything.
//
// The crew-only value is **omitted from the map entirely** for a bandit, rather than
// included and hidden by the template. The old page shipped both counts to every
// scanner inside a `style="display:hidden"` div; hiding data already delivered to a
// player's phone is not a boundary.
//
// The **member counts are shown to both roles**, and they are not race progress: the
// scanner has to check that the number of scouts in front of them matches what is
// registered, and a bandit needs that as much as crew does — more, since they are
// supposed to have caught the whole patrol.
//
// Counts are as of *before* this scan: the page renders first and `PUT /register`
// records the scan afterwards, so the template wording says "før" and treats zero
// catches as "first time".
func scanResultData(qrRow *qr.QR, team *patrulje.Patrulje, photoURL string, isBandit bool, catchCount, scanCount int) map[string]any {
	data := map[string]any{
		"qr":         qrRow,
		"armNumber":  fmt.Sprintf("%s-%d", team.TeamNumber, team.MemberCount),
		"team":       team,
		"photo":      photoURL,
		"isBandit":   isBandit,
		"catchCount": catchCount,

		// How many scouts the scanner should expect to see. This is the patrol's current
		// strength, not the number it started with: a patrol that drops below three cannot
		// continue alone, so its remaining members are reassigned to other teams — which
		// means a team can also be *larger* than it started.
		"expectedCount": team.ActiveMemberCount,
		"startCount":    team.MemberCount,

		// Whether to warn that the two differ. The arm marking the scouts wear encodes the
		// number they *started* with, so once strength changes the armband and reality
		// disagree — and a scanner counting heads against the armband would otherwise
		// think something is wrong, or worse, not notice that it is.
		"countChanged": team.ActiveMemberCount != team.MemberCount,
	}
	if !isBandit {
		data["scanCount"] = scanCount
	}
	return data
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

	// A discontinued patrol's map may be in someone else's hands: when a patrol leaves the
	// race its remaining scouts are reassigned to another team, and they bring their map.
	// So this code no longer reliably identifies who is standing here — ask, rather than
	// crediting a scan to a team that is no longer running.
	//
	// Asking rather than following the merge: the map may have gone to any of the teams the
	// scouts were split across, and only the person holding it can say.
	if patrulje.Discontinued() {
		http.Redirect(w, r, fmt.Sprintf("/map/%s/%d?reassign=1", qrID, cs), http.StatusSeeOther)
		return
	}

	ts, err := template.ParseFS(fs, "templates/base.html", "templates/coordinates.html")
	if err != nil {
		http.Error(w, "Internal Server Error (scan)", http.StatusInternalServerError)
		return
	}

	catchCount, err := a.models.Scan.CountCatchesByTeam(r.Context(), patrulje.TeamID)
	if err != nil {
		log.Printf("counting catches for %s: %v", patrulje.TeamID, err)
	}
	// Only read the crew-only count when the scanner is entitled to it.
	scanCount := 0
	if !user.IsBandit() {
		if scanCount, err = a.models.Scan.CountByTeam(r.Context(), patrulje.TeamID); err != nil {
			log.Printf("counting scans for %s: %v", patrulje.TeamID, err)
		}
	}

	data := scanResultData(
		qr, patrulje,
		a.coverPhotoURL(r.Context(), patrulje.TeamID),
		user.IsBandit(), catchCount, scanCount,
	)

	// Where to open the map if the browser refuses a position: the patrol's last known
	// place, so the scanner starts near where they are rather than panning across
	// Denmark in the dark. Empty when the patrol has never been scanned with a
	// position, and the template then opens on a wide view.
	//
	// **Crew only.** This is the patrol's position as some other scanner — very likely a
	// checkpoint — last recorded it, which is precisely the race progress a bandit may not
	// learn. It reached bandits until now, quietly, because it is only used to centre a
	// fallback map: a bandit who declined geolocation was handed a map already pointed at
	// wherever the patrol was last seen. Not queried at all for a bandit, per the rule that
	// a crew-only figure is not merely hidden.
	//
	// A bandit's own last reported position is remembered client-side instead (a cookie set
	// by coordinates.html), which is their own information and a better centre besides.
	if !user.IsBandit() {
		if latest, err := a.models.Scan.LatestByTeam(r.Context(), patrulje.TeamID); err == nil &&
			latest.Latitude != "" && latest.Longitude != "" {
			data["lastLatitude"] = latest.Latitude
			data["lastLongitude"] = latest.Longitude
		}
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

// rescanWindow is how recently the same scanner must have scanned the same patrulje
// for a repeat to look accidental.
//
// Thirty minutes is HQ's rule. Long enough that a fumbled double scan is caught, short
// enough that genuinely catching the same patrol twice in a night does not nag.
const rescanWindow = 30 * time.Minute

// needsRescanConfirmation reports whether a scan should be held for an explicit
// "yes, this really is a new scan" before being recorded.
//
// The rule, from HQ: look at the patrol's **single most recent** scan; if it was made
// by the same scanner less than 30 minutes ago, ask. Otherwise record straight away.
//
// Consequences, all intended:
//
//	same scanner, 5 min apart, nothing in between  -> ask
//	same scanner, 45 min apart                     -> record
//	same scanner twice, someone else in between    -> record
//	a different scanner immediately after          -> record
//
// A rescan *counts* — bandits do catch the same patrol more than once, so catchCount
// is a plain count of scans. This only guards against the same person recording one
// catch twice.
func needsRescanConfirmation(latest *scan.Scan, scannerID string, now time.Time) bool {
	if latest == nil {
		return false
	}
	if latest.ScannerID != scannerID {
		// Somebody else scanned in between, so this repeat is ordinary play.
		return false
	}
	elapsed := now.Sub(time.Unix(latest.Uts, 0))
	// A negative elapsed means clock skew between the app and the stream; treat it as
	// recent rather than as ancient, so skew cannot switch the guard off.
	return elapsed < rescanWindow
}

// metres sanitises a client-supplied accuracy into a plain number of metres, or "".
//
// The browser reports a float, sometimes with a long fractional tail. Rounding to whole
// metres keeps the column readable and discards precision the figure does not have, and
// parsing it at all keeps arbitrary strings out of the read model.
func metres(v string) string {
	if v == "" {
		return ""
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f < 0 {
		return ""
	}
	return strconv.FormatFloat(f, 'f', 0, 64)
}

func (a *App) registerHandler(w http.ResponseWriter, r *http.Request) {
	type input struct {
		QrID       types.QrID `json:"qrId"`
		TeamNumber int        `json:"teamNumber"`
		Latitude   string     `json:"latitude"`
		Longitude  string     `json:"longitude"`
		// Source is how the browser says the position was obtained, "gps" or "manual".
		// Normalised before use — it is a claim from the client, not a fact.
		Source string `json:"source"`
		// Accuracy is the radius of confidence in metres, as the geolocation API reported
		// it. Absent for a hand-placed marker.
		Accuracy string `json:"accuracy"`
		// Confirm is the scanner answering "yes, count this as a new scan" after being
		// asked. The page re-sends the same request with this set.
		Confirm bool `json:"confirm"`
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

	// The guard runs server-side, against the patrol's scan history — not in the
	// browser, and not on the QR code.
	if !in.Confirm {
		latest, err := a.models.Scan.LatestByTeam(r.Context(), patrulje.TeamID)
		if err != nil && !errors.Is(err, tables.ErrRecordNotFound) {
			// Never refuse a scan because the history could not be read. Recording a
			// possible duplicate is recoverable; losing a catch is not.
			log.Printf("reading latest scan for %s: %v", patrulje.TeamID, err)
		} else if needsRescanConfirmation(latest, string(user.ID), time.Now()) {
			// Not recorded yet — the page asks, then re-sends with confirm set. This is
			// deliberately not a silent drop: the scanner must be able to say yes and
			// have it count.
			a.writeJSON(w, http.StatusConflict, Envelope{
				"status":  "confirm",
				"message": "Du har lige scannet denne patrulje. Skal dette tælle som en ny scanning?",
			}, nil)
			return
		}
	}

	pos := event.Position{
		Latitude:  in.Latitude,
		Longitude: in.Longitude,
		Source:    event.NormalizeSource(in.Source),
		Accuracy:  metres(in.Accuracy),
	}
	if err := a.commands.QR.Scan(in.QrID, *patrulje, *user, pos); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	a.writeJSON(w, http.StatusCreated, Envelope{"status": "ok"}, nil)
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
	user := login.New(a.models, a.config.year, a.renderLogin)

	r := chi.NewRouter()
	r.Get("/healthcheck", a.HealthcheckHandler)
	// Route for the landing page. There is no team-number entry form: this service
	// records scans of real QR codes only, so the way in is the sticker on the map.
	// The page still sits behind Authenticate, because it is where a scanner logs in
	// before their first scan.
	r.Get("/", user.Authenticate(a.indexHandler))

	// Route for about page
	r.Get("/about", a.aboutHandler)
	//r.Get("/login", a.loginHandler)
	r.Get("/logout", user.LogoutHandler)
	r.Post("/login", user.LoginHandler)
	r.Get("/qr", a.requireExportToken(a.qrHandler))
	r.Get("/geo", a.requireExportToken(a.geoHandler))
	r.Get("/qr/{id}/{cs}", user.Authenticate(a.scanHandler))
	r.Post("/qr/{id}/{cs}", user.LoginHandler)
	r.Get("/map/{id}/{cs}", user.Authenticate(a.mapHandler))
	r.Post("/map/{id}/{cs}", user.Authenticate(a.doMapHandler))
	r.Put("/register", user.Authenticate(a.registerHandler))

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

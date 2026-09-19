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
	"sync"
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

	// One join, not a lookup per scan: this used to ask four extra questions about every
	// row, which is over ten thousand round trips on live data. The scanner's name, role
	// and lok arrive already resolved — see data.GeoReader.
	scans, err := a.models.Geo.Scans(r.Context())
	if err != nil {
		log.Printf("building the geo export: %v", err)
		http.Error(w, "Internal Server Error (geo)", http.StatusInternalServerError)
		return
	}

	geo := make([]row, 0, len(scans))
	for _, s := range scans {
		qrID, _ := strconv.Atoi(s.QrID)
		ID, _ := strconv.Atoi(fmt.Sprintf("%d%05d", s.Uts, qrID))
		geo = append(geo, row{
			ID:         ID,
			TeamNumber: fmt.Sprintf("%d", s.TeamNumber),
			TeamName:   s.TeamName,
			Timestamp:  time.Unix(s.Uts, 0).Format(time.RFC3339),
			Latitude:   s.Latitude,
			Longitude:  s.Longitude,
			Scanner:    s.Scanner,
			Lok:        s.Lok,
			Role:       s.Role,
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
	//
	// Kept only if it is actually among the sheets on offer. A post that hands out sketches
	// resolves to a sheet with no QR code, which the picker excludes — and naming an option
	// that is not in the list would leave the scanner hunting for it.
	suggestedMapID := ""
	suggestedMapName := ""
	if user, err := login.UserFromRequest(r); err == nil && user != nil {
		sheet, found, err := a.models.Kort.SheetForScanner(r.Context(), a.config.year, string(user.ID), time.Now())
		if err != nil {
			log.Printf("reading the sheet for scanner %s: %v", user.ID, err)
		} else if found && offeredSheet(spejderMaps, sheet.ID) {
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
		"maps":             []SheetOption{},
		"noMaps":           len(spejderMaps) == 0,
		"nextMapId":        "",
		"allMapsHandedOut": false,
		"reassign":         reassign,
		"carriedMapId":     carriedMapID,
		"suggestedMapId":   suggestedMapID,
		"suggestedMapName": suggestedMapName,
	}
	if team != nil {
		// HQ's note travels to this page too. Handing over a map is the other moment a
		// scanner stands in front of the patrol, and "fuld stop" reaching only the scan page
		// means a patrol under a stop order can be given their next map by someone who was
		// never told. It does **not** block the registration: the map in their hands still
		// has to be bound to them, and refusing would leave the code unattributed as well.
		addRemark(data, team)

		// The confirmation is only meaningful against the patrol's real photograph, so
		// the ref the scanner is shown is carried in the form and checked on POST.
		ref := a.coverPhotoRef(r.Context(), team.TeamID)
		data["armNumber"] = fmt.Sprintf("%s-%d", team.TeamNumber, team.MemberCount)
		data["photoRef"] = ref
		data["photo"] = a.coverPhotoURL(r.Context(), team.TeamID)
		data["confirm"] = ref != "" && (len(spejderMaps) > 0 || carriedMapID != "")
		data["noPhoto"] = ref == ""

		// Sheets are handed out in order, so which ones this patrulje may be given depends
		// on what they already hold. Only computable once a team is known, which is why the
		// picker is built here rather than beside the sheet read above.
		//
		// A read failure yields an empty held set, which is the cautious answer in the sense
		// that matters: it offers only the *first* sheet, so nothing later can be bound by
		// mistake. The scanner is not blocked outright either.
		held, err := a.models.QR.MapIDsByTeamNumber(r.Context(), a.config.year, number)
		if err != nil {
			log.Printf("reading registered sheets for team %d: %v", number, err)
			held = map[string]bool{}
		}
		options, next := sheetsInReach(spejderMaps, held)
		data["maps"] = options
		data["nextMapId"] = next
		data["allMapsHandedOut"] = next == "" && len(options) > 0

		// The sequence decides the default. A post's suggestion is only worth showing when
		// it agrees: the order is a hard rule, enforced on submit, so a post's sheet that is
		// not yet due cannot be preselected anyway — and two competing preselections on one
		// form is how a scanner ends up recording the sheet the page chose rather than the
		// one in their hand.
		if suggestedMapID != next {
			data["suggestedMapId"] = ""
			data["suggestedMapName"] = ""
		}

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

	// Sheets are handed out in order, and the disabled options on the page are not a
	// safeguard: a `disabled` attribute is a hint to a browser, and this form can be posted
	// without one. So the order is enforced here too.
	//
	// Skipped for a carried sheet, which is the same physical map moving to another team
	// rather than a new handover — the new team may well not hold the earlier sheets, and
	// refusing would strand a map the scouts are already carrying.
	if !carried {
		sheets, err := a.spejderSheets(r.Context())
		if err != nil {
			log.Printf("reading spejder map sheets: %v", err)
			a.registrationRefused(w, r, "Kortene kunne ikke læses lige nu. Prøv igen, og kontakt HQ hvis det bliver ved.")
			return
		}
		held, err := a.models.QR.MapIDsByTeamNumber(r.Context(), a.config.year, teamNumber)
		if err != nil {
			log.Printf("reading registered sheets for team %d: %v", teamNumber, err)
			a.registrationRefused(w, r, "Patruljens kort kunne ikke læses lige nu. Prøv igen, og kontakt HQ hvis det bliver ved.")
			return
		}
		if !sheetReachable(sheets, held, mapID) {
			// Name the sheet that is actually due: "wrong one" without "this one instead"
			// leaves a scanner guessing in the dark.
			_, next := sheetsInReach(sheets, held)
			message := "Patruljen får kortene i rækkefølge, og " + sheetName(sheets, mapID) + " er ikke næste kort."
			if next != "" {
				message += " Patruljen mangler " + sheetName(sheets, next) + " først."
			} else {
				message += " Patruljen har allerede fået alle kortene."
			}
			a.registrationRefused(w, r, message+" Kontakt HQ hvis det ikke passer.")
			return
		}
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

	// HQ's note about this patrol, shown to **both** roles. It is not race progress: it is
	// an instruction from HQ to whoever is standing in front of these scouts, and a bandit
	// needs "fuld stop" as much as crew does — more, since a bandit is the one likely to
	// send them running again.
	addRemark(data, team)
	if !isBandit {
		data["scanCount"] = scanCount
	}
	return data
}

// addRemark puts HQ's note about a patrol into template data, if there is one in force.
//
// Shared by the scan page and the registration page: both are moments where someone is
// standing in front of these scouts, which is exactly who the note is written for. A "fuld
// stop" that only appears on one of the two screens is a note the scanner can be handed a
// map without ever seeing.
//
// Almost every patrol has no note, so the keys are **absent** rather than empty and the
// templates render nothing at all in the ordinary case. An empty remark is off whatever the
// severity says, and `inactive` is a note HQ stood down — both are decided by
// RemarkInForce, not here.
func addRemark(data map[string]any, team *patrulje.Patrulje) {
	if team == nil || !team.RemarkInForce() {
		return
	}
	data["remark"] = team.Remark
	data["remarkStops"] = team.RemarkStops()
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

		// Whether this patrol is running to time, for the crew member manning the post.
		//
		// Crew-only, and not merely unrendered for a bandit: the verdict is derived from the
		// post's opening hours and from where the patrol was last seen on the route, which is
		// exactly the race progress a player may not learn. Reading it for a bandit would leak
		// it whether or not the template used it — the lesson of task 023.
		//
		// Inside the same crew branch as the position above rather than a branch of its own,
		// so "this is the route, and the route is crew information" is decided in one place.
		if t, ok := a.scanTimeliness(r.Context(), user, patrulje.TeamID, time.Now()); ok {
			addTimeliness(data, t)
		}
	}

	if err := ts.ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// scanTimeliness is the verdict for the scan page, or nothing.
//
// The two reads live here rather than in the handler so the handler keeps reading as a
// sequence of questions, and so the order of the guards is stated once:
//
//  1. A bandit gets nothing, and nothing is queried for them. A bandit is never rostered on a
//     post anyway — checkpersonnel holds personnel ids — so this guard is about the second
//     read, which is checkpoint activity and would be a leak on its own.
//  2. A scanner not manning a post gets nothing. That is most scanners, and it is also the
//     whole "is this postmandskab" test — see data.CheckpointReader.PostForScanner.
//  3. The previous checkpoint scan is only read when the post actually measures against it.
//     A fixed-hours post does not, and asking anyway would cost a query per scan for a value
//     nothing uses.
//
// A failed read gives no verdict, and says so in the log. Every other outcome here is a
// silence the scanner cannot tell apart from "this post has no hours", which is the one thing
// that must not go unrecorded.
func (a *App) scanTimeliness(ctx context.Context, user *login.User, teamID types.TeamID, now time.Time) (Timeliness, bool) {
	if user == nil || user.IsBandit() {
		return Timeliness{}, false
	}

	post, onPost, err := a.models.Checkpoint.PostForScanner(ctx, a.config.year, string(user.ID), now)
	if err != nil {
		log.Printf("reading the post for scanner %s: %v", user.ID, err)
		return Timeliness{}, false
	}
	if !onPost {
		return Timeliness{}, false
	}

	var previous time.Time
	var hasPrevious bool
	if !post.HasFixedHours() && post.HasRelativeHours() {
		if previous, hasPrevious, err = a.models.Checkpoint.PreviousCheckpointScan(ctx, a.config.year, string(teamID), post.ID); err != nil {
			log.Printf("reading the previous checkpoint scan of %s: %v", teamID, err)
			return Timeliness{}, false
		}
	}

	return postTimeliness(post, previous, hasPrevious, now)
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

// recentScans remembers each patrol's last scan as the app itself published it.
//
// # Why this exists
//
// The guard's real question is "what was this patrol's last scan", and the answer lived only
// in the `scan` projection — which is written asynchronously by a JetStream consumer. So the
// guard was reading a table that may not yet contain the scan published seconds earlier, and a
// projection that is behind makes the guard **silently absent**: no history found, no question
// asked, a duplicate recorded, nothing logged. HQ hit exactly that, scanning one code three
// times without ever being asked.
//
// This is the same hazard `waitForRegistration` was added for (task 015), from the other side:
// there the handler waits for the projection, here it cannot — nobody should stand in a field
// while a consumer catches up.
//
// The publisher already knows what it published, so that knowledge is kept here and used when
// it is *newer* than what the projection can see. The projection stays authoritative whenever
// it is up to date, which preserves the rule exactly: another scanner's scan in between still
// clears the question, because that scan reaches the projection and is newer than ours.
//
// In-process state is sound here because skan is a single instance, and losing it on restart is
// harmless: the projection is then the only source, which is where this started.
type recentScans struct {
	mu sync.Mutex
	by map[types.TeamID]scanMark
}

type scanMark struct {
	scannerID string
	uts       int64
}

func newRecentScans() *recentScans {
	return &recentScans{by: map[types.TeamID]scanMark{}}
}

// record notes that a scan of this patrulje was just published.
func (r *recentScans) record(teamID types.TeamID, scannerID string, at time.Time) {
	if r == nil || teamID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.by[teamID] = scanMark{scannerID: scannerID, uts: at.Unix()}
}

// latest returns what this process last published for a patrulje, if anything.
func (r *recentScans) latest(teamID types.TeamID) (scanMark, bool) {
	if r == nil || teamID == "" {
		return scanMark{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.by[teamID]
	return m, ok
}

// mostRecentScan picks whichever of the projection's answer and this process's own record is
// newer, so a lagging projection cannot hide a scan that was definitely made.
//
// Deliberately "newer wins" rather than "memory wins": if another scanner has scanned since,
// the projection knows about a scan we never published, and that scan is what the rule is
// about.
func mostRecentScan(projection *scan.Scan, own scanMark, haveOwn bool) *scan.Scan {
	if !haveOwn {
		return projection
	}
	if projection != nil && projection.Uts >= own.uts {
		return projection
	}
	return &scan.Scan{ScannerID: own.scannerID, Uts: own.uts}
}

// scanMarkString renders a scan for the guard's log line, including "none".
func scanMarkString(s *scan.Scan) string {
	if s == nil {
		return "none"
	}
	return fmt.Sprintf("%s@%d", s.ScannerID, s.Uts)
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
	//
	// The history comes from two places on purpose: the `scan` projection, and what this
	// process itself published. See recentScans — a projection that is a few seconds behind
	// would otherwise switch the guard off without a trace.
	if !in.Confirm {
		latest, err := a.models.Scan.LatestByTeam(r.Context(), patrulje.TeamID)
		if err != nil && !errors.Is(err, tables.ErrRecordNotFound) {
			// Never refuse a scan because the history could not be read. Recording a
			// possible duplicate is recoverable; losing a catch is not.
			log.Printf("reading latest scan for %s: %v", patrulje.TeamID, err)
			latest = nil
		}
		own, haveOwn := a.recentScans.latest(patrulje.TeamID)
		effective := mostRecentScan(latest, own, haveOwn)

		// One line per scan, because the failure mode here is invisible: when the guard does
		// not fire there is nothing to see afterwards, and "I was not asked" cannot be told
		// apart from "the rule said record" without knowing what it looked at.
		log.Printf("rescan guard: team=%s scanner=%s projection=%s own=%s(seen=%t) using=%s",
			patrulje.TeamID, user.ID,
			scanMarkString(latest), fmt.Sprintf("%s@%d", own.scannerID, own.uts), haveOwn,
			scanMarkString(effective))

		if needsRescanConfirmation(effective, string(user.ID), time.Now()) {
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
	// Remembered immediately, so the next scan is guarded whether or not the projection has
	// caught up by then.
	a.recentScans.record(patrulje.TeamID, string(user.ID), time.Now())

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

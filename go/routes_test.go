package main

import (
	"bytes"
	"context"
	"html/template"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/nathejk/shared-go/types"
	"nathejk.dk/internal/data"
	"nathejk.dk/nathejk/event"
	tables "nathejk.dk/nathejk/table"

	"nathejk.dk/nathejk/table/patrulje"
	"nathejk.dk/nathejk/table/photo"
	"nathejk.dk/nathejk/table/qr"
	"nathejk.dk/nathejk/table/scan"
)

// TestNeedsRescanConfirmation is HQ's rule, case by case.
//
// The check keys on the patrol's single most recent scan, so "another scanner in
// between" is expressed by that latest scan having a different scannerId.
func TestNeedsRescanConfirmation(t *testing.T) {
	now := time.Date(2026, 9, 12, 23, 30, 0, 0, time.UTC)
	ago := func(d time.Duration) int64 { return now.Add(-d).Unix() }

	tests := []struct {
		name   string
		latest *scan.Scan
		want   bool
	}{
		{
			name:   "never scanned",
			latest: nil,
			want:   false,
		},
		{
			name:   "same scanner 5 minutes ago, nothing in between",
			latest: &scan.Scan{ScannerID: "me", Uts: ago(5 * time.Minute)},
			want:   true,
		},
		{
			name:   "same scanner 45 minutes ago",
			latest: &scan.Scan{ScannerID: "me", Uts: ago(45 * time.Minute)},
			want:   false,
		},
		{
			// The boundary is "less than 30 minutes", so exactly 30 records.
			name:   "same scanner exactly 30 minutes ago",
			latest: &scan.Scan{ScannerID: "me", Uts: ago(30 * time.Minute)},
			want:   false,
		},
		{
			name:   "same scanner 29 minutes ago",
			latest: &scan.Scan{ScannerID: "me", Uts: ago(29 * time.Minute)},
			want:   true,
		},
		{
			// Another scanner scanned in between, so this repeat is ordinary play.
			name:   "someone else scanned in between",
			latest: &scan.Scan{ScannerID: "someone-else", Uts: ago(1 * time.Minute)},
			want:   false,
		},
		{
			// Clock skew must not switch the guard off.
			name:   "latest scan is in the future",
			latest: &scan.Scan{ScannerID: "me", Uts: now.Add(2 * time.Minute).Unix()},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := needsRescanConfirmation(tt.latest, "me", now); got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func testTeam() *patrulje.Patrulje {
	return &patrulje.Patrulje{TeamID: "team-1", TeamNumber: "42", MemberCount: 5, Name: "Ulvene"}
}

// TestScanResultDataOmitsCrewOnlyValuesForBandits is the fair-game rule as a test.
//
// The assertion is about *absence from the data*, not absence from the output. A
// bandit is a player, so the total scan count -- which reveals checkpoint progress --
// must never reach their browser at all. Hiding it in the markup, which the pre-port
// page did with a display:hidden div, is not a boundary.
func TestScanResultDataOmitsCrewOnlyValuesForBandits(t *testing.T) {
	data := scanResultData(&qr.QR{ID: "7"}, testTeam(), "http://foto/x", true, 3, 99)

	if _, present := data["scanCount"]; present {
		t.Fatalf("scanCount reached a bandit: %+v", data)
	}
	if got := data["catchCount"]; got != 3 {
		t.Fatalf("got catchCount %v, want 3 (bandits may know their own side's catches)", got)
	}
	if got := data["isBandit"]; got != true {
		t.Fatalf("got isBandit %v, want true", got)
	}
}

func TestScanResultDataGivesCrewEverything(t *testing.T) {
	data := scanResultData(&qr.QR{ID: "7"}, testTeam(), "http://foto/x", false, 3, 99)

	if got := data["scanCount"]; got != 99 {
		t.Fatalf("got scanCount %v, want 99", got)
	}
	if got := data["catchCount"]; got != 3 {
		t.Fatalf("got catchCount %v, want 3", got)
	}
}

// squashSpace collapses whitespace runs so assertions are about wording, not about
// where the template happens to wrap.
func squashSpace(s string) string { return strings.Join(strings.Fields(s), " ") }

// tagPattern matches HTML tags, so assertions can be written about the words a scanner
// reads rather than about the markup they are wrapped in — a number inside <strong> is
// still part of the same sentence.
var tagPattern = regexp.MustCompile(`<[^>]*>`)

// visibleText renders markup down to the text content, whitespace-normalised.
func visibleText(s string) string { return squashSpace(tagPattern.ReplaceAllString(s, " ")) }

// TestScanResultRendersWithoutCrewOnlyData renders the real template with a bandit's
// data. It would have caught the .remark bug: comparing a map key that no handler
// supplies is a template *execution* error, which truncates the page mid-response.
func TestScanResultRendersWithoutCrewOnlyData(t *testing.T) {
	for _, tc := range []struct {
		name       string
		isBandit   bool
		catchCount int
		wantText   string
		wantAbsent string
	}{
		{
			name: "bandit first catch", isBandit: true, catchCount: 0,
			wantText: "BINGO", wantAbsent: "99",
		},
		{
			name: "bandit later catch", isBandit: true, catchCount: 4,
			wantText: "fanget 4 gange før", wantAbsent: "99",
		},
		{
			// Danish singular: "1 gange" would be wrong.
			name: "bandit second catch uses the singular", isBandit: true, catchCount: 1,
			wantText: "fanget én gang før", wantAbsent: "99",
		},
		{
			name: "crew sees both counts", isBandit: false, catchCount: 4,
			wantText: "scannet 99 gange i alt",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts, err := template.ParseFS(fs, "templates/base.html", "templates/coordinates.html")
			if err != nil {
				t.Fatalf("parsing templates: %v", err)
			}

			var out bytes.Buffer
			data := scanResultData(&qr.QR{ID: "7"}, testTeam(), "http://foto/x", tc.isBandit, tc.catchCount, 99)
			if err := ts.ExecuteTemplate(&out, "base", data); err != nil {
				t.Fatalf("executing template: %v", err)
			}

			body := squashSpace(out.String())
			if !strings.Contains(body, tc.wantText) {
				t.Fatalf("output missing %q\n%s", tc.wantText, body)
			}
			if tc.wantAbsent != "" && strings.Contains(body, tc.wantAbsent) {
				t.Fatalf("output leaked crew-only value %q to a bandit\n%s", tc.wantAbsent, body)
			}
			if strings.Contains(body, "no value") {
				t.Fatalf("template referenced data no handler supplies\n%s", body)
			}
		})
	}
}

// stubQR reports "not found" a fixed number of times before the row appears, standing
// in for a projection that has not yet caught up.
type stubQR struct {
	notFoundFor int
	calls       int
}

func (s *stubQR) GetByID(context.Context, string, types.QrID) (*qr.QR, error) {
	s.calls++
	if s.calls <= s.notFoundFor {
		return nil, tables.ErrRecordNotFound
	}
	return &qr.QR{ID: "7"}, nil
}

// Not exercised here: waitForRegistration only reads one code by id.
func (s *stubQR) MapIDsByTeamNumber(context.Context, string, int) (map[string]bool, error) {
	return map[string]bool{}, nil
}

func newWaitApp(stub *stubQR) *App {
	a := &App{models: data.Models{QR: stub}}
	a.config.year = "2026"
	return a
}

func TestWaitForRegistration(t *testing.T) {
	t.Run("returns once the projection catches up", func(t *testing.T) {
		stub := &stubQR{notFoundFor: 2}
		if !newWaitApp(stub).waitForRegistration(context.Background(), "7") {
			t.Fatal("expected the binding to become visible")
		}
		if stub.calls != 3 {
			t.Fatalf("got %d reads, want 3 (two misses then a hit)", stub.calls)
		}
	})

	t.Run("the happy path does not sleep", func(t *testing.T) {
		stub := &stubQR{}
		start := time.Now()
		if !newWaitApp(stub).waitForRegistration(context.Background(), "7") {
			t.Fatal("expected the binding to be visible immediately")
		}
		if elapsed := time.Since(start); elapsed > 10*time.Millisecond {
			t.Fatalf("waited %s on a read that succeeded first time", elapsed)
		}
		if stub.calls != 1 {
			t.Fatalf("got %d reads, want 1", stub.calls)
		}
	})

	t.Run("gives up rather than hanging", func(t *testing.T) {
		// A projection that never catches up must not hold the request open: the
		// registration is already durable in the stream, so a bounce beats a hang.
		stub := &stubQR{notFoundFor: 1 << 30}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		done := make(chan bool, 1)
		go func() { done <- newWaitApp(stub).waitForRegistration(ctx, "7") }()

		select {
		case got := <-done:
			if got {
				t.Fatal("expected false when the projection never catches up")
			}
		case <-time.After(registrationVisibilityBudget + time.Second):
			t.Fatal("waitForRegistration did not return")
		}
	})
}

// TestMetres checks the accuracy value is sanitised rather than trusted.
//
// It arrives from the browser as a float with a long fractional tail, and the field is
// free-form, so anything could be posted. Whole metres is all the figure justifies.
func TestMetres(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"12.345678", "12"},
		{"12.7", "13"},
		{"0", "0"},
		{"1500", "1500"},
		{"", ""},
		{"abc", ""},           // junk becomes unknown, not stored
		{"-5", ""},            // a negative radius is meaningless
		{"'; DROP TABLE", ""}, // and cannot reach the statement builder as text
	} {
		if got := metres(tt.in); got != tt.want {
			t.Errorf("metres(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestNormalizeSource guards the rule that an unrecognised or absent source is
// "unknown" rather than being taken as a GPS fix.
func TestNormalizeSource(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"gps", event.LocationSourceGPS},
		{"manual", event.LocationSourceManual},
		{"", ""},
		{"GPS", ""},       // exact match only
		{"satellite", ""}, // an unknown claim is not silently promoted
	} {
		if got := event.NormalizeSource(tt.in); got != tt.want {
			t.Errorf("NormalizeSource(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestScanResultShowsExpectedHeadCount covers the count the scanner has to check against
// the scouts in front of them, and the warning when it no longer matches the armband.
//
// A patrol below three cannot continue alone, so its members are reassigned — which means
// a team can be larger than it started as well as smaller.
func TestScanResultShowsExpectedHeadCount(t *testing.T) {
	tests := []struct {
		name        string
		startCount  int
		activeCount int
		wantWarning bool
		wantText    string
	}{
		{
			name:       "unchanged strength does not warn",
			startCount: 5, activeCount: 5,
			wantWarning: false,
			wantText:    "Der skal være 5 spejdere",
		},
		{
			name:       "grown by reassignment warns",
			startCount: 4, activeCount: 7,
			wantWarning: true,
			wantText:    "kommet spejdere til fra et hold, der er udgået",
		},
		{
			name:       "shrunk warns",
			startCount: 6, activeCount: 4,
			wantWarning: true,
			wantText:    "Nogle spejdere er stoppet undervejs",
		},
		{
			name:       "a single remaining scout reads as singular",
			startCount: 5, activeCount: 1,
			wantWarning: true,
			wantText:    "Der skal være 1 spejder ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			team := testTeam()
			team.MemberCount = tt.startCount
			team.ActiveMemberCount = tt.activeCount

			data := scanResultData(&qr.QR{ID: "7"}, team, "", false, 0, 0)
			if got := data["expectedCount"]; got != tt.activeCount {
				t.Fatalf("expectedCount = %v, want %v (current strength, not start count)", got, tt.activeCount)
			}
			if got := data["countChanged"]; got != tt.wantWarning {
				t.Fatalf("countChanged = %v, want %v", got, tt.wantWarning)
			}

			ts, err := template.ParseFS(fs, "templates/base.html", "templates/coordinates.html")
			if err != nil {
				t.Fatalf("parsing templates: %v", err)
			}
			var out bytes.Buffer
			if err := ts.ExecuteTemplate(&out, "base", data); err != nil {
				t.Fatalf("executing template: %v", err)
			}
			body := visibleText(out.String())

			if !strings.Contains(body, tt.wantText) {
				t.Fatalf("output missing %q\n%s", tt.wantText, body)
			}
			if warned := strings.Contains(body, "Vær opmærksom"); warned != tt.wantWarning {
				t.Fatalf("warning shown = %v, want %v", warned, tt.wantWarning)
			}
		})
	}
}

// TestExpectedHeadCountIsShownToBandits: the head count is not race progress. A bandit is
// supposed to have caught the whole patrol, so they need it as much as crew do.
func TestExpectedHeadCountIsShownToBandits(t *testing.T) {
	team := testTeam()
	team.MemberCount = 4
	team.ActiveMemberCount = 7

	data := scanResultData(&qr.QR{ID: "7"}, team, "", true, 0, 99)

	if got := data["expectedCount"]; got != 7 {
		t.Fatalf("expectedCount = %v, want 7 for a bandit too", got)
	}
	if got := data["countChanged"]; got != true {
		t.Fatalf("countChanged = %v, want true for a bandit too", got)
	}
	// Still no crew-only figure.
	if _, present := data["scanCount"]; present {
		t.Fatalf("scanCount reached a bandit: %+v", data)
	}
}

// mapPageData is the shape mapHandler passes to templates/map.html, with only the keys
// the number-entry states read.
func mapPageData(reassign bool) map[string]any {
	return map[string]any{
		"qrid": "7", "checksum": "123",
		"confirm": false, "team": nil,
		"photo": "", "photoRef": "", "noPhoto": false, "discontinued": false,
		"maps":      []SheetOption{{ID: "kort-1", Name: "Deltagerkort 1", Reachable: true}},
		"noMaps":    false,
		"nextMapId": "kort-1", "allMapsHandedOut": false,
		"reassign": reassign, "carriedMapId": "",
		"suggestedMapId": "", "suggestedMapName": "",
	}
}

// TestMapPageDistinguishesUnusedFromDiscontinued guards a contradiction the page used to
// print: a code from a discontinued patrulje was introduced as "udgået af løbet" and then,
// two lines later, as one that "har ikke været scannet før".
//
// The two arrivals mean different things — never handed out, versus handed out to a team
// that has since left — so they must never share wording.
func TestMapPageDistinguishesUnusedFromDiscontinued(t *testing.T) {
	render := func(t *testing.T, reassign bool) string {
		t.Helper()
		ts, err := template.ParseFS(fs, "templates/base.html", "templates/map.html")
		if err != nil {
			t.Fatalf("parsing templates: %v", err)
		}
		var out bytes.Buffer
		if err := ts.ExecuteTemplate(&out, "base", mapPageData(reassign)); err != nil {
			t.Fatalf("executing template: %v", err)
		}
		return out.String()
	}

	t.Run("unused code", func(t *testing.T) {
		body := visibleText(render(t, false))

		if !strings.Contains(body, "ikke tilknyttet en patrulje endnu") {
			t.Fatalf("missing the unused-code description\n%s", body)
		}
		if strings.Contains(body, "udgået") {
			t.Fatalf("an unused code must not mention a discontinued patrol\n%s", body)
		}
		// It has just been scanned, so claiming otherwise is simply untrue.
		if strings.Contains(body, "har ikke været scannet før") {
			t.Fatalf("scanning it is what brought the scanner here\n%s", body)
		}
	})

	t.Run("code from a discontinued patrulje", func(t *testing.T) {
		raw := render(t, true)
		body := visibleText(raw)

		if !strings.Contains(body, "udgået af løbet") {
			t.Fatalf("missing the discontinued description\n%s", body)
		}
		if strings.Contains(body, "ikke tilknyttet en patrulje endnu") {
			t.Fatalf("a used code must not be described as unused\n%s", body)
		}
		if !strings.Contains(body, "har kortet nu") {
			t.Fatalf("should ask who holds the map now\n%s", body)
		}
		// Losing this on the way back turns a hand-over into a first-time registration,
		// and drops the sheet the scouts already carry.
		if !strings.Contains(raw, `name="reassign" value="1"`) {
			t.Fatalf("the number form must preserve reassign\n%s", raw)
		}
	})
}

// TestMapPageRefusesADiscontinuedPatrol: a patrol that has left the race has no active
// members, so there is nobody to hand a map to. The page must say so instead of offering
// the confirmation — and must not be mistaken for the *other* "udgået" sentence on it,
// which is about the team that previously held the code.
func TestMapPageRefusesADiscontinuedPatrol(t *testing.T) {
	render := func(t *testing.T, reassign bool) string {
		t.Helper()
		team := testTeam()
		team.SignupStatus = types.SignupStatusStarted
		team.ActiveMemberCount = 0
		if !team.Discontinued() {
			t.Fatal("the fixture is meant to be discontinued")
		}

		data := mapPageData(reassign)
		data["team"] = team
		data["armNumber"] = "42-5"
		data["photoRef"] = "ref"
		data["photo"] = "https://foto/photos/ref"
		// What mapHandler does once it sees Discontinued(): the photograph and the sheets
		// are there, and the confirmation is withdrawn anyway.
		data["discontinued"] = true
		data["confirm"] = false

		ts, err := template.ParseFS(fs, "templates/base.html", "templates/map.html")
		if err != nil {
			t.Fatalf("parsing templates: %v", err)
		}
		var out bytes.Buffer
		if err := ts.ExecuteTemplate(&out, "base", data); err != nil {
			t.Fatalf("executing template: %v", err)
		}
		return out.String()
	}

	for _, reassign := range []bool{false, true} {
		name := "unused code"
		if reassign {
			name = "reassign"
		}
		t.Run(name, func(t *testing.T) {
			raw := render(t, reassign)
			body := visibleText(raw)

			if !strings.Contains(body, "Patruljen er udgået") {
				t.Fatalf("missing the refusal\n%s", body)
			}
			// Naming the team is what lets the scanner see they typed the wrong number.
			if !strings.Contains(body, "Ulvene") {
				t.Fatalf("the refusal should name the patrol\n%s", body)
			}
			// Saying "no" without saying what to do instead strands the scanner.
			if !strings.Contains(body, "holdnummeret") {
				t.Fatalf("the refusal should say which number to use instead\n%s", body)
			}
			if strings.Contains(body, "tilknyt kortet") || strings.Contains(body, "flyt kortet") {
				t.Fatalf("a discontinued patrol must not be confirmable\n%s", body)
			}
			// This is about the team that was typed, not the code's previous holder.
			if strings.Contains(body, "Patruljen der havde dette kort") {
				t.Fatalf("the two udgået sentences have blurred\n%s", body)
			}
			if reassign && !strings.Contains(raw, `href="?reassign=1"`) {
				t.Fatalf("trying another number must keep reassign\n%s", raw)
			}
		})
	}
}

// TestMostRecentScanSurvivesALaggingProjection covers the hole that let three scans of one
// code be recorded without a single question.
//
// The guard read only the `scan` projection, which a JetStream consumer writes asynchronously.
// While that read does not yet include the scan published seconds ago, the guard finds no
// history, asks nothing, records a duplicate, and logs nothing. So the app also remembers what
// it published itself, and the *newer* of the two answers wins.
func TestMostRecentScanSurvivesALaggingProjection(t *testing.T) {
	now := time.Date(2026, 9, 12, 23, 30, 0, 0, time.UTC)
	ago := func(d time.Duration) int64 { return now.Add(-d).Unix() }

	t.Run("the projection has not caught up with our own scan", func(t *testing.T) {
		// What HQ hit: nothing visible in the projection yet.
		got := mostRecentScan(nil, scanMark{scannerID: "me", uts: ago(20 * time.Second)}, true)
		if !needsRescanConfirmation(got, "me", now) {
			t.Fatal("a scan this process published 20s ago must still be guarded")
		}
	})

	t.Run("the projection is behind by one scan", func(t *testing.T) {
		stale := &scan.Scan{ScannerID: "me", Uts: ago(40 * time.Minute)}
		got := mostRecentScan(stale, scanMark{scannerID: "me", uts: ago(1 * time.Minute)}, true)
		if !needsRescanConfirmation(got, "me", now) {
			t.Fatal("the newer of the two answers must win, or the guard reads ancient history")
		}
	})

	t.Run("another scanner since is still what the rule is about", func(t *testing.T) {
		// The projection knows about a scan this process never published. Newer wins, so the
		// question is correctly not asked: somebody else scanned in between.
		someoneElse := &scan.Scan{ScannerID: "you", Uts: ago(10 * time.Second)}
		got := mostRecentScan(someoneElse, scanMark{scannerID: "me", uts: ago(2 * time.Minute)}, true)
		if needsRescanConfirmation(got, "me", now) {
			t.Fatal("another scanner in between clears the question — the rule must not change")
		}
	})

	t.Run("no memory of our own falls back to the projection", func(t *testing.T) {
		p := &scan.Scan{ScannerID: "me", Uts: ago(1 * time.Minute)}
		if got := mostRecentScan(p, scanMark{}, false); got != p {
			t.Fatal("without a local record the projection is the only answer")
		}
		if mostRecentScan(nil, scanMark{}, false) != nil {
			t.Fatal("a patrol never scanned has no history at all")
		}
	})

	t.Run("recording then reading round-trips", func(t *testing.T) {
		r := newRecentScans()
		if _, ok := r.latest("team-1"); ok {
			t.Fatal("nothing recorded yet")
		}
		r.record("team-1", "me", now)
		m, ok := r.latest("team-1")
		if !ok || m.scannerID != "me" || m.uts != now.Unix() {
			t.Fatalf("got %+v, ok=%v", m, ok)
		}
		// A nil receiver is safe: tests and any handler built by hand may not set it.
		var nilled *recentScans
		nilled.record("team-1", "me", now)
		if _, ok := nilled.latest("team-1"); ok {
			t.Fatal("a nil recentScans must report nothing rather than panic")
		}
	})
}

// TestRescanQuestionIsAskedInThePage guards the fix for a scan that was counted without
// anyone being asked.
//
// The question used to be a native window.confirm(). Browsers may suppress those, and a
// suppressed dialog still returns a value — so the answer was decided by the browser's dialog
// policy rather than by the scanner: silently confirmed (a duplicate catch) or silently
// declined (a lost catch). Both are wrong, and both are invisible. The question must therefore
// be markup.
func TestRescanQuestionIsAskedInThePage(t *testing.T) {
	data := scanResultData(&qr.QR{ID: "7"}, testTeam(), "", false, 1, 2)
	data["lastLatitude"] = ""
	data["lastLongitude"] = ""

	ts, err := template.ParseFS(fs, "templates/base.html", "templates/coordinates.html")
	if err != nil {
		t.Fatalf("parsing templates: %v", err)
	}
	var out bytes.Buffer
	if err := ts.ExecuteTemplate(&out, "base", data); err != nil {
		t.Fatalf("executing template: %v", err)
	}
	raw := out.String()

	if strings.Contains(raw, "window.confirm") {
		t.Error("the rescan question must not be a native dialog: browsers may suppress it, " +
			"and the suppressed answer is the browser's rather than the scanner's")
	}
	for _, want := range []string{`id="askMessage"`, `id="confirmScan"`, `id="cancelScan"`} {
		if !strings.Contains(raw, want) {
			t.Errorf("missing %s: the question needs somewhere to be asked and answered", want)
		}
	}
	// Both answers must be reachable, and cancelling must not read like a failure.
	body := visibleText(raw)
	for _, want := range []string{
		"Ja, tæl det som en ny scanning",
		"Nej, det var et uheld",
		"Intet er registreret, før du svarer",
		"Fint — scanningen er ikke registreret",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q\n%s", want, body)
		}
	}
}

// TestSheetsInReachFollowTheHandoutOrder covers the sequential handout rule: a patrol that
// has not been given sheet 1 cannot be given sheet 2, and the sheet they are due is the
// default.
func TestSheetsInReachFollowTheHandoutOrder(t *testing.T) {
	// In handout order, as SpejderSheets returns them.
	sheets := []data.KortSheet{
		{ID: "k1", Name: "Deltagerkort 1"},
		{ID: "k2", Name: "Deltagerkort 2"},
		{ID: "k3", Name: "Deltagerkort 3"},
	}

	tests := []struct {
		name          string
		held          []string
		wantReachable []string
		wantNext      string
	}{
		{
			name:          "a patrol with nothing may only have the first sheet",
			held:          nil,
			wantReachable: []string{"k1"},
			wantNext:      "k1",
		},
		{
			name:          "holding the first opens the second, not the third",
			held:          []string{"k1"},
			wantReachable: []string{"k1", "k2"},
			wantNext:      "k2",
		},
		{
			// A lost or soaked map is replaced, carrying a new sticker for the same sheet.
			name:          "a held sheet stays available for a replacement",
			held:          []string{"k1", "k2"},
			wantReachable: []string{"k1", "k2", "k3"},
			wantNext:      "k3",
		},
		{
			// A gap must not be a dead end: the missing sheet is what they are due.
			name:          "a gap in the sequence is recoverable",
			held:          []string{"k1", "k3"},
			wantReachable: []string{"k1", "k2", "k3"},
			wantNext:      "k2",
		},
		{
			name:          "a patrol holding everything has no next sheet",
			held:          []string{"k1", "k2", "k3"},
			wantReachable: []string{"k1", "k2", "k3"},
			wantNext:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			held := map[string]bool{}
			for _, id := range tt.held {
				held[id] = true
			}

			options, next := sheetsInReach(sheets, held)

			if len(options) != len(sheets) {
				t.Fatalf("got %d options, want all %d listed", len(options), len(sheets))
			}
			// Every sheet is listed, in handout order, whether reachable or not.
			for i, o := range options {
				if o.ID != sheets[i].ID {
					t.Fatalf("option %d is %s, want %s: handout order lost", i, o.ID, sheets[i].ID)
				}
			}

			got := []string{}
			for _, o := range options {
				if o.Reachable {
					got = append(got, o.ID)
				}
			}
			if strings.Join(got, ",") != strings.Join(tt.wantReachable, ",") {
				t.Errorf("reachable = %v, want %v", got, tt.wantReachable)
			}
			if next != tt.wantNext {
				t.Errorf("next = %q, want %q", next, tt.wantNext)
			}

			// The server-side check must agree with the list, or the disabled options are
			// decoration: the form can be posted without them.
			for _, o := range options {
				if sheetReachable(sheets, held, o.ID) != o.Reachable {
					t.Errorf("sheetReachable(%s) disagrees with the picker", o.ID)
				}
			}
			if sheetReachable(sheets, held, "") {
				t.Error("no sheet chosen must not pass the order check")
			}
			if sheetReachable(sheets, held, "k-unknown") {
				t.Error("a sheet that is not in the set must not pass the order check")
			}
		})
	}
}

// TestHeldSheetsAreMarked: a scanner has to be able to see that a patrol already has a
// sheet, or the only clue that they picked a replacement is that nothing looks wrong.
func TestHeldSheetsAreMarked(t *testing.T) {
	options, next := sheetsInReach(
		[]data.KortSheet{{ID: "k1", Name: "Deltagerkort 1"}, {ID: "k2", Name: "Deltagerkort 2"}},
		map[string]bool{"k1": true},
	)

	data := mapPageData(false)
	data["maps"] = options
	data["nextMapId"] = next
	data["team"] = testTeam()
	data["armNumber"] = "42-5"
	data["photoRef"] = "ref"
	data["photo"] = "https://foto/photos/ref"
	data["confirm"] = true

	ts, err := template.ParseFS(fs, "templates/base.html", "templates/map.html")
	if err != nil {
		t.Fatalf("parsing templates: %v", err)
	}
	var out bytes.Buffer
	if err := ts.ExecuteTemplate(&out, "base", data); err != nil {
		t.Fatalf("executing template: %v", err)
	}
	raw := out.String()

	if !strings.Contains(raw, `<option value="k1">Deltagerkort 1 (udleveret)</option>`) {
		t.Errorf("a held sheet should be selectable and marked as handed out\n%s", raw)
	}
	if !strings.Contains(raw, `<option value="k2" selected>Deltagerkort 2</option>`) {
		t.Errorf("the sheet that is due should be preselected\n%s", raw)
	}
}

// TestUnreachableSheetsAreDisabled: they stay listed on purpose, so a scanner can see the
// sheet exists and is not due yet rather than suspect a broken list.
func TestUnreachableSheetsAreDisabled(t *testing.T) {
	options, next := sheetsInReach(
		[]data.KortSheet{{ID: "k1", Name: "Deltagerkort 1"}, {ID: "k2", Name: "Deltagerkort 2"}},
		nil,
	)

	data := mapPageData(false)
	data["maps"] = options
	data["nextMapId"] = next
	data["team"] = testTeam()
	data["armNumber"] = "42-5"
	data["photoRef"] = "ref"
	data["photo"] = "https://foto/photos/ref"
	data["confirm"] = true

	ts, err := template.ParseFS(fs, "templates/base.html", "templates/map.html")
	if err != nil {
		t.Fatalf("parsing templates: %v", err)
	}
	var out bytes.Buffer
	if err := ts.ExecuteTemplate(&out, "base", data); err != nil {
		t.Fatalf("executing template: %v", err)
	}
	raw := out.String()

	if !strings.Contains(raw, `<option value="k1" selected>Deltagerkort 1</option>`) {
		t.Errorf("the first sheet should be preselected\n%s", raw)
	}
	if !strings.Contains(raw, `<option value="k2" disabled>Deltagerkort 2 (ikke nået endnu)</option>`) {
		t.Errorf("a sheet out of reach should be listed but disabled\n%s", raw)
	}
}

// TestOfferedSheetKeepsASuggestionHonest: the picker lists only sheets that carry a QR code,
// while the handout-post suggestion can resolve to a sketch, which carries none. A suggestion
// that is not in the list must be dropped rather than named — pointing a scanner at an option
// that is not there is worse than saying nothing.
func TestOfferedSheetKeepsASuggestionHonest(t *testing.T) {
	sheets := []data.KortSheet{
		{ID: "kort-1", Name: "Deltagerkort 1"},
		{ID: "kort-2", Name: "Deltagerkort 2"},
	}

	tests := []struct {
		name string
		id   string
		want bool
	}{
		{"a sheet on offer", "kort-2", true},
		{"a sketch, which carries no QR code and is excluded", "kort-skitse", false},
		{"no suggestion at all", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := offeredSheet(sheets, tt.id); got != tt.want {
				t.Fatalf("offeredSheet(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}

	// An empty list cannot offer anything, not even an empty id.
	if offeredSheet(nil, "kort-1") {
		t.Fatal("nothing is on offer when there are no sheets")
	}
}

// TestIdentificationRefPicksAReadableRendition covers which rendition a scanner is shown.
//
// The photograph is the identity check, so it has to be big enough to recognise faces in —
// but past that point extra pixels only cost time on a field connection.
func TestIdentificationRefPicksAReadableRendition(t *testing.T) {
	display := photo.Photo{
		Ref: "display-2000",
		Renditions: []photo.PhotoRendition{
			{Name: "thumb256", Ref: "r256", Width: 256, Height: 192},
			{Name: "thumb1024", Ref: "r1024", Width: 1024, Height: 768},
			{Name: "thumb1600", Ref: "r1600", Width: 1600, Height: 1200},
		},
	}

	t.Run("smallest rendition that is still legible", func(t *testing.T) {
		if got := identificationRef(display); got != "r1024" {
			t.Fatalf("got %q, want r1024", got)
		}
	})

	t.Run("falls back to the display image when every rendition is too small", func(t *testing.T) {
		p := photo.Photo{
			Ref:        "display-2000",
			Renditions: []photo.PhotoRendition{{Name: "thumb256", Ref: "r256", Width: 256}},
		}
		if got := identificationRef(p); got != "display-2000" {
			t.Fatalf("got %q, want the display image", got)
		}
	})

	t.Run("falls back for a photograph predating renditions", func(t *testing.T) {
		if got := identificationRef(photo.Photo{Ref: "display-2000"}); got != "display-2000" {
			t.Fatalf("got %q, want the display image", got)
		}
	})

	t.Run("ignores a rendition with no ref", func(t *testing.T) {
		p := photo.Photo{
			Ref:        "display-2000",
			Renditions: []photo.PhotoRendition{{Name: "thumb1024", Ref: "", Width: 1024}},
		}
		if got := identificationRef(p); got != "display-2000" {
			t.Fatalf("got %q, want the display image", got)
		}
	})

	t.Run("never the original", func(t *testing.T) {
		// The projection does not expose the original at all — it carries the camera's
		// metadata, including where the picture was taken — so there is nothing to leak
		// here. This asserts the type stays that way.
		if got := identificationRef(display); got == "original" {
			t.Fatal("the original must never be served")
		}
	})
}

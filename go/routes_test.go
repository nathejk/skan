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
		"photo": "", "photoRef": "", "noPhoto": false,
		"maps": []data.KortSheet{{ID: "kort-1", Name: "Deltagerkort 1"}}, "noMaps": false,
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

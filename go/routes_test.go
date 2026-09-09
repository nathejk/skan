package main

import (
	"bytes"
	"context"
	"html/template"
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

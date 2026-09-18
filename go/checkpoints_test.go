package main

import (
	"bytes"
	"context"
	"html/template"
	"strings"
	"testing"
	"time"

	"nathejk.dk/internal/data"
	"nathejk.dk/internal/login"
	"nathejk.dk/nathejk/table/qr"
)

// TestPostTimeliness is the on-time rule, case by case.
//
// Both readings and every way the question can be unanswerable, because the unanswerable
// cases are the ones that would otherwise render a confident marker built from a zero.
func TestPostTimeliness(t *testing.T) {
	now := time.Date(2026, 9, 19, 23, 30, 0, 0, time.UTC)
	ago := func(d time.Duration) time.Time { return now.Add(-d) }

	fixed := func(from, until time.Time) data.Post {
		return data.Post{ID: "cp", Name: "Post 3A", OpenFrom: from, OpenUntil: until}
	}
	relative := func(d time.Duration) data.Post {
		return data.Post{ID: "cp", Name: "Post 3A", OpenDuration: d}
	}

	tests := []struct {
		name        string
		post        data.Post
		previous    time.Time
		hasPrevious bool
		wantOK      bool
		wantOnTime  bool
		wantMinutes int
		relative    bool
		wantAllowed int
	}{
		{
			name:        "fixed hours, still open",
			post:        fixed(ago(2*time.Hour), now.Add(42*time.Minute)),
			wantOK:      true,
			wantOnTime:  true,
			wantMinutes: 42,
		},
		{
			name:        "fixed hours, closed",
			post:        fixed(ago(3*time.Hour), ago(12*time.Minute)),
			wantOK:      true,
			wantOnTime:  false,
			wantMinutes: 12,
		},
		{
			// Early is not late. Nothing for the scanner to act on, and a red marker would
			// send them to HQ about a patrol that is doing well.
			name:        "fixed hours, arrived before opening",
			post:        fixed(now.Add(20*time.Minute), now.Add(3*time.Hour)),
			wantOK:      true,
			wantOnTime:  true,
			wantMinutes: 180,
		},
		{
			// The magnitude is reported and OnTime carries the sign, so a template never
			// has to render "-12 minutter".
			name:        "minutes are never negative",
			post:        fixed(ago(5*time.Hour), ago(90*time.Minute)),
			wantOK:      true,
			wantOnTime:  false,
			wantMinutes: 90,
		},
		{
			name:        "relative hours, within the allowance",
			post:        relative(30 * time.Minute),
			previous:    ago(24 * time.Minute),
			hasPrevious: true,
			wantOK:      true,
			wantOnTime:  true,
			wantMinutes: 24,
			relative:    true,
			wantAllowed: 30,
		},
		{
			name:        "relative hours, beyond the allowance",
			post:        relative(30 * time.Minute),
			previous:    ago(38 * time.Minute),
			hasPrevious: true,
			wantOK:      true,
			wantOnTime:  false,
			wantMinutes: 38,
			relative:    true,
			wantAllowed: 30,
		},
		{
			// Inclusive: a patrol arriving exactly on the limit has made it.
			name:        "relative hours, exactly on the limit",
			post:        relative(30 * time.Minute),
			previous:    ago(30 * time.Minute),
			hasPrevious: true,
			wantOK:      true,
			wantOnTime:  true,
			wantMinutes: 30,
			relative:    true,
			wantAllowed: 30,
		},
		{
			// The first post a patrol reaches has nothing before it. Measuring from the zero
			// time would report them tens of thousands of minutes late.
			name:   "relative hours, no previous checkpoint scan",
			post:   relative(30 * time.Minute),
			wantOK: false,
		},
		{
			name:        "relative hours, previous scan in the future",
			post:        relative(30 * time.Minute),
			previous:    now.Add(5 * time.Minute),
			hasPrevious: true,
			wantOK:      false,
		},
		{
			// Most posts in the current plan. No hours, no verdict.
			name:   "no hours configured",
			post:   data.Post{ID: "cp", Name: "Post 4B"},
			wantOK: false,
		},
		{
			// Real data: Post 1A closes before it opens. A wrong red marker for every patrol
			// all night is worse than none.
			name:   "fixed range that does not make sense",
			post:   fixed(now.Add(3*time.Hour), now.Add(1*time.Hour)),
			wantOK: false,
		},
		{
			// Both configured: the wall-clock deadline wins, because it is the one the rest
			// of the field is held to and the one whose consequence happens regardless.
			name:        "both kinds configured prefers the fixed deadline",
			post:        data.Post{ID: "cp", Name: "Mål", OpenFrom: ago(time.Hour), OpenUntil: now.Add(10 * time.Minute), OpenDuration: 5 * time.Minute},
			previous:    ago(4 * time.Hour),
			hasPrevious: true,
			wantOK:      true,
			wantOnTime:  true,
			wantMinutes: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := postTimeliness(tt.post, tt.previous, tt.hasPrevious, now)
			if ok != tt.wantOK {
				t.Fatalf("got ok=%v, want %v (%+v)", ok, tt.wantOK, got)
			}
			if !ok {
				return
			}
			if got.OnTime != tt.wantOnTime {
				t.Fatalf("got OnTime=%v, want %v", got.OnTime, tt.wantOnTime)
			}
			if got.Minutes != tt.wantMinutes {
				t.Fatalf("got Minutes=%d, want %d", got.Minutes, tt.wantMinutes)
			}
			if got.Relative != tt.relative {
				t.Fatalf("got Relative=%v, want %v", got.Relative, tt.relative)
			}
			if got.AllowedMinutes != tt.wantAllowed {
				t.Fatalf("got AllowedMinutes=%d, want %d", got.AllowedMinutes, tt.wantAllowed)
			}
			if got.PostName != tt.post.Name {
				t.Fatalf("got PostName=%q, want %q", got.PostName, tt.post.Name)
			}
		})
	}
}

// stubCheckpoint answers the two reads, and records that it was asked.
//
// The recording is the point of the bandit test below: the rule is that a crew-only figure
// must not be *queried* for a player, not merely that it must not be shown.
type stubCheckpoint struct {
	post     data.Post
	onPost   bool
	previous time.Time
	hasPrev  bool

	postCalls int
	prevCalls int
}

func (s *stubCheckpoint) PostForScanner(context.Context, string, string, time.Time) (data.Post, bool, error) {
	s.postCalls++
	return s.post, s.onPost, nil
}

func (s *stubCheckpoint) PreviousCheckpointScan(context.Context, string, string, string) (time.Time, bool, error) {
	s.prevCalls++
	return s.previous, s.hasPrev, nil
}

func newTimelinessApp(stub *stubCheckpoint) *App {
	a := &App{models: data.Models{Checkpoint: stub}}
	a.config.year = "2026"
	return a
}

// TestScanTimelinessIsCrewOnly is the fair-game rule for the verdict.
//
// A bandit is never rostered on a post, so this could look redundant — but the second read is
// checkpoint activity, and the whole lesson of task 023 is that a crew-only figure counts even
// when nothing renders it.
func TestScanTimelinessIsCrewOnly(t *testing.T) {
	now := time.Date(2026, 9, 19, 23, 30, 0, 0, time.UTC)
	stub := &stubCheckpoint{
		post:    data.Post{ID: "cp", Name: "Post 3A", OpenDuration: 30 * time.Minute},
		onPost:  true,
		hasPrev: true, previous: now.Add(-10 * time.Minute),
	}
	bandit := &login.User{ID: "b1", Role: login.RoleBandit}

	if _, ok := newTimelinessApp(stub).scanTimeliness(context.Background(), bandit, "team-1", now); ok {
		t.Fatal("a bandit was given a timeliness verdict")
	}
	if stub.postCalls != 0 || stub.prevCalls != 0 {
		t.Fatalf("checkpoint data was queried for a bandit: %d post reads, %d scan reads", stub.postCalls, stub.prevCalls)
	}
}

// TestScanTimelinessNeedsAPost covers the "is this scanner postmandskab" test: being rostered
// on a post is the qualification, and the previous-scan read must not happen without one.
func TestScanTimelinessNeedsAPost(t *testing.T) {
	now := time.Date(2026, 9, 19, 23, 30, 0, 0, time.UTC)
	stub := &stubCheckpoint{onPost: false}
	crew := &login.User{ID: "c1", Role: login.RoleCrew}

	if _, ok := newTimelinessApp(stub).scanTimeliness(context.Background(), crew, "team-1", now); ok {
		t.Fatal("a scanner who mans no post was given a verdict")
	}
	if stub.prevCalls != 0 {
		t.Fatalf("the previous checkpoint scan was read without a post: %d reads", stub.prevCalls)
	}
}

// TestScanTimelinessSkipsThePreviousScanForFixedHours: a wall-clock post does not measure
// against the previous one, so asking would cost a query per scan for a value nothing uses.
func TestScanTimelinessSkipsThePreviousScanForFixedHours(t *testing.T) {
	now := time.Date(2026, 9, 19, 23, 30, 0, 0, time.UTC)
	stub := &stubCheckpoint{
		post:   data.Post{ID: "cp", Name: "Mål", OpenFrom: now.Add(-time.Hour), OpenUntil: now.Add(20 * time.Minute)},
		onPost: true,
	}
	crew := &login.User{ID: "c1", Role: login.RoleCrew}

	got, ok := newTimelinessApp(stub).scanTimeliness(context.Background(), crew, "team-1", now)
	if !ok {
		t.Fatal("expected a verdict for a post with fixed hours")
	}
	if !got.OnTime || got.Minutes != 20 || got.Relative {
		t.Fatalf("got %+v, want on time, 20 minutes, wall-clock", got)
	}
	if stub.prevCalls != 0 {
		t.Fatalf("the previous checkpoint scan was read for a fixed-hours post: %d reads", stub.prevCalls)
	}
}

// TestTimelinessMarkerRendering checks the one thing a scanner actually reads.
//
// Assertions are on visible text with the tags stripped, because the wording is broken up by
// <strong> and a substring check against raw HTML passes or fails for the wrong reasons. The
// colour is asserted on the class, which is the only place it lives.
func TestTimelinessMarkerRendering(t *testing.T) {
	ts, err := template.ParseFS(fs, "templates/base.html", "templates/coordinates.html")
	if err != nil {
		t.Fatalf("parsing templates: %v", err)
	}

	render := func(t *testing.T, apply func(map[string]any)) string {
		t.Helper()
		data := scanResultData(&qr.QR{ID: "7"}, testTeam(), "", false, 1, 2)
		apply(data)
		var out bytes.Buffer
		if err := ts.ExecuteTemplate(&out, "base", data); err != nil {
			t.Fatalf("executing template: %v", err)
		}
		return out.String()
	}

	t.Run("green marker with minutes until close", func(t *testing.T) {
		html := render(t, func(d map[string]any) {
			addTimeliness(d, Timeliness{OnTime: true, PostName: "Post 3A", Minutes: 42})
		})
		if !strings.Contains(html, "panel-success") {
			t.Fatal("an on-time scan did not get the green marker")
		}
		if strings.Contains(html, "panel-danger") {
			t.Fatal("an on-time scan also got the red marker")
		}
		text := squashSpace(visibleText(html))
		for _, want := range []string{"TIL TIDEN", "Posten lukker om 42 min.", "Gælder Post 3A."} {
			if !strings.Contains(text, want) {
				t.Fatalf("missing %q in: %s", want, text)
			}
		}
	})

	t.Run("red marker with minutes past close", func(t *testing.T) {
		html := render(t, func(d map[string]any) {
			addTimeliness(d, Timeliness{OnTime: false, PostName: "Mål", Minutes: 12})
		})
		if !strings.Contains(html, "panel-danger") {
			t.Fatal("an overtime scan did not get the red marker")
		}
		text := squashSpace(visibleText(html))
		if !strings.Contains(text, "OVERTID") {
			t.Fatalf("missing the overtime heading in: %s", text)
		}
		if !strings.Contains(text, "Posten lukkede for 12 min. siden") {
			t.Fatalf("missing the minutes past close in: %s", text)
		}
	})

	t.Run("relative hours name the allowance", func(t *testing.T) {
		// The elapsed figure on its own cannot be checked: a scanner who disagrees with the
		// verdict needs to see what it was compared against.
		html := render(t, func(d map[string]any) {
			addTimeliness(d, Timeliness{OnTime: false, PostName: "Post 2B", Minutes: 38, Relative: true, AllowedMinutes: 30})
		})
		text := squashSpace(visibleText(html))
		if !strings.Contains(text, "38 min. siden sidste post") {
			t.Fatalf("missing the elapsed time in: %s", text)
		}
		if !strings.Contains(text, "der er afsat 30 min.") {
			t.Fatalf("missing the allowance in: %s", text)
		}
		if strings.Contains(text, "Posten lukker") {
			t.Fatalf("a relative post was described as closing at a time: %s", text)
		}
	})

	t.Run("no marker at all without a verdict", func(t *testing.T) {
		// The ordinary case: a scanner in the field, a bandit, or a post with no hours. A
		// marker here would be built from a zero, and nothing on the page would say so.
		html := render(t, func(map[string]any) {})
		if strings.Contains(html, "panel-success") || strings.Contains(html, "panel-danger") {
			t.Fatal("a marker was rendered without a verdict")
		}
		text := squashSpace(visibleText(html))
		if strings.Contains(text, "TIL TIDEN") || strings.Contains(text, "OVERTID") {
			t.Fatalf("a verdict was worded without one being given: %s", text)
		}
	})
}

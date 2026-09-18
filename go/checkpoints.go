package main

import (
	"time"

	"nathejk.dk/internal/data"
)

// Timeliness is the verdict a post's scanner gets about the patrulje in front of them.
//
// Two kinds of post mean two different things by "on time", and the difference is not
// cosmetic: one is a wall-clock deadline the whole field shares, the other a per-patrol
// allowance that started at the previous post. So the number and the sentence differ, while
// the marker — green or red — is the same, because that is the part a scanner reads at a
// glance in the dark.
type Timeliness struct {
	// OnTime is the marker. False means overtime, never "unknown": an unknown answer is
	// absence of a Timeliness altogether, so a template can never render a red marker for
	// a question nobody could answer.
	OnTime bool

	// PostName is the post the verdict is about, because the scanner may have been moved
	// and a verdict that does not say what it measured is not checkable.
	PostName string

	// Minutes is the figure to show, always non-negative — the sign lives in OnTime, and
	// "-12 minutter" is not a thing a scanner should have to interpret.
	//
	// Fixed hours: minutes until close, or minutes past close when overtime.
	// Relative hours: minutes since the last checkpoint.
	Minutes int

	// Relative distinguishes the two readings for the template's wording.
	Relative bool

	// AllowedMinutes is the relative allowance, for a sentence that can be checked
	// ("38 min siden sidste post, der er afsat 30"). Zero for a fixed-hours post.
	AllowedMinutes int
}

// postTimeliness turns a post, the patrol's previous checkpoint scan, and the current time
// into a verdict — or nothing.
//
// # Nothing is a real answer
//
// ok=false is returned whenever the question cannot be answered: a post hq has configured
// with neither kind of hours (which is most of them in the current plan), a fixed range that
// does not make sense, or a relative post the patrol has no previous checkpoint scan for.
// Every one of those would otherwise produce a confident marker derived from a zero, and a
// scanner has no way to tell a computed green from a defaulted one.
//
// # Fixed hours before relative
//
// A post with both configured is ambiguous, and the wall-clock deadline wins: it is the one
// the rest of the field is being held to, and the one whose consequence — the post closing —
// happens whatever this patrol's allowance says.
//
// # Before opening is on time
//
// A patrol arriving before the post opens is early, not late. There is nothing for a scanner
// to act on there, and a red marker would send them to HQ about a patrol that is doing well.
func postTimeliness(post data.Post, previous time.Time, hasPrevious bool, now time.Time) (Timeliness, bool) {
	switch {
	case post.HasFixedHours():
		remaining := post.OpenUntil.Sub(now)
		if remaining >= 0 {
			return Timeliness{
				OnTime:   true,
				PostName: post.Name,
				Minutes:  minutesOf(remaining),
			}, true
		}
		return Timeliness{
			OnTime:   false,
			PostName: post.Name,
			Minutes:  minutesOf(-remaining),
		}, true

	case post.HasRelativeHours():
		if !hasPrevious {
			return Timeliness{}, false
		}
		elapsed := now.Sub(previous)
		// A previous scan in the future, or one this handler has already counted, means the
		// clocks disagree with each other rather than that the patrol is early. Nothing
		// useful can be measured from it.
		if elapsed < 0 {
			return Timeliness{}, false
		}
		return Timeliness{
			// The allowance is inclusive: a patrol arriving exactly on the limit has made
			// it. Rounding decides this either way at the boundary, so it may as well
			// decide it in the patrol's favour.
			OnTime:         elapsed <= post.OpenDuration,
			PostName:       post.Name,
			Minutes:        minutesOf(elapsed),
			Relative:       true,
			AllowedMinutes: minutesOf(post.OpenDuration),
		}, true
	}

	return Timeliness{}, false
}

// minutesOf rounds a duration to whole minutes.
//
// Truncation would read badly at the deadline: 59 seconds past close is "0 minutter for
// sent", which looks like a rounding bug next to a red marker. Never negative — callers pass
// a magnitude.
func minutesOf(d time.Duration) int {
	if d < 0 {
		d = -d
	}
	return int(d.Round(time.Minute) / time.Minute)
}

// addTimeliness puts the verdict into template data, if there is one.
//
// One key holding the whole struct, not a field per value. The template then cannot render
// half a verdict, and "is there a verdict at all" is a single absent key — the same shape as
// the remark, and the reason `{{ if .timeliness }}` is enough: a missing map key is nil and
// falsy, while any struct value is truthy.
func addTimeliness(data map[string]any, t Timeliness) {
	data["timeliness"] = t
}

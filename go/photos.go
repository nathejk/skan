package main

import (
	"context"
	"fmt"

	"github.com/nathejk/shared-go/types"
	"nathejk.dk/nathejk/table/photo"
)

// minIdentificationWidth is the narrowest photograph worth showing a scanner.
//
// The photograph is the identity check: it is how a scanner refuses a wrong team number, so
// they have to be able to recognise faces in it. 1024px covers a phone screen at twice its
// logical width, which is enough for that; the 256px rendition the projection denormalises
// for lists is not, and would make the check guesswork.
const minIdentificationWidth = 1024

// coverPhotoURL returns the URL of the photograph that represents a patrulje, or "" when
// the team has none.
//
// The order is: the cover an organizer explicitly chose, otherwise the team's newest
// photograph. The fallback is the path that actually runs — nothing is known to publish
// `photocoverselected` yet, so most teams have no chosen cover and the newest photograph is
// what a scanner sees.
//
// An empty result means "this team has no photograph", which callers must treat as an error
// state rather than papering over: a patrulje cannot start the race without being
// photographed, so by the time anyone scans one of its codes a photograph exists. Absence
// means something upstream is wrong, and confirming a patrol's identity against a
// placeholder is worse than refusing.
//
// # Which rendition
//
// The smallest rendition at least minIdentificationWidth wide, falling back to the display
// image. On the live data that is `thumb1024` at 160 KB rather than the 2000px display image
// at 540 KB — a third of the bytes for a picture that is still perfectly recognisable on a
// phone.
//
// This deliberately replaces an earlier choice to serve `thumbRef`, the *smallest*
// rendition, to keep the page light on a weak field connection. That was the wrong trade:
// 256px is a list thumbnail, and a scanner squinting at one cannot confidently say "no, that
// is not this patrol" — which is the whole point of showing it.
//
// It is never the **original**, which the photo projection deliberately does not expose:
// that carries the camera's metadata, including where the picture was taken.
func (a *App) coverPhotoURL(ctx context.Context, teamID types.TeamID) string {
	p, ok := a.coverPhoto(ctx, teamID)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%s/photos/%s", a.config.fotoBaseURL, identificationRef(p))
}

// identificationRef picks the ref to show for recognising a patrol.
func identificationRef(p photo.Photo) string {
	best := ""
	bestWidth := 0
	for _, r := range p.Renditions {
		if r.Ref == "" || r.Width < minIdentificationWidth {
			continue
		}
		// Smallest qualifying rendition: past the point where faces are legible, extra
		// pixels only cost time on a bad connection.
		if best == "" || r.Width < bestWidth {
			best, bestWidth = r.Ref, r.Width
		}
	}
	if best != "" {
		return best
	}
	// No rendition is big enough — a photograph from before renditions existed, or one
	// scaled only for lists. The display image is then the honest choice: too many bytes
	// beats too few pixels when the picture is what a decision rests on.
	return p.Ref
}

// coverPhoto resolves which photograph represents a patrulje: the chosen cover, else the
// newest, else nothing.
func (a *App) coverPhoto(ctx context.Context, teamID types.TeamID) (photo.Photo, bool) {
	photos, err := a.models.Photo.ByTeam(ctx, a.config.year, string(teamID))
	if err != nil || len(photos) == 0 {
		return photo.Photo{}, false
	}

	chosen := photos[0] // ByTeam returns newest first.
	if ref := a.chosenRef(ctx, teamID); ref != "" {
		for _, p := range photos {
			if p.Ref == ref {
				chosen = p
				break
			}
		}
	}
	return chosen, true
}

// coverPhotoRef is the **display** ref of the representing photograph, or "".
//
// Deliberately the display ref and not whichever rendition is shown: this is the
// photograph's identity, and it is what the registration form carries and doMapHandler
// compares. Tying the confirmation to a rendition would break it the moment the rendition
// set changed.
func (a *App) coverPhotoRef(ctx context.Context, teamID types.TeamID) string {
	p, ok := a.coverPhoto(ctx, teamID)
	if !ok {
		return ""
	}
	return p.Ref
}

// chosenRef returns the explicitly selected cover ref, or "" if there is none.
//
// A read failure is treated as "no choice" rather than propagated: the fallback is
// always available and always correct enough, so a hiccup in this table must not
// deny a scanner the photograph they need to confirm a patrol.
func (a *App) chosenRef(ctx context.Context, teamID types.TeamID) string {
	if a.models.PhotoCover == nil {
		return ""
	}
	ref, err := a.models.PhotoCover.Ref(a.config.year, string(teamID))
	if err != nil {
		return ""
	}
	return ref
}

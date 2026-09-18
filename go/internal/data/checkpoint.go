package data

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Post is a checkpoint as the scanner manning it needs it: its name, and the two ways hq
// can express when a patrulje is due.
//
// Deliberately not the copied `checkpoint.Checkpoint`. That struct carries a position, a
// description, per-team on-time lists and the scanners on duty, none of which this page
// asks about — and its OpenDuration is a time.Duration scanned from a column of whole
// minutes, which is a trap worth leaving behind rather than importing.
type Post struct {
	ID           string
	Name         string
	CheckgroupID string

	// OpenFrom/OpenUntil are the post's wall-clock hours. Both zero means hq has not set
	// any: the column default is 0, not NULL, so "unset" and "midnight 1970" are the same
	// value and there is nothing better to distinguish them by.
	OpenFrom  time.Time
	OpenUntil time.Time

	// OpenDuration is how long a patrulje may take to get here from the previous post.
	// Stored in whole minutes (see checkpoint/consumer.go, which divides the event's
	// RelativeTimeDuration by time.Minute) and converted here, so callers never have to
	// remember the unit.
	OpenDuration time.Duration
}

// HasFixedHours reports whether this post closes at a wall-clock time.
//
// Requires the range to make sense, not merely to be present. Real data has `Post 1A` with
// openUntil *before* openFrom, and treating that as fixed hours would tell every scanner
// there that every patrol is hours overdue — a confident, wrong red marker, which is worse
// than no marker at all.
func (p Post) HasFixedHours() bool {
	return !p.OpenFrom.IsZero() && !p.OpenUntil.IsZero() && p.OpenUntil.After(p.OpenFrom)
}

// HasRelativeHours reports whether this post's allowance runs from the previous post.
func (p Post) HasRelativeHours() bool { return p.OpenDuration > 0 }

// CheckpointReader reads the checkpoint plan as a scanner's page needs it.
//
// Same reasoning as KortReader: `nathejk/table/checkpoint` and `checkpersonnel` are hq's
// packages, copied in here and kept close to their origin, and their queriers cannot answer
// the question this page has — `checkpersonnel.Filter` has no user field at all, so finding
// a scanner's own assignment through it would mean reading every assignment for the year and
// filtering in Go, on a page a scanner is waiting for.
type CheckpointReader struct {
	DB *sql.DB
}

// PostForScanner returns the checkpoint this scanner is manning right now.
//
// This is also the whole test for "is the scanner postmandskab": there is no such userType
// to consult — `personnel.userType` holds `gøgler` and `friend` — and being rostered on a
// post at this moment is both the more accurate question and the only one that identifies
// *which* post, without which there is no verdict to give.
//
// found=false is ordinary. Most scanners are not on a post: bandits never are, and a guide
// or samarit scanning in the field has no opening hours to be measured against.
//
// More than one match is also found=false. A crew member rostered on two posts at once is a
// planning error, and picking one of them would decide the verdict — a patrol can easily be
// on time for one post and late for another.
func (r CheckpointReader) PostForScanner(ctx context.Context, year, userID string, at time.Time) (Post, bool, error) {
	if year == "" || userID == "" {
		return Post{}, false, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// startUts/endUts of 0 mean "unbounded", which is how most assignments on the stream
	// look — nothing has published a removal either. The shift is honoured when it is set,
	// and the year filter is what stops last year's roster from putting a scanner on a post
	// tonight.
	const query = `SELECT c.id, c.name, c.checkgroupId, c.openFromUts, c.openUntilUts, c.openDuration
		FROM checkpersonnel cp
		JOIN checkpoint c ON c.id = cp.checkpointId AND c.year = cp.year
		WHERE cp.userId = ? AND cp.year = ?
		  AND (cp.startUts = 0 OR cp.startUts <= ?)
		  AND (cp.endUts = 0 OR cp.endUts >= ?)
		ORDER BY c.id ASC
		LIMIT 2`

	uts := at.Unix()
	rows, err := r.DB.QueryContext(ctx, query, userID, year, uts, uts)
	if err != nil {
		return Post{}, false, fmt.Errorf("reading the post for scanner %q: %w", userID, err)
	}
	defer func() { _ = rows.Close() }()

	// LIMIT 2 so "exactly one" can be told from "several" without reading the rest.
	posts := []Post{}
	for rows.Next() {
		var p Post
		var openFrom, openUntil, durationMinutes int64
		if err := rows.Scan(&p.ID, &p.Name, &p.CheckgroupID, &openFrom, &openUntil, &durationMinutes); err != nil {
			return Post{}, false, fmt.Errorf("scanning the post for scanner %q: %w", userID, err)
		}
		if openFrom > 0 {
			p.OpenFrom = time.Unix(openFrom, 0)
		}
		if openUntil > 0 {
			p.OpenUntil = time.Unix(openUntil, 0)
		}
		p.OpenDuration = time.Duration(durationMinutes) * time.Minute
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		return Post{}, false, fmt.Errorf("reading the post for scanner %q: %w", userID, err)
	}
	if len(posts) != 1 {
		return Post{}, false, nil
	}
	return posts[0], true, nil
}

// PreviousCheckpointScan is when a patrulje was last scanned at a checkpoint other than
// this one.
//
// The clock a relative post measures against. It is deliberately the previous *checkpoint*
// scan and not the previous scan of any kind: a bandit catching a patrol on the way here
// does not restart their allowance, and a guide or samarit scan says nothing about the
// route either.
//
// `excludePostID` keeps this post's own scans out. A patrol standing here has usually just
// been scanned by the colleague at the same post, or is being rescanned after a
// confirmation, and measuring from that would report every patrol as arriving in 0 minutes.
//
// A scan is "at a checkpoint" if its scanner was rostered on one at the time. That is the
// same definition PostForScanner uses, applied to the earlier scan, which is why the shift
// bounds are compared against the scan's own timestamp rather than now.
//
// found=false is ordinary early in the race: the first post a patrol reaches has nothing
// before it. Callers must show no verdict then, not a zero.
func (r CheckpointReader) PreviousCheckpointScan(ctx context.Context, year, teamID, excludePostID string) (time.Time, bool, error) {
	if year == "" || teamID == "" {
		return time.Time{}, false, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// EXISTS rather than a JOIN: a scanner can be rostered on several posts across the
	// night, and a join would return the same scan once per roster row.
	const query = `SELECT s.uts
		FROM scan s
		WHERE s.teamId = ?
		  AND EXISTS (
			SELECT 1 FROM checkpersonnel cp
			WHERE cp.userId = s.scannerId
			  AND cp.year = ?
			  AND cp.checkpointId <> ?
			  AND (cp.startUts = 0 OR cp.startUts <= s.uts)
			  AND (cp.endUts = 0 OR cp.endUts >= s.uts)
		  )
		ORDER BY s.uts DESC
		LIMIT 1`

	var uts int64
	err := r.DB.QueryRowContext(ctx, query, teamID, year, excludePostID).Scan(&uts)
	switch {
	case err == sql.ErrNoRows:
		return time.Time{}, false, nil
	case err != nil:
		return time.Time{}, false, fmt.Errorf("reading the previous checkpoint scan of %q: %w", teamID, err)
	case uts <= 0:
		return time.Time{}, false, nil
	}
	return time.Unix(uts, 0), true, nil
}

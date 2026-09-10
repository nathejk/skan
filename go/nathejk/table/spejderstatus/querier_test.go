package spejderstatus

import (
	"context"
	"strings"
	"testing"

	"github.com/nathejk/shared-go/types"
)

// The query builder is tested rather than a database, which is what the rest of this
// package does (see consumer_test.go's recordingWriter). What is worth pinning is the shape
// of the statement — placeholders instead of interpolated values, a year constraint that
// cannot be forgotten, and the ordering the shelter screen relies on — and none of that
// needs a server to assert.

func TestByStatusesQueryUsesPlaceholders(t *testing.T) {
	query, args := byStatusesQuery("2026", []types.MemberStatus{
		types.MemberStatusTransit,
		types.MemberStatusSheltered,
	})

	if !strings.Contains(query, "status IN (?,?)") {
		t.Errorf("expected two placeholders in the IN clause, got %q", query)
	}
	// The values must travel as arguments. A status interpolated into the SQL would work
	// perfectly for every value the lifecycle defines and be an injection the day a caller
	// passes something it read from a request.
	for _, s := range []string{"transit", "sheltered"} {
		if strings.Contains(query, s) {
			t.Errorf("status %q was interpolated into the statement: %q", s, query)
		}
	}
	want := []any{"2026", "transit", "sheltered"}
	if len(args) != len(want) {
		t.Fatalf("expected %d args, got %d: %v", len(want), len(args), args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Errorf("arg %d: expected %v, got %v", i, want[i], args[i])
		}
	}
}

func TestByStatusesQueryConstrainsTheYear(t *testing.T) {
	query, args := byStatusesQuery("2025", []types.MemberStatus{types.MemberStatusWaiting})

	if !strings.Contains(query, "year = ?") {
		t.Errorf("expected a year constraint, got %q", query)
	}
	// Year first, because the argument order is the one thing a reader cannot infer from the
	// statement alone, and getting it wrong would silently query the wrong year rather than
	// fail.
	if args[0] != "2025" {
		t.Errorf("expected the year as the first argument, got %v", args[0])
	}
}

// The shelter shows recent arrivals first, and the id tiebreak is what stops rows shuffling
// between two loads: a patrol starting writes its whole roster with the same timestamp.
func TestByStatusesQueryOrdersByRecencyThenID(t *testing.T) {
	query, _ := byStatusesQuery("2026", []types.MemberStatus{types.MemberStatusSheltered})

	if !strings.Contains(query, "ORDER BY updatedAt DESC, id") {
		t.Errorf("expected ordering by updatedAt DESC then id, got %q", query)
	}
}

// An empty set must not reach the database. `status IN ()` is a syntax error in MySQL, so a
// caller that filtered its own list down to nothing would get a database error where it
// meant to ask a question with an empty answer.
func TestGetByStatusesEmptySetIssuesNoQuery(t *testing.T) {
	// A nil *sql.DB is the assertion: any attempt to query would panic.
	q := &querier{}

	got, err := q.GetByStatuses(context.Background(), "2026", nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got == nil {
		t.Error("expected an empty slice rather than nil, so callers can range without a check")
	}
	if len(got) != 0 {
		t.Errorf("expected no members, got %d", len(got))
	}
}

// Every SpejderStatus read shares one column list, so a column added to the struct cannot be
// added to some queries and forgotten in others. The scan order depends on it.
func TestSelectSpejderStatusColumnOrder(t *testing.T) {
	want := "SELECT id, year, initialTeamId, currentTeamId, status, updatedAt"
	if !strings.HasPrefix(selectSpejderStatus, want) {
		t.Errorf("expected the shared column list to start %q, got %q", want, selectSpejderStatus)
	}
	if !strings.Contains(selectSpejderStatus, "FROM spejderstatus") {
		t.Errorf("expected the shared select to name its table, got %q", selectSpejderStatus)
	}
}

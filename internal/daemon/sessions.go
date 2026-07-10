package daemon

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/yasyf/cc-present/internal/state"
)

// sessionSummary is one open artifact in the GET /api/sessions listing. UpdatedAt
// serializes as an RFC3339 string via time.Time's default JSON encoding — the
// iOS client decodes it as a string.
type sessionSummary struct {
	Subject   string    `json:"subject"`
	Slug      string    `json:"slug"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updatedAt"`
	Revision  int       `json:"revision"`
}

// handleSessions lists every open artifact, most-recently-updated first, so the
// iOS client can pick one to open. The subjects table is drained fully before
// the per-subject event reads, so the single-writer connection is never held
// across the reduction (mirrors gcAssets).
func (rs *restServer) handleSessions(w http.ResponseWriter, r *http.Request) {
	summaries, err := openSessions(r.Context(), rs.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for i := range summaries {
		events, err := loadEvents(r.Context(), rs.db, summaries[i].Subject)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		st, err := state.Reduce(events)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		summaries[i].Title = st.Doc.Title
		summaries[i].Revision = docRevision(events)
	}
	writeJSON(w, http.StatusOK, summaries)
}

// openSessions reads every open subject, most-recently-updated first, fully
// draining the rows before returning so the single writer connection is freed
// for the per-subject event reads that follow.
func openSessions(ctx context.Context, db *sql.DB) ([]sessionSummary, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, slug, status, updated_at FROM subjects WHERE status=? ORDER BY updated_at DESC`, statusOpen)
	if err != nil {
		return nil, fmt.Errorf("list open subjects: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []sessionSummary{}
	for rows.Next() {
		var (
			s       sessionSummary
			updated int64
		)
		if err := rows.Scan(&s.Subject, &s.Slug, &s.Status, &updated); err != nil {
			return nil, fmt.Errorf("scan subject: %w", err)
		}
		s.UpdatedAt = time.Unix(updated, 0)
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subjects: %w", err)
	}
	return out, nil
}

// docRevision is the artifact's current revision: the count of doc.replaced
// events in its log (0 for a document never replaced).
func docRevision(events []state.Event) int {
	n := 0
	for _, ev := range events {
		if ev.Type == EventDocReplaced {
			n++
		}
	}
	return n
}

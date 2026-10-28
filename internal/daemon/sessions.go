package daemon

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"
)

// sessionSummary is one artifact in the GET /api/sessions listing. UpdatedAt
// serializes as an RFC3339 string via time.Time's default JSON encoding — the
// iOS client decodes it as a string.
type sessionSummary struct {
	Subject    string    `json:"subject"`
	Slug       string    `json:"slug"`
	SessionID  string    `json:"sessionId"`
	Title      string    `json:"title"`
	Status     string    `json:"status"`
	UpdatedAt  time.Time `json:"updatedAt"`
	Revision   int       `json:"revision"`
	EventCount int       `json:"eventCount"`
}

// handleSessions lists artifacts most-recently-updated first, so the iOS client
// can pick one to open. It lists only open artifacts by default; ?all=true
// includes closed ones, the view the sessions CLI command wants. The subjects
// table is drained fully before the per-subject event reads, so the single-writer
// connection is never held across them.
func (rs *restServer) handleSessions(w http.ResponseWriter, r *http.Request) {
	db := rs.db()
	summaries, err := listSessions(r.Context(), db, r.URL.Query().Get("all") == "true")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for i := range summaries {
		var title sql.NullString
		if err := db.QueryRowContext(r.Context(), `SELECT COUNT(*), COALESCE(SUM(type = ?), 0),
			(SELECT json_extract(payload, '$.doc.title') FROM events WHERE subject_id = ? AND type = ? ORDER BY seq DESC LIMIT 1)
			FROM events WHERE subject_id = ?`,
			EventDocReplaced, summaries[i].Subject, EventDocReplaced, summaries[i].Subject,
		).Scan(&summaries[i].EventCount, &summaries[i].Revision, &title); err != nil {
			http.Error(w, fmt.Sprintf("subject %s: %v", summaries[i].Subject, err), http.StatusInternalServerError)
			return
		}
		summaries[i].Title = title.String
	}
	writeJSON(w, http.StatusOK, summaries)
}

// listSessions reads subjects most-recently-updated first, fully draining the
// rows before returning so the single writer connection is freed for the
// per-subject event reads that follow. includeClosed lists every subject; when
// false only open subjects are returned, the iOS picker's view.
func listSessions(ctx context.Context, db *sql.DB, includeClosed bool) ([]sessionSummary, error) {
	query := `SELECT id, slug, session_id, status, updated_at FROM subjects`
	var args []any
	if !includeClosed {
		query += ` WHERE status=?`
		args = append(args, statusOpen)
	}
	query += ` ORDER BY updated_at DESC`
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list subjects: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []sessionSummary{}
	for rows.Next() {
		var (
			s       sessionSummary
			session sql.NullString
			updated int64
		)
		if err := rows.Scan(&s.Subject, &s.Slug, &session, &s.Status, &updated); err != nil {
			return nil, fmt.Errorf("scan subject: %w", err)
		}
		s.SessionID = session.String
		s.UpdatedAt = time.Unix(updated, 0)
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subjects: %w", err)
	}
	return out, nil
}

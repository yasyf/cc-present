package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	ccevent "github.com/yasyf/cc-interact/event"

	"github.com/yasyf/cc-present/internal/doc"
)

// The owner's board at 12:01Z on Oct 31, when it would not load after a daemon
// restart: 304 cards of about 30 KB, each carrying a merge group of 25 rows and
// upserted seven times within the round that answers it, 38 rounds of 8 cards,
// about 350 answered choices, and 213 answered cards retired while about 70
// stay live.
const (
	ownerBoardCards      = 304
	ownerBoardRoundSize  = 8
	ownerBoardPasses     = 7
	ownerBoardLiveCards  = 72
	ownerBoardRetired    = 213
	ownerBoardMergeRows  = 25
	coldLoadLimit        = time.Second
	firstRenderGzipLimit = 512 << 10
)

type fixtureEvent struct {
	origin  ccevent.Origin
	typ     string
	payload string
}

func ownerBoardCard(id string, pass int) string {
	rows := make([]doc.PRRow, ownerBoardMergeRows)
	for i := range rows {
		rows[i] = doc.PRRow{
			Number:  40000 + i,
			Head:    fmt.Sprintf("%040d", pass*100+i),
			Title:   fmt.Sprintf("component: change %d of %s", i, id),
			Change:  strings.Repeat("what this changes for a user. ", 3),
			Plan:    "matches the plan",
			Risk:    "low",
			Review:  "approved, CI green",
			Blocker: "none",
			Md:      strings.Repeat("row detail line with enough words to read like a real one. ", 6),
		}
	}
	card := map[string]any{
		"id": id, "type": "card", "title": "Decision " + id, "summary": strings.Repeat("summary ", 20),
		"children": []any{
			map[string]any{"id": id + "-groups", "type": "pr-groups", "repo": "Forge-AI/monorepo", "groups": []doc.PRGroup{{ID: id + "-g", Title: "Group", Summary: "what the group changes", PRs: rows}}},
			map[string]any{"id": id + "-md", "type": "markdown", "md": strings.Repeat("Context paragraph for the decision on this card. ", 160)},
			map[string]any{"id": id + "-page", "type": "display.page", "md": strings.Repeat("page body ", 600)},
			map[string]any{"id": id + "-choice", "type": "choice", "prompt": "Pick one", "options": []map[string]string{{"id": "a", "label": "A"}, {"id": "b", "label": "B"}, {"id": "c", "label": "C"}}},
			map[string]any{"id": id + "-triage", "type": "triage", "items": []map[string]string{{"id": "i1", "label": "one"}, {"id": "i2", "label": "two"}, {"id": "i3", "label": "three"}}},
		},
	}
	return string(mustJSON(map[string]any{"block": card}))
}

func ownerBoardEvents() []fixtureEvent {
	var out []fixtureEvent
	agent := func(typ, payload string) { out = append(out, fixtureEvent{ccevent.OriginAgent, typ, payload}) }
	human := func(typ, payload string) { out = append(out, fixtureEvent{ccevent.OriginHuman, typ, payload}) }
	cardID := func(n int) string { return fmt.Sprintf("card-%03d", n) }
	retired := 0
	for first := 0; first < ownerBoardCards; first += ownerBoardRoundSize {
		for pass := range ownerBoardPasses {
			for n := first; n < first+ownerBoardRoundSize; n++ {
				id := cardID(n)
				agent(EventBlockUpserted, ownerBoardCard(id, pass))
				if (n+pass)%9 != 0 {
					human("triage.decided", fmt.Sprintf(`{"blockId":"%s-triage","verdicts":{"i%d":{"verdict":"approved"}}}`, id, 1+(n+pass)%3))
				}
				if pass == 1 || (pass == 4 && n == first) {
					human("choice.selected", fmt.Sprintf(`{"blockId":"%s-choice","optionIds":["a"]}`, id))
				}
				if (n+pass)%21 == 0 {
					agent("reply.created", fmt.Sprintf(`{"id":"r-%s-%d","blockId":"%s","md":"noted"}`, id, pass, id))
				}
			}
		}
		human("submit", `{"revision":1,"viewed":[]}`)
		for n := first - ownerBoardLiveCards; n < first-ownerBoardLiveCards+ownerBoardRoundSize; n++ {
			if n >= 0 && retired < ownerBoardRetired {
				agent("block.removed", fmt.Sprintf(`{"id":%q}`, cardID(n)))
				retired++
			}
		}
	}
	return out
}

func seedOwnerBoard(t *testing.T, h *restHarness) (events, payloadBytes int) {
	t.Helper()
	ctx := context.Background()
	tx, err := h.cc.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	var seq int64
	if err := tx.QueryRowContext(ctx, `SELECT MAX(seq) FROM events WHERE subject_id=?`, h.id).Scan(&seq); err != nil {
		t.Fatalf("max seq: %v", err)
	}
	for _, ev := range ownerBoardEvents() {
		seq++
		payload := injectIdentity(ev.typ, json.RawMessage(ev.payload))
		payloadBytes += len(payload)
		if _, err := tx.ExecContext(ctx, `INSERT INTO events(subject_id, seq, origin, type, payload, created_at) VALUES (?,?,?,?,?,?)`,
			h.id, seq, string(ev.origin), ev.typ, string(payload), seq); err != nil {
			t.Fatalf("insert event %d: %v", seq, err)
		}
		events++
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return events, payloadBytes
}

// TestColdLoadAtOwnerBoardSize replays the owner's board as a restarted daemon
// sees it: the first /api/state reduces the whole log, and must answer within
// coldLoadLimit with a gzipped first render under firstRenderGzipLimit.
func TestColdLoadAtOwnerBoardSize(t *testing.T) {
	h := newRestHarnessWith(t, `{"version":1,"title":"T","blocks":[]}`, emptyPackLoader(t))
	events, payloadBytes := seedOwnerBoard(t, h)

	req := httptest.NewRequest(http.MethodGet, "/api/state?session=board--abcd0000", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	start := time.Now()
	h.mux().ServeHTTP(w, req)
	took := time.Since(start)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", w.Code, w.Body.String())
	}
	t.Logf("cold /api/state over %d events (%d MB) took %v, first render %d KB gzipped", events, payloadBytes>>20, took, w.Body.Len()>>10)
	if limit := coldLoadLimit * raceSlowdown; took > limit {
		t.Errorf("cold /api/state took %v, want under %v", took, limit)
	}
	if w.Body.Len() > firstRenderGzipLimit {
		t.Errorf("first render is %d KB gzipped, want under %d KB", w.Body.Len()>>10, firstRenderGzipLimit>>10)
	}
}

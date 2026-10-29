package daemon

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	ccd "github.com/yasyf/cc-interact/daemon"
	ccevent "github.com/yasyf/cc-interact/event"
)

// Since values an outcomes read accepts besides an event seq and an RFC 3339
// time.
const (
	// SinceAuto reads the delta since the last submit once the board has one,
	// and the whole state before that.
	SinceAuto = "auto"
	// SinceAll reads the whole state.
	SinceAll = "all"
	// SinceLastSubmit reads what the newest submit delivered and everything
	// after it: the interactions after the submit before it.
	SinceLastSubmit = "last-submit"
	// SinceNew reads the interactions after the previous outcomes read of the
	// board.
	SinceNew = "new"
)

// Delta is the human interactions on a board after one point of its log,
// oldest first. Through is the newest seq the read covers; a later read passes
// it as since to pick up where this one stopped.
type Delta struct {
	Since        int64              `json:"since"`
	Through      int64              `json:"through"`
	Interactions []DeltaInteraction `json:"interactions"`
}

// DeltaInteraction is one human interaction: its event seq and type, the block
// it answers, when it landed, and its payload without the identity keys. A
// choice, input, decision or pack value appears once per block, at its latest
// write.
type DeltaInteraction struct {
	Seq     int64           `json:"seq"`
	Type    string          `json:"type"`
	BlockID string          `json:"blockId,omitempty"`
	At      time.Time       `json:"at"`
	Payload json.RawMessage `json:"payload"`
}

var deltaTypes = []any{
	EventDecisionCreated, EventChoiceSelected, EventInputSubmitted, EventPackInteraction,
	EventFeedbackCreated, EventAnnotationCreated, EventAnnotationRemoved, EventTriageDecided, EventSubmit,
}

var lastWriteWins = map[string]bool{
	EventDecisionCreated: true, EventChoiceSelected: true, EventInputSubmitted: true, EventPackInteraction: true,
}

// handleOutcomes returns the board's reduced state, or the delta of human
// interactions since the point the request names, and records the read in the
// board's cursor under cursors for a later read since "new".
func handleOutcomes(hc ccd.HandlerCtx, cursors string) ccd.Reply {
	sub, err := resolve(hc)
	if err != nil {
		return errReply(err.Error())
	}
	req := decodeBody(hc.Env.Body)
	cursor := filepath.Join(cursors, sub.ID+".cursor")
	since, delta, err := outcomesSince(hc.Ctx, hc.DB, sub.ID, req.Since, cursor)
	if err != nil {
		return errReply(err.Error())
	}
	board, err := caughtUp(hc.Ctx, hc.DB, sub.ID)
	if err != nil {
		return errReply(err.Error())
	}
	head := board.seq
	res := result{Retired: newAnswerGate(hc.Ctx, hc.DB, sub.ID, &board.st, board.retired, nil).retiredIDs()}
	if !delta {
		res.State, err = outcomeState(&board.st, req.View)
	}
	board.mu.Unlock()
	if err == nil && delta {
		res.Delta, err = loadDelta(hc.Ctx, hc.DB, sub.ID, since, head)
	}
	if err == nil && req.Since != "" {
		err = writeCursor(cursor, head)
	}
	if err != nil {
		return errReply(err.Error())
	}
	return okReply(res)
}

func outcomesSince(ctx context.Context, db *sql.DB, subjectID, since, cursor string) (int64, bool, error) {
	switch since {
	case "", SinceAll:
		return 0, false, nil
	case SinceNew:
		seq, err := readCursor(cursor)
		return seq, true, err
	case SinceAuto, SinceLastSubmit:
		submits, err := queryInts(ctx, db, `SELECT seq FROM events WHERE subject_id=? AND type=? ORDER BY seq DESC LIMIT 2`, subjectID, EventSubmit)
		if err != nil {
			return 0, false, fmt.Errorf("load submits: %w", err)
		}
		if len(submits) == 0 {
			return 0, since == SinceLastSubmit, nil
		}
		if len(submits) == 1 {
			return 0, true, nil
		}
		return submits[1], true, nil
	}
	if seq, err := strconv.ParseInt(since, 10, 64); err == nil {
		return seq, true, nil
	}
	t, err := time.Parse(time.RFC3339, since)
	if err != nil {
		return 0, false, fmt.Errorf("since %q is not an event seq, %s, %s, or an RFC 3339 time", since, SinceLastSubmit, SinceNew)
	}
	before, err := queryInts(ctx, db, `SELECT COALESCE(MAX(seq), 0) FROM events WHERE subject_id=? AND created_at<?`, subjectID, t.Unix())
	if err != nil {
		return 0, false, fmt.Errorf("find seq before %s: %w", since, err)
	}
	return before[0], true, nil
}

func loadDelta(ctx context.Context, db *sql.DB, subjectID string, since, through int64) (json.RawMessage, error) {
	args := append([]any{subjectID, ccevent.OriginHuman, since, through}, deltaTypes...)
	rows, err := db.QueryContext(ctx, `SELECT seq, type, payload, created_at FROM events WHERE subject_id=? AND origin=? AND seq>? AND seq<=? AND type IN (?,?,?,?,?,?,?,?,?) ORDER BY seq ASC`, args...)
	if err != nil {
		return nil, fmt.Errorf("load interactions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var all []DeltaInteraction
	for rows.Next() {
		var (
			in      DeltaInteraction
			payload string
			created int64
		)
		if err := rows.Scan(&in.Seq, &in.Type, &payload, &created); err != nil {
			return nil, fmt.Errorf("scan interaction: %w", err)
		}
		var p map[string]json.RawMessage
		if err := json.Unmarshal([]byte(payload), &p); err != nil {
			return nil, fmt.Errorf("read %s at seq %d: %w", in.Type, in.Seq, err)
		}
		if id, ok := p["blockId"]; ok {
			if err := json.Unmarshal(id, &in.BlockID); err != nil {
				return nil, fmt.Errorf("read %s at seq %d: %w", in.Type, in.Seq, err)
			}
		}
		delete(p, "blockId")
		delete(p, "type")
		delete(p, "schemaVersion")
		in.At, in.Payload = time.Unix(created, 0), mustJSON(p)
		all = append(all, in)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return json.Marshal(Delta{Since: since, Through: through, Interactions: latestPerBlock(all)})
}

// latestPerBlock keeps only the newest write of each last-write-wins value per
// block, so a write-in typed in four saves reads as its final text.
func latestPerBlock(all []DeltaInteraction) []DeltaInteraction {
	last := map[string]int64{}
	for _, in := range all {
		if lastWriteWins[in.Type] {
			last[in.Type+"\x00"+in.BlockID] = in.Seq
		}
	}
	out := []DeltaInteraction{}
	for _, in := range all {
		if !lastWriteWins[in.Type] || last[in.Type+"\x00"+in.BlockID] == in.Seq {
			out = append(out, in)
		}
	}
	return out
}

func queryInts(ctx context.Context, db *sql.DB, query string, args ...any) ([]int64, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []int64
	for rows.Next() {
		var n int64
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func readCursor(path string) (int64, error) {
	//nolint:gosec // G304: path is the daemon's state dir joined with a subject id it resolved itself.
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read outcomes cursor: %w", err)
	}
	seq, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("read outcomes cursor %s: %w", path, err)
	}
	return seq, nil
}

func writeCursor(path string, seq int64) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("write outcomes cursor: %w", err)
	}
	if err := os.WriteFile(path, []byte(strconv.FormatInt(seq, 10)), 0o600); err != nil {
		return fmt.Errorf("write outcomes cursor: %w", err)
	}
	return nil
}

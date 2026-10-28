package daemon

import (
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/yasyf/cc-present/internal/state"
)

var boards sync.Map

type boardKey struct {
	db *sql.DB
	id string
}

// reducedBoard is one subject's reduced state, caught up from the last seq it
// applied, so a tap, an op or a page load decodes only the events since the
// previous one instead of the board's whole history. Every read of st and every
// load-derive-append against the board holds mu. A tap holds answering from its
// validation until a Slack-synced card has its answer, so a second pick on the
// card sees it resolved, while mu is free for the Slack answer to land.
type reducedBoard struct {
	answering sync.Mutex
	mu        sync.Mutex
	seq       int64
	revision  int
	events    int
	st        state.State
	retired   map[string]bool
	snapshot  encoded
	rounds    encoded
}

type encoded struct {
	seq  int64
	body []byte
	gz   []byte
}

func caughtUp(ctx context.Context, db *sql.DB, id string) (*reducedBoard, error) {
	board := boardFor(db, id)
	if err := board.lock(ctx, db, id); err != nil {
		return nil, err
	}
	return board, nil
}

func boardFor(db *sql.DB, id string) *reducedBoard {
	cached, _ := boards.LoadOrStore(boardKey{db: db, id: id}, &reducedBoard{})
	return cached.(*reducedBoard)
}

func (b *reducedBoard) lock(ctx context.Context, db *sql.DB, id string) error {
	b.mu.Lock()
	if err := b.catchUp(ctx, db, id); err != nil {
		b.mu.Unlock()
		return err
	}
	if b.st.Interactions.Closed.Value {
		boards.Delete(boardKey{db: db, id: id})
	}
	return nil
}

func forgetBoardsExcept(db *sql.DB, open []string) {
	boards.Range(func(k, _ any) bool {
		if key := k.(boardKey); key.db == db && !slices.Contains(open, key.id) {
			boards.Delete(key)
		}
		return true
	})
}

func (b *reducedBoard) catchUp(ctx context.Context, db *sql.DB, subID string) error {
	events, err := loadEventsAfter(ctx, db, subID, b.seq)
	if err != nil {
		return err
	}
	if b.seq == 0 {
		b.st, b.retired, b.revision, b.events = state.New(), map[string]bool{}, 0, 0
	}
	if err := b.st.ApplyRetiring(events, b.retired); err != nil {
		b.seq = 0
		return err
	}
	for _, ev := range events {
		if ev.Type == EventDocReplaced {
			b.revision++
		}
		b.events++
		b.seq = ev.Seq
	}
	return nil
}

func (b *reducedBoard) encode(slot *encoded, v func() (any, error)) (encoded, error) {
	if slot.body != nil && slot.seq == b.seq {
		return *slot, nil
	}
	val, err := v()
	if err != nil {
		return encoded{}, err
	}
	body, err := json.Marshal(val)
	if err != nil {
		return encoded{}, err
	}
	gz, err := gzipBytes(body, gzip.DefaultCompression)
	if err != nil {
		return encoded{}, err
	}
	*slot = encoded{seq: b.seq, body: body, gz: gz}
	return *slot, nil
}

func warmBoards(ctx context.Context, db *sql.DB) error {
	ids, err := openSubjectIDs(ctx, db)
	if err != nil {
		return err
	}
	var errs []error
	for _, id := range ids {
		board, err := caughtUp(ctx, db, id)
		if err != nil {
			errs = append(errs, fmt.Errorf("subject %s: %w", id, err))
			continue
		}
		board.mu.Unlock()
	}
	return errors.Join(errs...)
}

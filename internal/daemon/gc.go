package daemon

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/yasyf/cc-present/internal/assets"
	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/state"
)

// assetGrace is how recently an asset must have been written to survive a sweep
// while still unreferenced; it guards an upload that lands between the sweep's
// subject enumeration and its deletion pass.
const assetGrace = 15 * time.Minute

// gcAssets collects every asset referenced by an open subject's live document and
// deletes every unreferenced file older than assetGrace. Assets referenced only
// by closed subjects are unreferenced, so they are collectable. The store DB is
// read fully — the open subject ids, then each subject's log — before any
// filesystem work, so the single writer connection is never held across I/O.
func gcAssets(ctx context.Context, db *sql.DB, ast *assets.Store) error {
	ids, err := openSubjectIDs(ctx, db)
	if err != nil {
		return err
	}
	keep := map[string]bool{}
	for _, id := range ids {
		events, err := loadEvents(ctx, db, id)
		if err != nil {
			return err
		}
		st, err := state.Reduce(events)
		if err != nil {
			return fmt.Errorf("reduce subject %s: %w", id, err)
		}
		for _, b := range st.Doc.Blocks {
			for _, sha := range doc.AssetRefs(b) {
				keep[sha] = true
			}
		}
	}
	if _, err := ast.Sweep(keep, assetGrace); err != nil {
		return fmt.Errorf("sweep assets: %w", err)
	}
	return nil
}

// openSubjectIDs returns the ids of every open subject. subject.Store has no
// cross-subject query, so this reads the subjects table through the store DB
// escape hatch and fully drains the rows before returning, freeing the single
// connection for the per-subject event reads that follow.
func openSubjectIDs(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT id FROM subjects WHERE status=?`, statusOpen)
	if err != nil {
		return nil, fmt.Errorf("list open subjects: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan subject id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subjects: %w", err)
	}
	return ids, nil
}

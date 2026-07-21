package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yasyf/cc-interact/cmd"

	"github.com/yasyf/cc-present/internal/app"
	ccdaemon "github.com/yasyf/cc-present/internal/daemon"
	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/packs"
	"github.com/yasyf/cc-present/internal/version"
)

const noArtifact = "no cc-present artifact for this window; run `cc-present start` first"

func mustCwd(cwd string) string {
	if cwd != "" {
		return cwd
	}
	d, _ := os.Getwd()
	return d
}

// sessionOr defaults an empty --session to the Claude session id the plugin sets,
// so a subject resolves consistently across separate CLI invocations.
func sessionOr(s string) string {
	if s != "" {
		return s
	}
	return os.Getenv("CLAUDE_CODE_SESSION_ID")
}

// readInput reads a document/block argument from a file, or from in for "-".
func readInput(arg string, in io.Reader) ([]byte, error) {
	if arg == "-" {
		return io.ReadAll(in)
	}
	//nolint:gosec // G304: reading the document/block file the user named is the command's purpose.
	return os.ReadFile(arg)
}

// validateRoundFlags rejects an unknown --round value and a --round-title given
// without --round new — the two client-side usage errors caught before dialing.
func validateRoundFlags(round, roundTitle string) error {
	switch round {
	case "", "current", "new":
	default:
		return errors.New(`--round must be "current" or "new"`)
	}
	if roundTitle != "" && round != "new" {
		return errors.New(`--round-title requires --round new`)
	}
	return nil
}

func client(ctx context.Context, d cmd.Deps) (*ccdaemon.Client, error) {
	raw, err := d.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	return ccdaemon.NewClient(raw), nil
}

// jsonErrorAt annotates a JSON syntax error with the line and column of the
// offending byte in raw; any other error passes through unchanged. A nested
// block's type error offsets a RawMessage sub-slice, not raw, so it stays bare.
func jsonErrorAt(raw []byte, err error) error {
	var se *json.SyntaxError
	if !errors.As(err, &se) {
		return err
	}
	line, col := lineCol(raw, se.Offset)
	return fmt.Errorf("%w (line %d, column %d)", err, line, col)
}

// lineCol maps a 1-based json byte offset (the count read when the error fired,
// so the offending byte is the last one) to its 1-based line and column.
func lineCol(data []byte, offset int64) (int, int) {
	if offset > int64(len(data)) {
		offset = int64(len(data))
	}
	pos := offset - 1
	if pos < 0 {
		pos = 0
	}
	line, col := 1, 1
	for _, b := range data[:pos] {
		if b == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return line, col
}

// dryRunReport inlines any local image against the content-addressed store, then
// runs the whole-document validator, returning the text to print and whether the
// document passed; a false result maps to a non-zero exit at the call site.
func dryRunReport(dd *doc.Doc, pt doc.PackTypes) (string, bool) {
	if err := inlineImages(dd.Blocks, localUploader); err != nil {
		return err.Error(), false
	}
	if err := dd.Validate(pt); err != nil {
		return err.Error(), false
	}
	return "ok", true
}

// visualNudge returns a single-line, non-blocking reminder naming every choice
// that ships without a visual, or "" when every choice carries one. A choice is
// satisfied by an option.visual, a diagram/image sibling in its card, or a
// top-level diagram/image the forward-attaching run leads into it.
func visualNudge(dd *doc.Doc) string {
	var proseOnly []string
	leadHasVisual := false
	for _, b := range dd.Blocks {
		switch b := b.(type) {
		case *doc.Choice:
			if len(doc.Visuals(b)) == 0 && !leadHasVisual {
				proseOnly = append(proseOnly, b.ID)
			}
		case *doc.Card:
			cardHasVisual := cardHasVisualLeadIn(b)
			for _, child := range b.Children {
				if c, ok := child.(*doc.Choice); ok && len(doc.Visuals(c)) == 0 && !cardHasVisual {
					proseOnly = append(proseOnly, c.ID)
				}
			}
		}
		leadHasVisual = continuesVisualRun(b, leadHasVisual)
	}
	if len(proseOnly) == 0 {
		return ""
	}
	subject := "choice ships"
	if len(proseOnly) > 1 {
		subject = "choices ship"
	}
	return fmt.Sprintf("hint: %d %s without a visual (%s); attach an option.visual or lead the card with a diagram",
		len(proseOnly), subject, strings.Join(proseOnly, ", "))
}

// isVisualLeadIn reports whether b is a rendered-picture block — diagram, image,
// chart, or filetree — the block types that satisfy a choice by leading it in.
func isVisualLeadIn(b doc.Block) bool {
	switch b.BlockType() {
	case "diagram", "image", "chart", "filetree":
		return true
	default:
		return false
	}
}

// cardHasVisualLeadIn reports whether a card carries a diagram or image sibling
// that leads in every choice it nests.
func cardHasVisualLeadIn(card *doc.Card) bool {
	for _, child := range card.Children {
		if isVisualLeadIn(child) {
			return true
		}
	}
	return false
}

// continuesVisualRun reports whether the top-level forward-attaching run still
// carries a diagram/image once b is consumed: a visual sustains it, a context
// block passes it through, a decidable/card/section ends it.
func continuesVisualRun(b doc.Block, running bool) bool {
	if isVisualLeadIn(b) {
		return true
	}
	switch b.BlockType() {
	case "markdown", "code", "diff", "table", "progress", "term", "record":
		return running
	default:
		return false
	}
}

// blockDoc wraps a lone block in a minimal envelope so update-block --dry-run
// reuses the whole-document validator, checking the block as the top-level block
// update-block inserts it as.
func blockDoc(b doc.Block) *doc.Doc {
	return &doc.Doc{Version: 1, Title: "dry-run", Blocks: []doc.Block{b}}
}

// stripDocKey drops the top-level "doc" key from reduced-state JSON so
// outcomes --no-doc emits only the human interactions and round partition.
func stripDocKey(raw json.RawMessage) (json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	delete(m, "doc")
	return json.Marshal(m)
}

// newStartCmd creates or resumes this window's artifact and prints its ref, URL,
// and channel state, one per line.
func newStartCmd(d cmd.Deps) *cobra.Command {
	var session, cwd, title, docPath string
	var fresh bool
	c := &cobra.Command{
		Use:   "start",
		Short: "Create or resume this window's cc-present artifact and print its URL",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			ctx := c.Context()
			if err := d.EnsureCurrent(ctx); err != nil {
				return err
			}
			cl, err := client(ctx, d)
			if err != nil {
				return err
			}
			defer func() { _ = cl.CloseSession() }()
			sess, scope, pid := sessionOr(session), mustCwd(cwd), d.ClaudePID()
			var docJSON json.RawMessage
			if docPath != "" {
				raw, err := readInput(docPath, c.InOrStdin())
				if err != nil {
					return err
				}
				dd := &doc.Doc{}
				if err := json.Unmarshal(raw, dd); err != nil {
					return fmt.Errorf("decode doc: %w", err)
				}
				_, port, err := cl.Resolve(ctx, sess, scope, pid)
				if err != nil {
					return err
				}
				if err := inlineImages(dd.Blocks, httpUploader(port)); err != nil {
					return err
				}
				if docJSON, err = json.Marshal(dd); err != nil {
					return err
				}
			}
			res, err := cl.Start(ctx, sess, scope, pid, fresh, title, docJSON)
			if err != nil {
				return err
			}
			out := c.OutOrStdout()
			_, _ = fmt.Fprintf(out, "session: %s\n", res.SubjectID)
			_, _ = fmt.Fprintf(out, "url: %s\n", res.URL)
			for _, u := range res.TailnetURLs {
				_, _ = fmt.Fprintf(out, "tailnet: %s\n", u)
			}
			_, _ = fmt.Fprintf(out, "channel: %s\n", res.ChannelState)
			return nil
		},
	}
	c.Flags().StringVar(&session, "session", "", "Claude session id (defaults to $CLAUDE_CODE_SESSION_ID)")
	c.Flags().StringVar(&cwd, "cwd", "", "working directory (recorded on the request; artifacts are per-window, not resolved by directory)")
	c.Flags().StringVar(&title, "title", "", "artifact title used for the URL slug when no --doc is given")
	c.Flags().BoolVar(&fresh, "new", false, "force a fresh artifact, detaching any existing one for this window")
	c.Flags().StringVar(&docPath, "doc", "", "seed the artifact with a document from a file (- for stdin)")
	return c
}

// newPushCmd replaces the artifact's document. --dry-run validates the document
// locally and exits non-zero with the validation error on stdout. --round
// current|new declares intent when the push adds a top-level block to an engaged
// round, and --round-title titles a round --round new opens.
func newPushCmd(d cmd.Deps) *cobra.Command {
	var session, cwd, round, roundTitle string
	var dryRun bool
	c := &cobra.Command{
		Use:   "push <file|->",
		Short: "Replace the artifact's document",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			if err := validateRoundFlags(round, roundTitle); err != nil {
				return err
			}
			raw, err := readInput(args[0], c.InOrStdin())
			if err != nil {
				return err
			}
			dd := &doc.Doc{}
			if err := json.Unmarshal(raw, dd); err != nil {
				return fmt.Errorf("decode doc: %w", jsonErrorAt(raw, err))
			}
			if dryRun {
				cfg, err := app.ReadConfig()
				if err != nil {
					return err
				}
				if hint := visualNudge(dd); hint != "" {
					_, _ = fmt.Fprintln(c.ErrOrStderr(), hint)
				}
				msg, ok := dryRunReport(dd, packs.Load(cfg.PackDirs, cfg.DisabledPacks))
				_, _ = fmt.Fprintln(c.OutOrStdout(), msg)
				if !ok {
					os.Exit(1)
				}
				return nil
			}
			ctx := c.Context()
			if err := d.EnsureCurrent(ctx); err != nil {
				return err
			}
			cl, err := client(ctx, d)
			if err != nil {
				return err
			}
			defer func() { _ = cl.CloseSession() }()
			sess, scope, pid := sessionOr(session), mustCwd(cwd), d.ClaudePID()
			subjectID, port, err := cl.Resolve(ctx, sess, scope, pid)
			if err != nil {
				return err
			}
			if subjectID == "" {
				return errors.New(noArtifact)
			}
			if err := inlineImages(dd.Blocks, httpUploader(port)); err != nil {
				return err
			}
			docJSON, err := json.Marshal(dd)
			if err != nil {
				return err
			}
			res, err := cl.Push(ctx, sess, scope, pid, docJSON, round, roundTitle)
			if err != nil {
				return err
			}
			out := c.OutOrStdout()
			_, _ = fmt.Fprintf(out, "revision: %d\n", res.Revision)
			if res.Round > 0 {
				_, _ = fmt.Fprintf(out, "round: %d\n", res.Round)
			}
			_, _ = fmt.Fprintf(out, "url: %s\n", res.URL)
			for _, u := range res.TailnetURLs {
				_, _ = fmt.Fprintf(out, "tailnet: %s\n", u)
			}
			if hint := visualNudge(dd); hint != "" {
				_, _ = fmt.Fprintln(c.ErrOrStderr(), hint)
			}
			return nil
		},
	}
	c.Flags().StringVar(&session, "session", "", "Claude session id (defaults to $CLAUDE_CODE_SESSION_ID)")
	c.Flags().StringVar(&cwd, "cwd", "", "working directory (recorded on the request; artifacts are per-window, not resolved by directory)")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "validate the document only; print every violation and exit non-zero")
	c.Flags().StringVar(&round, "round", "", `round intent when a new top-level block lands in an engaged round: "current" (add to it) or "new" (close it and open the next)`)
	c.Flags().StringVar(&roundTitle, "round-title", "", "title for the round opened by --round new")
	return c
}

// newUpdateBlockCmd inserts or replaces a single block, optionally after another.
// --round current|new declares intent when the upsert adds a top-level block to
// an engaged round, and --round-title titles a round --round new opens; it prints
// round: N only when the upsert opened round N.
func newUpdateBlockCmd(d cmd.Deps) *cobra.Command {
	var session, cwd, after, round, roundTitle string
	var dryRun bool
	c := &cobra.Command{
		Use:   "update-block <file|->",
		Short: "Insert or replace a single block",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			if err := validateRoundFlags(round, roundTitle); err != nil {
				return err
			}
			raw, err := readInput(args[0], c.InOrStdin())
			if err != nil {
				return err
			}
			blk, err := doc.DecodeBlock(raw)
			if err != nil {
				return jsonErrorAt(raw, err)
			}
			if dryRun {
				cfg, err := app.ReadConfig()
				if err != nil {
					return err
				}
				msg, ok := dryRunReport(blockDoc(blk), packs.Load(cfg.PackDirs, cfg.DisabledPacks))
				_, _ = fmt.Fprintln(c.OutOrStdout(), msg)
				if !ok {
					os.Exit(1)
				}
				return nil
			}
			ctx := c.Context()
			if err := d.EnsureCurrent(ctx); err != nil {
				return err
			}
			cl, err := client(ctx, d)
			if err != nil {
				return err
			}
			defer func() { _ = cl.CloseSession() }()
			sess, scope, pid := sessionOr(session), mustCwd(cwd), d.ClaudePID()
			subjectID, port, err := cl.Resolve(ctx, sess, scope, pid)
			if err != nil {
				return err
			}
			if subjectID == "" {
				return errors.New(noArtifact)
			}
			if err := inlineImages([]doc.Block{blk}, httpUploader(port)); err != nil {
				return err
			}
			blockJSON, err := json.Marshal(blk)
			if err != nil {
				return err
			}
			n, err := cl.UpsertBlock(ctx, sess, scope, pid, blockJSON, after, round, roundTitle)
			if err != nil {
				return err
			}
			if n > 0 {
				_, _ = fmt.Fprintf(c.OutOrStdout(), "round: %d\n", n)
			}
			return nil
		},
	}
	c.Flags().StringVar(&session, "session", "", "Claude session id (defaults to $CLAUDE_CODE_SESSION_ID)")
	c.Flags().StringVar(&cwd, "cwd", "", "working directory (recorded on the request; artifacts are per-window, not resolved by directory)")
	c.Flags().StringVar(&after, "after", "", "insert a new block after a top-level block, or into a card after a child (unknown ids error)")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "validate the single block only; print every violation and exit non-zero")
	c.Flags().StringVar(&round, "round", "", `round intent when the block is a new top-level block in an engaged round: "current" (add to it) or "new" (close it and open the next)`)
	c.Flags().StringVar(&roundTitle, "round-title", "", "title for the round opened by --round new")
	return c
}

// newRemoveBlockCmd removes a block by id.
func newRemoveBlockCmd(d cmd.Deps) *cobra.Command {
	var session, cwd string
	c := &cobra.Command{
		Use:   "remove-block <id>",
		Short: "Remove a block by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			ctx := c.Context()
			if err := d.EnsureCurrent(ctx); err != nil {
				return err
			}
			cl, err := client(ctx, d)
			if err != nil {
				return err
			}
			defer func() { _ = cl.CloseSession() }()
			return cl.RemoveBlock(ctx, sessionOr(session), mustCwd(cwd), d.ClaudePID(), args[0])
		},
	}
	c.Flags().StringVar(&session, "session", "", "Claude session id (defaults to $CLAUDE_CODE_SESSION_ID)")
	c.Flags().StringVar(&cwd, "cwd", "", "working directory (recorded on the request; artifacts are per-window, not resolved by directory)")
	return c
}

// newReplyCmd appends an agent reply to a block's thread.
func newReplyCmd(d cmd.Deps) *cobra.Command {
	var session, cwd, block, bodyMd string
	c := &cobra.Command{
		Use:   "reply",
		Short: "Append an agent reply to a block's thread",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			if block == "" {
				return errors.New("reply requires --block")
			}
			if bodyMd == "" {
				return errors.New("reply requires --body")
			}
			ctx := c.Context()
			if err := d.EnsureCurrent(ctx); err != nil {
				return err
			}
			cl, err := client(ctx, d)
			if err != nil {
				return err
			}
			defer func() { _ = cl.CloseSession() }()
			if err := cl.Reply(ctx, sessionOr(session), mustCwd(cwd), d.ClaudePID(), block, bodyMd); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(c.OutOrStdout(), "replied: %s\n", block)
			return nil
		},
	}
	c.Flags().StringVar(&block, "block", "", "block id to reply under")
	c.Flags().StringVar(&bodyMd, "body", "", "reply markdown")
	c.Flags().StringVar(&session, "session", "", "Claude session id (defaults to $CLAUDE_CODE_SESSION_ID)")
	c.Flags().StringVar(&cwd, "cwd", "", "working directory (recorded on the request; artifacts are per-window, not resolved by directory)")
	return c
}

// newRoundCmd force-advances the round or titles the current one, printing the
// resulting round number.
func newRoundCmd(d cmd.Deps) *cobra.Command {
	var session, cwd, title string
	c := &cobra.Command{
		Use:   "round",
		Short: "Force-advance the round or title the current one",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			ctx := c.Context()
			if err := d.EnsureCurrent(ctx); err != nil {
				return err
			}
			cl, err := client(ctx, d)
			if err != nil {
				return err
			}
			defer func() { _ = cl.CloseSession() }()
			n, err := cl.Round(ctx, sessionOr(session), mustCwd(cwd), d.ClaudePID(), title)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(c.OutOrStdout(), "round: %d\n", n)
			return nil
		},
	}
	c.Flags().StringVar(&title, "title", "", "title for the round (optional)")
	c.Flags().StringVar(&session, "session", "", "Claude session id (defaults to $CLAUDE_CODE_SESSION_ID)")
	c.Flags().StringVar(&cwd, "cwd", "", "working directory (recorded on the request; artifacts are per-window, not resolved by directory)")
	return c
}

// newRevisingCmd declares the block ids the agent is revising, with an optional
// note. Child ids resolve to their enclosing cards. No ids and no note — or
// --clear — abandons the announcement; no ids with a note is the doc-level
// drafting state. --clear rejects block ids and --note.
func newRevisingCmd(d cmd.Deps) *cobra.Command {
	var session, cwd, note string
	var clearFlag bool
	c := &cobra.Command{
		Use:   "revising [blockId...]",
		Short: "Declare the block ids the agent is revising, or abandon the announcement",
		Args:  cobra.ArbitraryArgs,
		RunE: func(c *cobra.Command, args []string) error {
			if clearFlag && (len(args) > 0 || note != "") {
				return errors.New("--clear takes no block ids and no --note")
			}
			ctx := c.Context()
			if err := d.EnsureCurrent(ctx); err != nil {
				return err
			}
			cl, err := client(ctx, d)
			if err != nil {
				return err
			}
			defer func() { _ = cl.CloseSession() }()
			return cl.Revising(ctx, sessionOr(session), mustCwd(cwd), d.ClaudePID(), args, note)
		},
	}
	c.Flags().StringVar(&note, "note", "", "note shown with the revising announcement")
	c.Flags().BoolVar(&clearFlag, "clear", false, "abandon the announcement — sends the empty set with no note (same as a bare call with no ids)")
	c.Flags().StringVar(&session, "session", "", "Claude session id (defaults to $CLAUDE_CODE_SESSION_ID)")
	c.Flags().StringVar(&cwd, "cwd", "", "working directory (recorded on the request; artifacts are per-window, not resolved by directory)")
	return c
}

// newOutcomesCmd prints the artifact's reduced state as JSON — the post-submit
// drain.
func newOutcomesCmd(d cmd.Deps) *cobra.Command {
	var session, cwd, block string
	var noDoc bool
	c := &cobra.Command{
		Use:   "outcomes",
		Short: "Print the artifact's reduced document and human interactions as JSON",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			ctx := c.Context()
			if err := d.EnsureCurrent(ctx); err != nil {
				return err
			}
			cl, err := client(ctx, d)
			if err != nil {
				return err
			}
			defer func() { _ = cl.CloseSession() }()
			raw, err := cl.Outcomes(ctx, sessionOr(session), mustCwd(cwd), d.ClaudePID())
			if err != nil {
				return err
			}
			if block != "" {
				raw, err = filterBlock(raw, block)
				if err != nil {
					return err
				}
			}
			if noDoc {
				raw, err = stripDocKey(raw)
				if err != nil {
					return err
				}
			}
			var buf bytes.Buffer
			if err := json.Indent(&buf, raw, "", "  "); err != nil {
				return err
			}
			_, _ = fmt.Fprintln(c.OutOrStdout(), buf.String())
			return nil
		},
	}
	c.Flags().BoolVar(&noDoc, "no-doc", false, "omit the reduced document, printing only the human interactions and rounds")
	c.Flags().StringVar(&block, "block", "", "filter the state to a single block id (a child id filters to its own entries; the doc keeps the enclosing card)")
	c.Flags().StringVar(&session, "session", "", "Claude session id (defaults to $CLAUDE_CODE_SESSION_ID)")
	c.Flags().StringVar(&cwd, "cwd", "", "working directory (recorded on the request; artifacts are per-window, not resolved by directory)")
	return c
}

// filterBlock narrows reduced-state JSON to a single block: the document keeps
// only the enclosing top-level block's subtree, and each block-keyed interaction
// map keeps only the block's own entries. An id no block carries is an error. A
// child id resolves through doc.Locate — its own interactions, its enclosing
// card's subtree.
func filterBlock(raw json.RawMessage, id string) (json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	var dd doc.Doc
	if err := json.Unmarshal(m["doc"], &dd); err != nil {
		return nil, err
	}
	loc, ok := doc.Locate(&dd, id)
	if !ok {
		return nil, fmt.Errorf("no block %q in the current document", id)
	}
	kept := make([]doc.Block, 0, 1)
	for _, b := range dd.Blocks {
		if b.BlockID() == loc.TopID {
			kept = append(kept, b)
		}
	}
	dd.Blocks = kept
	docJSON, err := json.Marshal(&dd)
	if err != nil {
		return nil, err
	}
	m["doc"] = docJSON
	if inter, ok := m["interactions"]; ok {
		filtered, err := filterInteractions(inter, id)
		if err != nil {
			return nil, err
		}
		m["interactions"] = filtered
	}
	return json.Marshal(m)
}

// filterInteractions keeps only id's entries in each block-keyed interaction
// map, leaving the non-keyed submit and close signals untouched.
func filterInteractions(raw json.RawMessage, id string) (json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	for _, key := range []string{"decisions", "choices", "inputs", "packs", "feedback", "replies", "annotations", "triage"} {
		sub, ok := m[key]
		if !ok {
			continue
		}
		filtered, err := filterMapByKey(sub, id)
		if err != nil {
			return nil, err
		}
		m[key] = filtered
	}
	return json.Marshal(m)
}

// filterMapByKey narrows a JSON object to the single entry under id, preserving
// that entry's value bytes verbatim; a missing id yields an empty object.
func filterMapByKey(raw json.RawMessage, id string) (json.RawMessage, error) {
	var mm map[string]json.RawMessage
	if err := json.Unmarshal(raw, &mm); err != nil {
		return nil, err
	}
	out := map[string]json.RawMessage{}
	if v, ok := mm[id]; ok {
		out[id] = v
	}
	return json.Marshal(out)
}

// newVersionCmd prints the build version — the same string as --version.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the build version",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			_, _ = fmt.Fprintln(c.OutOrStdout(), version.String())
			return nil
		},
	}
}

// newCloseCmd terminally closes this window's artifact.
func newCloseCmd(d cmd.Deps) *cobra.Command {
	var session, cwd, summary string
	c := &cobra.Command{
		Use:   "close",
		Short: "Terminally close this window's artifact",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			ctx := c.Context()
			if err := d.EnsureCurrent(ctx); err != nil {
				return err
			}
			cl, err := client(ctx, d)
			if err != nil {
				return err
			}
			defer func() { _ = cl.CloseSession() }()
			slug, err := cl.Close(ctx, sessionOr(session), mustCwd(cwd), d.ClaudePID(), summary)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(c.OutOrStdout(), "closed: %s\n", slug)
			return nil
		},
	}
	c.Flags().StringVar(&summary, "summary", "", "closing summary recorded on the present.closed event")
	c.Flags().StringVar(&session, "session", "", "Claude session id (defaults to $CLAUDE_CODE_SESSION_ID)")
	c.Flags().StringVar(&cwd, "cwd", "", "working directory (recorded on the request; artifacts are per-window, not resolved by directory)")
	return c
}

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/yasyf/cc-interact/cmd"

	"github.com/yasyf/cc-present/internal/app"
	ccdaemon "github.com/yasyf/cc-present/internal/daemon"
	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/packs"
	"github.com/yasyf/cc-present/internal/slackcards"
	"github.com/yasyf/cc-present/internal/state"
	"github.com/yasyf/cc-present/internal/version"
)

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

func namedOther(session string) bool {
	return session != "" && session != os.Getenv("CLAUDE_CODE_SESSION_ID")
}

func artifactWindow(d cmd.Deps, session string) (string, int) {
	if session != "" {
		return session, 0
	}
	return sessionOr(""), d.ClaudePID()
}

func requireSession(d cmd.Deps, c *cobra.Command) *cobra.Command {
	c.PreRunE = func(c *cobra.Command, _ []string) error {
		session, _ := c.Flags().GetString("session")
		if session == "" {
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
		cwd, _ := c.Flags().GetString("cwd")
		subjectID, _, err := cl.Resolve(ctx, session, mustCwd(cwd), 0)
		if err != nil {
			return err
		}
		if subjectID == "" {
			return ccdaemon.NoArtifact(session)
		}
		return nil
	}
	return c
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

// dryRunReport fills code blocks from their src, links references, and inlines
// any local image against the content-addressed store, then runs the
// whole-document validator, returning the text to print (with any unresolved
// references) and whether the document passed; a false result maps to a
// non-zero exit at the call site.
func dryRunReport(ctx context.Context, dd *doc.Doc, pt doc.PackTypes, root string) (string, bool) {
	if err := fillCode(ctx, dd.Blocks, root); err != nil {
		return err.Error(), false
	}
	unresolved, err := linkRefs(ctx, root, dd.Sources, dd.Blocks, localSnippetStore)
	if err != nil {
		return err.Error(), false
	}
	if err := inlineImages(dd.Blocks, localUploader); err != nil {
		return err.Error(), false
	}
	if err := dd.Validate(pt); err != nil {
		return err.Error(), false
	}
	if hint := unresolvedRefsHint(unresolved); hint != "" {
		return "ok\n" + hint, true
	}
	return "ok", true
}

// liveSources reads the source aliases of the artifact's current document, so
// update-block resolves alias references against the doc it lands in.
func liveSources(ctx context.Context, cl *ccdaemon.Client, sess, scope string, pid int) (map[string]string, error) {
	raw, _, err := cl.Outcomes(ctx, sess, scope, pid)
	if err != nil {
		return nil, err
	}
	var st struct {
		Doc struct {
			Sources map[string]string `json:"sources"`
		} `json:"doc"`
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, fmt.Errorf("decode live doc: %w", err)
	}
	return st.Doc.Sources, nil
}

// blockDoc wraps a lone block in a minimal envelope so update-block --dry-run
// reuses the whole-document validator, checking the block as the top-level block
// update-block inserts it as.
func blockDoc(b doc.Block) *doc.Doc {
	return &doc.Doc{Version: 1, Title: "dry-run", Blocks: []doc.Block{b}}
}

// stripDocs drops the top-level "doc" key from reduced-state JSON and cuts each
// closed round's frozen blocks down to their ids, so outcomes --no-doc emits
// only the human interactions and round partition.
func stripDocs(raw json.RawMessage) (json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	delete(m, "doc")
	rounds, err := mapHistory(m["rounds"], func(r map[string]json.RawMessage) error {
		var blocks []struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(r["blocks"], &blocks); err != nil {
			return err
		}
		ids := make([]string, len(blocks))
		for i, b := range blocks {
			ids[i] = b.ID
		}
		idsJSON, err := json.Marshal(ids)
		if err != nil {
			return err
		}
		delete(r, "blocks")
		r["blockIds"] = idsJSON
		return nil
	})
	if err != nil {
		return nil, err
	}
	m["rounds"] = rounds
	return json.Marshal(m)
}

func mapHistory(raw json.RawMessage, fn func(map[string]json.RawMessage) error) (json.RawMessage, error) {
	var rounds map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rounds); err != nil {
		return nil, err
	}
	var history []map[string]json.RawMessage
	if err := json.Unmarshal(rounds["history"], &history); err != nil {
		return nil, err
	}
	for _, r := range history {
		if err := fn(r); err != nil {
			return nil, err
		}
	}
	historyJSON, err := json.Marshal(history)
	if err != nil {
		return nil, err
	}
	rounds["history"] = historyJSON
	return json.Marshal(rounds)
}

func startMode(fresh, replace bool) ccdaemon.StartMode {
	switch {
	case replace:
		return ccdaemon.StartReplace
	case fresh:
		return ccdaemon.StartNew
	default:
		return ccdaemon.StartResume
	}
}

// newStartCmd creates or resumes this window's artifact and prints its ref, URL,
// and channel state, one per line.
func newStartCmd(d cmd.Deps) *cobra.Command {
	var session, cwd, title, docPath, root string
	var fresh, replace bool
	var reopen []string
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
				if err := fillCode(ctx, dd.Blocks, root); err != nil {
					return err
				}
				_, port, err := cl.Resolve(ctx, sess, scope, pid)
				if err != nil {
					return err
				}
				unresolved, err := linkRefs(ctx, root, dd.Sources, dd.Blocks, httpSnippetStore(port))
				if err != nil {
					return err
				}
				if hint := unresolvedRefsHint(unresolved); hint != "" {
					_, _ = fmt.Fprintln(c.ErrOrStderr(), hint)
				}
				if err := inlineImages(dd.Blocks, httpUploader(port)); err != nil {
					return err
				}
				if docJSON, err = json.Marshal(dd); err != nil {
					return err
				}
			}
			res, err := cl.Start(ctx, sess, scope, pid, startMode(fresh, replace), namedOther(session), reopen, title, docJSON)
			if err != nil {
				return err
			}
			printStart(c, res.SubjectID, res.URL, res.TailnetURLs, res.ChannelState)
			printWarnings(c, res.Warnings)
			return nil
		},
	}
	c.Flags().StringVar(&session, "session", "", "Claude session id (defaults to $CLAUDE_CODE_SESSION_ID)")
	c.Flags().StringVar(&cwd, "cwd", "", "working directory (recorded on the request; artifacts are per-window, not resolved by directory)")
	c.Flags().StringVar(&title, "title", "", "artifact title used for the URL slug when no --doc is given")
	c.Flags().BoolVar(&fresh, "new", false, "start a fresh artifact; refuses while this session has an open one")
	c.Flags().BoolVar(&replace, "replace", false, "close this session's open artifact and start a fresh one")
	c.Flags().StringVar(&docPath, "doc", "", "seed the artifact with a document from a file (- for stdin)")
	c.Flags().StringSliceVar(&reopen, "reopen", nil, reopenUsage)
	c.Flags().StringVar(&root, "root", "", "checkout that code block src paths and markdown code cites resolve under (defaults to the git toplevel of the current directory)")
	return c
}

const reopenUsage = "block id to ask again although the human already answered it (repeatable)"

func printWarnings(c *cobra.Command, warnings []string) {
	for _, w := range warnings {
		_, _ = fmt.Fprintf(c.ErrOrStderr(), "warning: %s\n", w)
	}
}

func withRetired(raw json.RawMessage, retired []string) (json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if retired == nil {
		retired = []string{}
	}
	ids, err := json.Marshal(retired)
	if err != nil {
		return nil, err
	}
	m["retired"] = ids
	return json.Marshal(m)
}

func printStart(c *cobra.Command, subjectID, url string, tailnet []string, channel string) {
	out := c.OutOrStdout()
	_, _ = fmt.Fprintf(out, "session: %s\n", subjectID)
	_, _ = fmt.Fprintf(out, "url: %s\n", url)
	for _, u := range tailnet {
		_, _ = fmt.Fprintf(out, "tailnet: %s\n", u)
	}
	_, _ = fmt.Fprintf(out, "channel: %s\n", channel)
}

// newPushCmd replaces the artifact's document. --dry-run validates the document
// locally and exits non-zero with the validation error on stdout. --round
// current|new declares intent when the push adds a top-level block to an engaged
// round, and --round-title titles a round --round new opens.
func newPushCmd(d cmd.Deps) *cobra.Command {
	var session, cwd, round, roundTitle, root string
	var dryRun bool
	var reopen []string
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
				msg, ok := dryRunReport(c.Context(), dd, packs.Load(c.Context(), cfg.PackDirs, cfg.DisabledPacks), root)
				if hint := longCodeNudge(dd.Blocks); hint != "" {
					_, _ = fmt.Fprintln(c.ErrOrStderr(), hint)
				}
				_, _ = fmt.Fprintln(c.OutOrStdout(), msg)
				if !ok {
					os.Exit(1)
				}
				return nil
			}
			ctx := c.Context()
			if err := fillCode(ctx, dd.Blocks, root); err != nil {
				return err
			}
			if err := d.EnsureCurrent(ctx); err != nil {
				return err
			}
			cl, err := client(ctx, d)
			if err != nil {
				return err
			}
			defer func() { _ = cl.CloseSession() }()
			sess, pid := artifactWindow(d, session)
			scope := mustCwd(cwd)
			subjectID, port, err := cl.Resolve(ctx, sess, scope, pid)
			if err != nil {
				return err
			}
			if subjectID == "" {
				return ccdaemon.NoArtifact(sess)
			}
			unresolved, err := linkRefs(ctx, root, dd.Sources, dd.Blocks, httpSnippetStore(port))
			if err != nil {
				return err
			}
			if err := inlineImages(dd.Blocks, httpUploader(port)); err != nil {
				return err
			}
			docJSON, err := json.Marshal(dd)
			if err != nil {
				return err
			}
			ids := prGroupIDs(dd.Blocks)
			before, err := groupsBefore(ctx, cl, board{sess, scope, pid}, ids)
			if err != nil {
				return err
			}
			res, err := cl.Push(ctx, sess, scope, pid, docJSON, reopen, round, roundTitle)
			if err != nil {
				return err
			}
			printWarnings(c, res.Warnings)
			out := c.OutOrStdout()
			_, _ = fmt.Fprintf(out, "revision: %d\n", res.Revision)
			if res.Round > 0 {
				_, _ = fmt.Fprintf(out, "round: %d\n", res.Round)
			}
			_, _ = fmt.Fprintf(out, "url: %s\n", res.URL)
			for _, u := range res.TailnetURLs {
				_, _ = fmt.Fprintf(out, "tailnet: %s\n", u)
			}
			if hint := longCodeNudge(dd.Blocks); hint != "" {
				_, _ = fmt.Fprintln(c.ErrOrStderr(), hint)
			}
			if hint := unresolvedRefsHint(unresolved); hint != "" {
				_, _ = fmt.Fprintln(c.ErrOrStderr(), hint)
			}
			if len(ids) == 0 {
				return nil
			}
			if err := syncGroups(ctx, cl, board{sess, scope, pid}, out, ids, before); err != nil {
				return fmt.Errorf("pushed revision %d, but its Slack group cards did not all update: %w", res.Revision, err)
			}
			return nil
		},
	}
	c.Flags().StringVar(&session, "session", "", "Claude session id (defaults to $CLAUDE_CODE_SESSION_ID)")
	c.Flags().StringVar(&cwd, "cwd", "", "working directory (recorded on the request; artifacts are per-window, not resolved by directory)")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "validate the document only; print every violation and exit non-zero")
	c.Flags().StringSliceVar(&reopen, "reopen", nil, reopenUsage)
	c.Flags().StringVar(&root, "root", "", "checkout that code block src paths and markdown code cites resolve under (defaults to the git toplevel of the current directory)")
	c.Flags().StringVar(&round, "round", "", `round intent when a new top-level block lands in an engaged round: "current" (add to it) or "new" (close it and open the next)`)
	c.Flags().StringVar(&roundTitle, "round-title", "", "title for the round opened by --round new")
	return c
}

// newUpdateBlockCmd inserts or replaces a single block, optionally after another.
// --round current|new declares intent when the upsert adds a top-level block to
// an engaged round, and --round-title titles a round --round new opens; it prints
// round: N only when the upsert opened round N.
func newUpdateBlockCmd(d cmd.Deps) *cobra.Command {
	var session, cwd, after, round, roundTitle, root string
	var dryRun bool
	var reopen []string
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
				msg, ok := dryRunReport(c.Context(), blockDoc(blk), packs.Load(c.Context(), cfg.PackDirs, cfg.DisabledPacks), root)
				if hint := longCodeNudge([]doc.Block{blk}); hint != "" {
					_, _ = fmt.Fprintln(c.ErrOrStderr(), hint)
				}
				_, _ = fmt.Fprintln(c.OutOrStdout(), msg)
				if !ok {
					os.Exit(1)
				}
				return nil
			}
			ctx := c.Context()
			if err := fillCode(ctx, []doc.Block{blk}, root); err != nil {
				return err
			}
			if err := d.EnsureCurrent(ctx); err != nil {
				return err
			}
			cl, err := client(ctx, d)
			if err != nil {
				return err
			}
			defer func() { _ = cl.CloseSession() }()
			sess, pid := artifactWindow(d, session)
			scope := mustCwd(cwd)
			subjectID, port, err := cl.Resolve(ctx, sess, scope, pid)
			if err != nil {
				return err
			}
			if subjectID == "" {
				return ccdaemon.NoArtifact(sess)
			}
			sources, err := liveSources(ctx, cl, sess, scope, pid)
			if err != nil {
				return err
			}
			unresolved, err := linkRefs(ctx, root, sources, []doc.Block{blk}, httpSnippetStore(port))
			if err != nil {
				return err
			}
			if err := inlineImages([]doc.Block{blk}, httpUploader(port)); err != nil {
				return err
			}
			blockJSON, err := json.Marshal(blk)
			if err != nil {
				return err
			}
			ids := prGroupIDs([]doc.Block{blk})
			before, err := groupsBefore(ctx, cl, board{sess, scope, pid}, ids)
			if err != nil {
				return err
			}
			n, warnings, err := cl.UpsertBlock(ctx, sess, scope, pid, blockJSON, reopen, after, round, roundTitle)
			if err != nil {
				return err
			}
			printWarnings(c, warnings)
			if n > 0 {
				_, _ = fmt.Fprintf(c.OutOrStdout(), "round: %d\n", n)
			}
			if hint := longCodeNudge([]doc.Block{blk}); hint != "" {
				_, _ = fmt.Fprintln(c.ErrOrStderr(), hint)
			}
			if hint := unresolvedRefsHint(unresolved); hint != "" {
				_, _ = fmt.Fprintln(c.ErrOrStderr(), hint)
			}
			if len(ids) == 0 {
				return nil
			}
			if err := syncGroups(ctx, cl, board{sess, scope, pid}, c.OutOrStdout(), ids, before); err != nil {
				return fmt.Errorf("updated %q, but its Slack group cards did not all update: %w", blk.BlockID(), err)
			}
			return nil
		},
	}
	c.Flags().StringVar(&session, "session", "", "Claude session id (defaults to $CLAUDE_CODE_SESSION_ID)")
	c.Flags().StringVar(&cwd, "cwd", "", "working directory (recorded on the request; artifacts are per-window, not resolved by directory)")
	c.Flags().StringVar(&after, "after", "", "insert a new block after a top-level block, or into a card after a child (unknown ids error)")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "validate the single block only; print every violation and exit non-zero")
	c.Flags().StringSliceVar(&reopen, "reopen", nil, reopenUsage)
	c.Flags().StringVar(&root, "root", "", "checkout that code block src paths and markdown code cites resolve under (defaults to the git toplevel of the current directory)")
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
			sess, pid := artifactWindow(d, session)
			return cl.RemoveBlock(ctx, sess, mustCwd(cwd), pid, args[0])
		},
	}
	c.Flags().StringVar(&session, "session", "", "Claude session id (defaults to $CLAUDE_CODE_SESSION_ID)")
	c.Flags().StringVar(&cwd, "cwd", "", "working directory (recorded on the request; artifacts are per-window, not resolved by directory)")
	return c
}

// newReplyCmd appends an agent reply to a block's thread, under the note --re
// names when set.
func newReplyCmd(d cmd.Deps) *cobra.Command {
	var session, cwd, block, re, bodyMd string
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
			sess, pid := artifactWindow(d, session)
			waits, err := cl.Reply(ctx, sess, mustCwd(cwd), pid, block, re, bodyMd)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(c.OutOrStdout(), "replied: %s\n", block)
			if waits == "" {
				return nil
			}
			if _, err := slackcards.CCSlack(ctx, "dm-status", bodyMd, "--about", waits, "--session", sess); err != nil {
				return fmt.Errorf("replied on the board, but not under the Slack card %q: %w", waits, err)
			}
			_, _ = fmt.Fprintf(c.OutOrStdout(), "slack: replied under %s\n", waits)
			return nil
		},
	}
	c.Flags().StringVar(&block, "block", "", "block id to reply under")
	c.Flags().StringVar(&re, "re", "", "feedback id (from the feedback.created event) this reply answers; the board threads the reply under that note")
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
			sess, pid := artifactWindow(d, session)
			n, err := cl.Round(ctx, sess, mustCwd(cwd), pid, title)
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
			sess, pid := artifactWindow(d, session)
			return cl.Revising(ctx, sess, mustCwd(cwd), pid, args, note)
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
			sess, pid := artifactWindow(d, session)
			raw, retired, err := cl.Outcomes(ctx, sess, mustCwd(cwd), pid)
			if err != nil {
				return err
			}
			if raw, err = withRetired(raw, retired); err != nil {
				return err
			}
			if raw, err = nestReplies(raw); err != nil {
				return err
			}
			if block != "" {
				raw, err = filterBlock(raw, block)
				if err != nil {
					return err
				}
			}
			if noDoc {
				raw, err = stripDocs(raw)
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
	c.Flags().BoolVar(&noDoc, "no-doc", false, "omit the reduced document and cut closed rounds to their block ids, printing only the human interactions and rounds")
	c.Flags().StringVar(&block, "block", "", "filter the state to a single block id (a child id filters to its own entries; the doc keeps the enclosing card)")
	c.Flags().StringVar(&session, "session", "", "Claude session id (defaults to $CLAUDE_CODE_SESSION_ID)")
	c.Flags().StringVar(&cwd, "cwd", "", "working directory (recorded on the request; artifacts are per-window, not resolved by directory)")
	return c
}

type threadedNote struct {
	state.Feedback
	Replies []state.Reply `json:"replies,omitempty"`
}

// nestReplies moves each reply that answers a note under that note's
// interactions.feedback entry, so the outcomes read as threads; a reply to the
// block itself stays in interactions.replies.
func nestReplies(raw json.RawMessage) (json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	var in map[string]json.RawMessage
	if err := json.Unmarshal(m["interactions"], &in); err != nil {
		return nil, err
	}
	var feedback map[string][]threadedNote
	if err := json.Unmarshal(in["feedback"], &feedback); err != nil {
		return nil, err
	}
	var replies map[string][]state.Reply
	if err := json.Unmarshal(in["replies"], &replies); err != nil {
		return nil, err
	}
	notes := map[string]*threadedNote{}
	for _, list := range feedback {
		for i := range list {
			notes[list[i].ID] = &list[i]
		}
	}
	for block, list := range replies {
		var loose []state.Reply
		for _, r := range list {
			if note, ok := notes[r.Re]; ok {
				note.Replies = append(note.Replies, r)
				continue
			}
			loose = append(loose, r)
		}
		if loose == nil {
			delete(replies, block)
			continue
		}
		replies[block] = loose
	}
	var err error
	if in["feedback"], err = json.Marshal(feedback); err != nil {
		return nil, err
	}
	if in["replies"], err = json.Marshal(replies); err != nil {
		return nil, err
	}
	if m["interactions"], err = json.Marshal(in); err != nil {
		return nil, err
	}
	return json.Marshal(m)
}

// filterBlock narrows reduced-state JSON to a single block: the document and
// each closed round keep only the enclosing top-level block's subtree, and each
// block-keyed interaction map keeps only the block's own entries. An id no block
// in the current document carries is an error. A child id resolves through
// doc.Locate — its own interactions, its enclosing card's subtree.
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
	rounds, err := mapHistory(m["rounds"], func(r map[string]json.RawMessage) error {
		var blocks []json.RawMessage
		if err := json.Unmarshal(r["blocks"], &blocks); err != nil {
			return err
		}
		keptBlocks := make([]json.RawMessage, 0, 1)
		for _, b := range blocks {
			var head struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(b, &head); err != nil {
				return err
			}
			if head.ID == loc.TopID {
				keptBlocks = append(keptBlocks, b)
			}
		}
		blocksJSON, err := json.Marshal(keptBlocks)
		if err != nil {
			return err
		}
		r["blocks"] = blocksJSON
		return filterKeyed(r, id)
	})
	if err != nil {
		return nil, err
	}
	m["rounds"] = rounds
	return json.Marshal(m)
}

// filterInteractions keeps only id's entries in each block-keyed interaction
// map, leaving the non-keyed submit and close signals untouched.
func filterInteractions(raw json.RawMessage, id string) (json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if err := filterKeyed(m, id); err != nil {
		return nil, err
	}
	return json.Marshal(m)
}

func filterKeyed(m map[string]json.RawMessage, id string) error {
	for _, key := range []string{"decisions", "choices", "inputs", "packs", "feedback", "replies", "annotations", "triage", "viewed"} {
		sub, ok := m[key]
		if !ok {
			continue
		}
		filtered, err := filterMapByKey(sub, id)
		if err != nil {
			return err
		}
		m[key] = filtered
	}
	return nil
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
			sess, pid := artifactWindow(d, session)
			slug, err := cl.Close(ctx, sess, mustCwd(cwd), pid, summary)
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

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
// locally and exits non-zero with the validation error on stdout.
func newPushCmd(d cmd.Deps) *cobra.Command {
	var session, cwd string
	var dryRun bool
	c := &cobra.Command{
		Use:   "push <file|->",
		Short: "Replace the artifact's document",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
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
			rev, err := cl.Push(ctx, sess, scope, pid, docJSON)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(c.OutOrStdout(), "revision: %d\n", rev)
			return nil
		},
	}
	c.Flags().StringVar(&session, "session", "", "Claude session id (defaults to $CLAUDE_CODE_SESSION_ID)")
	c.Flags().StringVar(&cwd, "cwd", "", "working directory (recorded on the request; artifacts are per-window, not resolved by directory)")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "validate the document only; print every violation and exit non-zero")
	return c
}

// newUpdateBlockCmd inserts or replaces a single block, optionally after another.
func newUpdateBlockCmd(d cmd.Deps) *cobra.Command {
	var session, cwd, after string
	var dryRun bool
	c := &cobra.Command{
		Use:   "update-block <file|->",
		Short: "Insert or replace a single block",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
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
			return cl.UpsertBlock(ctx, sess, scope, pid, blockJSON, after)
		},
	}
	c.Flags().StringVar(&session, "session", "", "Claude session id (defaults to $CLAUDE_CODE_SESSION_ID)")
	c.Flags().StringVar(&cwd, "cwd", "", "working directory (recorded on the request; artifacts are per-window, not resolved by directory)")
	c.Flags().StringVar(&after, "after", "", "insert a new block after this block id (append when absent or unknown)")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "validate the single block only; print every violation and exit non-zero")
	return c
}

// newRemoveBlockCmd removes a top-level block by id.
func newRemoveBlockCmd(d cmd.Deps) *cobra.Command {
	var session, cwd string
	c := &cobra.Command{
		Use:   "remove-block <id>",
		Short: "Remove a top-level block by id",
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

// newRevisingCmd declares the top-level block ids the agent is revising, with an
// optional note. No ids and no note abandons the announcement; no ids with a note
// is the doc-level drafting state.
func newRevisingCmd(d cmd.Deps) *cobra.Command {
	var session, cwd, note string
	c := &cobra.Command{
		Use:   "revising [blockId...]",
		Short: "Declare the block ids the agent is revising, with an optional note",
		Args:  cobra.ArbitraryArgs,
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
			return cl.Revising(ctx, sessionOr(session), mustCwd(cwd), d.ClaudePID(), args, note)
		},
	}
	c.Flags().StringVar(&note, "note", "", "note shown with the revising announcement")
	c.Flags().StringVar(&session, "session", "", "Claude session id (defaults to $CLAUDE_CODE_SESSION_ID)")
	c.Flags().StringVar(&cwd, "cwd", "", "working directory (recorded on the request; artifacts are per-window, not resolved by directory)")
	return c
}

// newOutcomesCmd prints the artifact's reduced state as JSON — the post-submit
// drain.
func newOutcomesCmd(d cmd.Deps) *cobra.Command {
	var session, cwd string
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
	c.Flags().StringVar(&session, "session", "", "Claude session id (defaults to $CLAUDE_CODE_SESSION_ID)")
	c.Flags().StringVar(&cwd, "cwd", "", "working directory (recorded on the request; artifacts are per-window, not resolved by directory)")
	return c
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

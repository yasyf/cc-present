package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/yasyf/cc-interact/cmd"

	ccdaemon "github.com/yasyf/cc-present/internal/daemon"
	"github.com/yasyf/cc-present/internal/doc"
)

const noArtifact = "no cc-present artifact for this scope; run `cc-present start` first"

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

func client(d cmd.Deps) *ccdaemon.Client { return ccdaemon.NewClient(d.NewClient()) }

// newStartCmd creates or resumes this scope's artifact and prints its ref, URL,
// and channel state, one per line.
func newStartCmd(d cmd.Deps) *cobra.Command {
	var session, cwd, title, docPath string
	var fresh bool
	c := &cobra.Command{
		Use:   "start",
		Short: "Create or resume this scope's cc-present artifact and print its URL",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			ctx := c.Context()
			if err := d.EnsureCurrent(ctx); err != nil {
				return err
			}
			cl := client(d)
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
	c.Flags().StringVar(&cwd, "cwd", "", "working directory / scope (defaults to the current directory)")
	c.Flags().StringVar(&title, "title", "", "artifact title used for the URL slug when no --doc is given")
	c.Flags().BoolVar(&fresh, "new", false, "force a fresh artifact, detaching any existing one for this scope")
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
				return fmt.Errorf("decode doc: %w", err)
			}
			if dryRun {
				if err := inlineImages(dd.Blocks, localUploader); err != nil {
					_, _ = fmt.Fprintln(c.OutOrStdout(), err)
					os.Exit(1)
				}
				if err := dd.Validate(); err != nil {
					_, _ = fmt.Fprintln(c.OutOrStdout(), err)
					os.Exit(1)
				}
				_, _ = fmt.Fprintln(c.OutOrStdout(), "ok")
				return nil
			}
			ctx := c.Context()
			if err := d.EnsureCurrent(ctx); err != nil {
				return err
			}
			cl := client(d)
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
	c.Flags().StringVar(&cwd, "cwd", "", "working directory / scope (defaults to the current directory)")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "validate the document only; print the first error and exit non-zero")
	return c
}

// newUpdateBlockCmd inserts or replaces a single block, optionally after another.
func newUpdateBlockCmd(d cmd.Deps) *cobra.Command {
	var session, cwd, after string
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
				return err
			}
			ctx := c.Context()
			if err := d.EnsureCurrent(ctx); err != nil {
				return err
			}
			cl := client(d)
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
	c.Flags().StringVar(&cwd, "cwd", "", "working directory / scope (defaults to the current directory)")
	c.Flags().StringVar(&after, "after", "", "insert a new block after this block id (append when absent or unknown)")
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
			return client(d).RemoveBlock(ctx, sessionOr(session), mustCwd(cwd), d.ClaudePID(), args[0])
		},
	}
	c.Flags().StringVar(&session, "session", "", "Claude session id (defaults to $CLAUDE_CODE_SESSION_ID)")
	c.Flags().StringVar(&cwd, "cwd", "", "working directory / scope (defaults to the current directory)")
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
			return client(d).Reply(ctx, sessionOr(session), mustCwd(cwd), d.ClaudePID(), block, bodyMd)
		},
	}
	c.Flags().StringVar(&block, "block", "", "block id to reply under")
	c.Flags().StringVar(&bodyMd, "body", "", "reply markdown")
	c.Flags().StringVar(&session, "session", "", "Claude session id (defaults to $CLAUDE_CODE_SESSION_ID)")
	c.Flags().StringVar(&cwd, "cwd", "", "working directory / scope (defaults to the current directory)")
	return c
}

// newOutcomesCmd prints the artifact's reduced state as JSON — the post-submit
// drain.
func newOutcomesCmd(d cmd.Deps) *cobra.Command {
	var session, cwd string
	c := &cobra.Command{
		Use:   "outcomes",
		Short: "Print the artifact's reduced document and human interactions as JSON",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			ctx := c.Context()
			if err := d.EnsureCurrent(ctx); err != nil {
				return err
			}
			raw, err := client(d).Outcomes(ctx, sessionOr(session), mustCwd(cwd), d.ClaudePID())
			if err != nil {
				return err
			}
			var buf bytes.Buffer
			if err := json.Indent(&buf, raw, "", "  "); err != nil {
				return err
			}
			_, _ = fmt.Fprintln(c.OutOrStdout(), buf.String())
			return nil
		},
	}
	c.Flags().StringVar(&session, "session", "", "Claude session id (defaults to $CLAUDE_CODE_SESSION_ID)")
	c.Flags().StringVar(&cwd, "cwd", "", "working directory / scope (defaults to the current directory)")
	return c
}

// newCloseCmd terminally closes this scope's artifact.
func newCloseCmd(d cmd.Deps) *cobra.Command {
	var session, cwd, summary string
	c := &cobra.Command{
		Use:   "close",
		Short: "Terminally close this scope's artifact",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			ctx := c.Context()
			if err := d.EnsureCurrent(ctx); err != nil {
				return err
			}
			slug, err := client(d).Close(ctx, sessionOr(session), mustCwd(cwd), d.ClaudePID(), summary)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(c.OutOrStdout(), "closed: %s\n", slug)
			return nil
		},
	}
	c.Flags().StringVar(&summary, "summary", "", "closing summary recorded on the present.closed event")
	c.Flags().StringVar(&session, "session", "", "Claude session id (defaults to $CLAUDE_CODE_SESSION_ID)")
	c.Flags().StringVar(&cwd, "cwd", "", "working directory / scope (defaults to the current directory)")
	return c
}

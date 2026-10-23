package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/yasyf/cc-interact/cmd"

	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/slackcards"
)

func newAskCmd(d cmd.Deps) *cobra.Command {
	var session, cwd string
	c := &cobra.Command{
		Use:   "ask <file|->",
		Short: "Ask one card's question on the board and in Slack, synced both ways",
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
			card, ok := blk.(*doc.Card)
			if !ok {
				return fmt.Errorf("ask takes one card, not a %s", blk.BlockType())
			}
			ch, err := slackcards.Check(card)
			if err != nil {
				return err
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
			sc, err := json.Marshal(slackcards.ToSlack(card, ch))
			if err != nil {
				return err
			}
			out, err := slackcards.CCSlack(ctx, "dm-status", "--card", string(sc), "--mirrored", "--session", sess)
			if err != nil {
				return err
			}
			var posted struct {
				Permalink string `json:"permalink"`
			}
			if err := json.Unmarshal(out, &posted); err != nil || posted.Permalink == "" {
				return fmt.Errorf("cc-slack posted card %q but returned no permalink: %s", card.ID, out)
			}
			card.Slack = &doc.CardSlack{URL: posted.Permalink}
			blockJSON, err := json.Marshal(card)
			if err != nil {
				return err
			}
			res, err := cl.SlackCard(ctx, sess, mustCwd(cwd), pid, blockJSON)
			if err != nil {
				return fmt.Errorf("card %q is posted in Slack at %s but not on the board: %w", card.ID, posted.Permalink, err)
			}
			w := c.OutOrStdout()
			_, _ = fmt.Fprintf(w, "url: %s\n", res.URL)
			for _, u := range res.TailnetURLs {
				_, _ = fmt.Fprintf(w, "tailnet: %s\n", u)
			}
			_, _ = fmt.Fprintf(w, "slack: %s\n", posted.Permalink)
			return nil
		},
	}
	c.Flags().StringVar(&session, "session", "", "Claude session id (defaults to $CLAUDE_CODE_SESSION_ID)")
	c.Flags().StringVar(&cwd, "cwd", "", "working directory (recorded on the request; artifacts are per-window, not resolved by directory)")
	return c
}

func newSlackSyncCmd(d cmd.Deps) *cobra.Command {
	var session, cwd string
	c := &cobra.Command{
		Use:    "slack-sync posted|answered",
		Short:  "Mirror a cc-slack ask card's post or answer onto the session's board",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			if args[0] != "posted" && args[0] != "answered" {
				return fmt.Errorf("slack-sync takes posted or answered, not %q", args[0])
			}
			raw, err := io.ReadAll(c.InOrStdin())
			if err != nil {
				return err
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
			if args[0] == "answered" {
				return cl.SlackAnswer(ctx, sess, mustCwd(cwd), pid, raw)
			}
			var p slackcards.Posted
			if err := json.Unmarshal(raw, &p); err != nil {
				return fmt.Errorf("decode posted card: %w", err)
			}
			blockJSON, err := slackcards.FromSlack(p)
			if err != nil {
				return err
			}
			_, err = cl.SlackCard(ctx, sess, mustCwd(cwd), pid, blockJSON)
			return err
		},
	}
	c.Flags().StringVar(&session, "session", "", "Claude session id (defaults to $CLAUDE_CODE_SESSION_ID)")
	c.Flags().StringVar(&cwd, "cwd", "", "working directory (recorded on the request; artifacts are per-window, not resolved by directory)")
	return c
}

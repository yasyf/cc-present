package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"slices"

	"github.com/spf13/cobra"
	"github.com/yasyf/cc-interact/cmd"

	ccdaemon "github.com/yasyf/cc-present/internal/daemon"
	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/slackcards"
	"github.com/yasyf/cc-present/internal/state"
)

func newAskCmd(d cmd.Deps) *cobra.Command {
	var session, cwd, thread string
	c := &cobra.Command{
		Use:   "ask <file|->",
		Short: "Put one card on the board, then send its question to Slack, synced both ways",
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
			ts, err := slackThread(thread)
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
			res, err := mirror(ctx, cl, board{sess, mustCwd(cwd), pid}, card, ch, ts)
			if err != nil {
				return err
			}
			printBoard(c.OutOrStdout(), res)
			_, _ = fmt.Fprintf(c.OutOrStdout(), "slack: %s\n", card.Slack.URL)
			return nil
		},
	}
	c.Flags().StringVar(&thread, "slack", "", "ts or permalink of the DM thread to post in (defaults to this session's status thread)")
	c.Flags().StringVar(&session, "session", "", "Claude session id (defaults to $CLAUDE_CODE_SESSION_ID)")
	c.Flags().StringVar(&cwd, "cwd", "", "working directory (recorded on the request; artifacts are per-window, not resolved by directory)")
	return c
}

func newSendCmd(d cmd.Deps) *cobra.Command {
	var session, cwd, thread string
	var blocks []string
	c := &cobra.Command{
		Use:   "send",
		Short: "Send the board's open question cards to Slack in one call, synced both ways",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			ts, err := slackThread(thread)
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
			b := board{sess, mustCwd(cwd), pid}
			raw, err := cl.Outcomes(ctx, b.session, b.scope, b.pid)
			if err != nil {
				return err
			}
			var st struct {
				Doc          *doc.Doc `json:"doc"`
				Interactions struct {
					Choices map[string]state.Selection `json:"choices"`
				} `json:"interactions"`
			}
			if err := json.Unmarshal(raw, &st); err != nil {
				return fmt.Errorf("decode the board: %w", err)
			}
			questions, skipped, err := openQuestions(st.Doc, st.Interactions.Choices, blocks)
			if err != nil {
				return err
			}
			for _, s := range skipped {
				_, _ = fmt.Fprintf(c.ErrOrStderr(), "warning: %s\n", s)
			}
			if len(questions) == 0 {
				_, _ = fmt.Fprintln(c.OutOrStdout(), "no open question on the board")
				return nil
			}
			var res ccdaemon.StartResult
			var errs []error
			for _, q := range questions {
				r, err := mirror(ctx, cl, b, q.card, q.choice, ts)
				if err != nil {
					errs = append(errs, err)
					continue
				}
				res = r
				_, _ = fmt.Fprintf(c.OutOrStdout(), "slack: %s %s\n", q.card.ID, q.card.Slack.URL)
			}
			if res.URL != "" {
				printBoard(c.OutOrStdout(), res)
			}
			return errors.Join(errs...)
		},
	}
	c.Flags().StringVar(&thread, "slack", "", "ts or permalink of the DM thread to post in (defaults to this session's status thread)")
	c.Flags().StringSliceVar(&blocks, "blocks", nil, "ids of the cards to send (defaults to every open question card)")
	c.Flags().StringVar(&session, "session", "", "Claude session id (defaults to $CLAUDE_CODE_SESSION_ID)")
	c.Flags().StringVar(&cwd, "cwd", "", "working directory (recorded on the request; artifacts are per-window, not resolved by directory)")
	return c
}

type board struct {
	session string
	scope   string
	pid     int
}

type question struct {
	card   *doc.Card
	choice *doc.Choice
}

func openQuestions(d *doc.Doc, choices map[string]state.Selection, named []string) ([]question, []string, error) {
	var picked []question
	var skipped []string
	var errs []error
	found := map[string]bool{}
	for _, blk := range d.Blocks {
		card, ok := blk.(*doc.Card)
		if !ok || (len(named) > 0 && !slices.Contains(named, card.ID)) {
			continue
		}
		found[card.ID] = true
		ch, ok := slackcards.Question(card)
		switch {
		case !ok && len(named) == 0:
			continue
		case card.Status == slackcards.Resolved:
			skipped = append(skipped, fmt.Sprintf("card %q is already answered", card.ID))
			continue
		case ok && answered(choices[ch.ID]):
			skipped = append(skipped, fmt.Sprintf("card %q is already answered on the board", card.ID))
			continue
		}
		ch, err := slackcards.Check(card)
		switch {
		case err != nil && len(named) > 0:
			errs = append(errs, err)
		case err != nil:
			skipped = append(skipped, err.Error())
		default:
			picked = append(picked, question{card, ch})
		}
	}
	for _, id := range named {
		if !found[id] {
			errs = append(errs, fmt.Errorf("no card %q at the top level of the board", id))
		}
	}
	return picked, skipped, errors.Join(errs...)
}

func answered(sel state.Selection) bool {
	return len(sel.OptionIDs) > 0 || sel.Other != ""
}

func mirror(ctx context.Context, cl *ccdaemon.Client, b board, card *doc.Card, ch *doc.Choice, ts string) (ccdaemon.StartResult, error) {
	sc, err := json.Marshal(slackcards.ToSlack(card, ch))
	if err != nil {
		return ccdaemon.StartResult{}, err
	}
	args := []string{"dm-status", "--card", string(sc), "--mirrored", "--session", b.session}
	if ts != "" {
		args = append(args, "--thread", ts)
	}
	out, err := slackcards.CCSlack(ctx, args...)
	if err != nil {
		return ccdaemon.StartResult{}, fmt.Errorf("card %q: %w", card.ID, err)
	}
	var posted struct {
		Permalink string `json:"permalink"`
	}
	if err := json.Unmarshal(out, &posted); err != nil || posted.Permalink == "" {
		return ccdaemon.StartResult{}, fmt.Errorf("cc-slack posted card %q but returned no permalink: %s", card.ID, out)
	}
	card.Slack = &doc.CardSlack{URL: posted.Permalink}
	blockJSON, err := json.Marshal(card)
	if err != nil {
		return ccdaemon.StartResult{}, err
	}
	res, err := cl.SlackCard(ctx, b.session, b.scope, b.pid, blockJSON)
	if err != nil {
		return ccdaemon.StartResult{}, fmt.Errorf("card %q is posted in Slack at %s but not on the board: %w", card.ID, posted.Permalink, err)
	}
	return res, nil
}

func printBoard(w io.Writer, res ccdaemon.StartResult) {
	_, _ = fmt.Fprintf(w, "url: %s\n", res.URL)
	for _, u := range res.TailnetURLs {
		_, _ = fmt.Fprintf(w, "tailnet: %s\n", u)
	}
}

var (
	slackTS        = regexp.MustCompile(`^\d{10}\.\d{6}$`)
	slackPermalink = regexp.MustCompile(`/p(\d{10})(\d{6})$`)
)

func slackThread(s string) (string, error) {
	if s == "" || slackTS.MatchString(s) {
		return s, nil
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("--slack %q is neither a Slack ts nor a message permalink", s)
	}
	if ts := u.Query().Get("thread_ts"); slackTS.MatchString(ts) {
		return ts, nil
	}
	m := slackPermalink.FindStringSubmatch(u.Path)
	if m == nil {
		return "", fmt.Errorf("--slack %q is neither a Slack ts nor a message permalink", s)
	}
	return m[1] + "." + m[2], nil
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

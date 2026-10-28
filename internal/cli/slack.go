package cli

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"slices"
	"strings"

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
			res, _, err := mirror(ctx, cl, board{sess, mustCwd(cwd), pid}, card, ch, ts)
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
			st, err := readBoard(ctx, cl, b)
			if err != nil {
				return err
			}
			questions, skipped, err := openQuestions(st, blocks)
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
				r, posted, err := mirror(ctx, cl, b, q.card, q.choice, ts)
				if err != nil {
					errs = append(errs, err)
					_, _ = fmt.Fprintf(c.OutOrStdout(), "failed: %s: %v\n", q.card.ID, err)
					continue
				}
				res = r
				_, _ = fmt.Fprintf(c.OutOrStdout(), "sent: %s ts=%s %s\n", q.card.ID, posted, q.card.Slack.URL)
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

type sendBoard struct {
	Doc          *doc.Doc `json:"doc"`
	Interactions struct {
		Choices map[string]state.Selection           `json:"choices"`
		Triage  map[string]map[string]state.Decision `json:"triage"`
		Closed  state.Closed                         `json:"closed"`
	} `json:"interactions"`
	Rounds struct {
		Current     int            `json:"current"`
		BlockRounds map[string]int `json:"blockRounds"`
	} `json:"rounds"`
}

func readBoard(ctx context.Context, cl *ccdaemon.Client, b board) (*sendBoard, error) {
	raw, _, err := cl.Outcomes(ctx, b.session, b.scope, b.pid, ccdaemon.OutcomeLive)
	if err != nil {
		return nil, err
	}
	var st sendBoard
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, fmt.Errorf("decode the board: %w", err)
	}
	return &st, nil
}

func groupsBefore(ctx context.Context, cl *ccdaemon.Client, b board) ([]*doc.PRGroups, error) {
	st, err := readBoard(ctx, cl, b)
	if err != nil {
		return nil, err
	}
	return boundGroups(st), nil
}

func boundGroups(st *sendBoard) []*doc.PRGroups {
	if st.Doc == nil {
		return nil
	}
	var bound []*doc.PRGroups
	for _, g := range prGroups(st.Doc.Blocks) {
		if g.Thread() != "" {
			bound = append(bound, g)
		}
	}
	return bound
}

func syncGroups(ctx context.Context, cl *ccdaemon.Client, b board, w io.Writer, ids []string, before []*doc.PRGroups) error {
	st, err := readBoard(ctx, cl, b)
	if err != nil {
		return err
	}
	after := boundGroups(st)
	var errs []error
	post := func(g *doc.PRGroups, cards []slackcards.StatusCard) {
		for _, card := range cards {
			permalink, err := slackcards.PostGroupCard(ctx, b.session, g.Thread(), card)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			_, _ = fmt.Fprintf(w, "slack: %s %s\n", strings.TrimPrefix(card.Group, g.ID+":"), cmp.Or(permalink, "nothing open, never posted"))
		}
	}
	for _, prev := range before {
		if !slices.ContainsFunc(after, func(g *doc.PRGroups) bool { return g.ID == prev.ID }) {
			gone := *prev
			gone.Groups = nil
			post(prev, slackcards.GroupCards(prev, &gone, st.Interactions.Triage[prev.ID]))
		}
	}
	for _, g := range after {
		if !slices.Contains(ids, g.ID) {
			continue
		}
		i := slices.IndexFunc(before, func(p *doc.PRGroups) bool { return p.ID == g.ID })
		var prev *doc.PRGroups
		if i >= 0 {
			prev = before[i]
		}
		post(g, slackcards.GroupCards(prev, g, st.Interactions.Triage[g.ID]))
	}
	return errors.Join(errs...)
}

func prGroups(blocks []doc.Block) []*doc.PRGroups {
	var found []*doc.PRGroups
	for _, b := range blocks {
		doc.Walk(b, func(b doc.Block) {
			if g, ok := b.(*doc.PRGroups); ok {
				found = append(found, g)
			}
		})
	}
	return found
}

func prGroupIDs(blocks []doc.Block) []string {
	groups := prGroups(blocks)
	ids := make([]string, 0, len(groups))
	for _, g := range groups {
		ids = append(ids, g.ID)
	}
	return ids
}

type question struct {
	card   *doc.Card
	choice *doc.Choice
}

func openQuestions(b *sendBoard, named []string) ([]question, []string, error) {
	if b.Interactions.Closed.Value {
		return nil, nil, errors.New("the board is closed; start a new one before sending its questions")
	}
	var picked []question
	var skipped []string
	var errs []error
	found := map[string]bool{}
	for _, blk := range b.Doc.Blocks {
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
		case ok && answered(b.Interactions.Choices[ch.ID]):
			skipped = append(skipped, fmt.Sprintf("card %q is already answered on the board", card.ID))
			continue
		case frozen(b, card.ID):
			skipped = append(skipped, fmt.Sprintf("card %q sits in a closed round; update-block it to ask it again", card.ID))
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

func frozen(b *sendBoard, id string) bool {
	r, ok := b.Rounds.BlockRounds[id]
	return ok && r < b.Rounds.Current
}

func answered(sel state.Selection) bool {
	return len(sel.OptionIDs) > 0 || sel.Other != ""
}

func mirror(ctx context.Context, cl *ccdaemon.Client, b board, card *doc.Card, ch *doc.Choice, ts string) (ccdaemon.StartResult, string, error) {
	unposted, err := json.Marshal(card)
	if err != nil {
		return ccdaemon.StartResult{}, "", err
	}
	if err := cl.SlackCardCheck(ctx, b.session, b.scope, b.pid, unposted); err != nil {
		return ccdaemon.StartResult{}, "", fmt.Errorf("card %q, not posted to Slack: %w", card.ID, err)
	}
	sc, err := json.Marshal(slackcards.ToSlack(card, ch))
	if err != nil {
		return ccdaemon.StartResult{}, "", err
	}
	args := []string{"dm-status", "--card", string(sc), "--mirrored", "--session", b.session}
	if ts != "" {
		args = append(args, "--thread", ts)
	}
	out, err := slackcards.CCSlack(ctx, args...)
	if err != nil {
		return ccdaemon.StartResult{}, "", fmt.Errorf("card %q: %w", card.ID, err)
	}
	var posted struct {
		Permalink string `json:"permalink"`
		TS        string `json:"ts"`
	}
	if err := json.Unmarshal(out, &posted); err != nil || posted.Permalink == "" {
		return ccdaemon.StartResult{}, "", fmt.Errorf("cc-slack posted card %q but returned no permalink: %s", card.ID, out)
	}
	card.Slack = &doc.CardSlack{URL: posted.Permalink}
	blockJSON, err := json.Marshal(card)
	if err != nil {
		return ccdaemon.StartResult{}, "", err
	}
	res, err := cl.SlackCard(ctx, b.session, b.scope, b.pid, blockJSON)
	if err != nil {
		return ccdaemon.StartResult{}, "", fmt.Errorf("card %q is posted in Slack at %s but not on the board: %w", card.ID, posted.Permalink, err)
	}
	return res, posted.TS, nil
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
		Use:    "slack-sync posted|answered|decided",
		Short:  "Mirror a cc-slack ask card's post or answer, or a row tap on a group card, onto the session's board",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			if !slices.Contains([]string{"posted", "answered", "decided"}, args[0]) {
				return fmt.Errorf("slack-sync takes posted, answered, or decided, not %q", args[0])
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
			switch args[0] {
			case "answered":
				return cl.SlackAnswer(ctx, sess, mustCwd(cwd), pid, raw)
			case "decided":
				return cl.SlackDecided(ctx, sess, mustCwd(cwd), pid, raw)
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

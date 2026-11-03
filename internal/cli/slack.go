package cli

import (
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yasyf/cc-interact/cmd"

	"github.com/yasyf/cc-present/internal/assets"
	ccdaemon "github.com/yasyf/cc-present/internal/daemon"
	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/slackcards"
	"github.com/yasyf/cc-present/internal/state"
)

func newAskCmd(d cmd.Deps) *cobra.Command {
	var session, cwd, thread string
	var w ccdaemon.Writer
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
			scope := mustCwd(cwd)
			_, port, err := cl.Resolve(ctx, sess, scope, pid)
			if err != nil {
				return err
			}
			if err := inlineImages([]doc.Block{card}, httpUploader(port)); err != nil {
				return err
			}
			res, _, err := mirror(ctx, cl, board{sess, scope, pid, w}, w, card, ch, ts, c.ErrOrStderr())
			if err != nil {
				return err
			}
			printBoard(c.OutOrStdout(), res)
			_, _ = fmt.Fprintf(c.OutOrStdout(), "slack: %s\n", card.Slack.URL)
			return nil
		},
	}
	c.Flags().StringVar(&thread, "slack", "", "ts or permalink of a DM message to keep watched for replies; the cards post at the DM root")
	c.Flags().StringVar(&session, "session", "", "Claude session id (defaults to $CLAUDE_CODE_SESSION_ID)")
	c.Flags().StringVar(&cwd, "cwd", "", "working directory (recorded on the request; artifacts are per-window, not resolved by directory)")
	writerFlags(c, &w)
	return c
}

func newSendCmd(d cmd.Deps) *cobra.Command {
	var session, cwd, thread, lane string
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
			b := board{sess, mustCwd(cwd), pid, ccdaemon.Writer{Lane: lane}}
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
			res, err := sendQuestions(ctx, cl, b, st.rev, questions, ts, c.OutOrStdout(), c.ErrOrStderr())
			if res.URL != "" {
				printBoard(c.OutOrStdout(), res)
			}
			return err
		},
	}
	c.Flags().StringVar(&thread, "slack", "", "ts or permalink of a DM message to keep watched for replies; the cards post at the DM root")
	c.Flags().StringSliceVar(&blocks, "blocks", nil, "ids of the cards to send (defaults to every open question card)")
	c.Flags().StringVar(&lane, "lane", os.Getenv("CC_PRESENT_LANE"), "lane sending the cards (defaults to $CC_PRESENT_LANE); sending records each card's Slack post without changing who owns it")
	c.Flags().StringVar(&session, "session", "", "Claude session id (defaults to $CLAUDE_CODE_SESSION_ID)")
	c.Flags().StringVar(&cwd, "cwd", "", "working directory (recorded on the request; artifacts are per-window, not resolved by directory)")
	return c
}

type board struct {
	session string
	scope   string
	pid     int
	writer  ccdaemon.Writer
}

type sendBoard struct {
	Doc          *doc.Doc `json:"doc"`
	Interactions struct {
		Decisions map[string]state.Decision            `json:"decisions"`
		Choices   map[string]state.Selection           `json:"choices"`
		Inputs    map[string]state.InputValue          `json:"inputs"`
		Triage    map[string]map[string]state.Decision `json:"triage"`
		Closed    state.Closed                         `json:"closed"`
	} `json:"interactions"`
	Rounds struct {
		Current     int            `json:"current"`
		BlockRounds map[string]int `json:"blockRounds"`
	} `json:"rounds"`
	rev int64
}

func readBoard(ctx context.Context, cl *ccdaemon.Client, b board) (*sendBoard, error) {
	read, err := cl.OutcomesSince(ctx, b.session, b.scope, b.pid, ccdaemon.OutcomeLive, "")
	if err != nil {
		return nil, err
	}
	st := sendBoard{rev: read.Rev}
	if err := json.Unmarshal(read.State, &st); err != nil {
		return nil, fmt.Errorf("decode the board: %w", err)
	}
	return &st, nil
}

// syncSlack carries a board write to Slack: the group cards it touched, and the
// simple questions it asked. A board's session is its Slack thread, so a board
// with no session asks nothing there.
func syncSlack(ctx context.Context, cl *ccdaemon.Client, b board, w, warn io.Writer, before *sendBoard, blocks []doc.Block) error {
	ids := prGroupIDs(blocks)
	bound := boundGroups(before)
	mirrors := b.session != "" && slackcards.Installed()
	if len(ids) == 0 && len(bound) == 0 && !mirrors {
		return nil
	}
	after, err := readBoard(ctx, cl, b)
	if err != nil {
		return err
	}
	errs := []error{syncGroups(ctx, b, w, after, ids, bound)}
	if mirrors {
		questions, nudges := writtenQuestions(before, after, blocks)
		for _, nudge := range nudges {
			_, _ = fmt.Fprintf(warn, "warning: %s\n", nudge)
		}
		_, err := sendQuestions(ctx, cl, b, after.rev, questions, "", w, warn)
		errs = append(errs, err)
	}
	return errors.Join(errs...)
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

func syncGroups(ctx context.Context, b board, w io.Writer, st *sendBoard, ids []string, before []*doc.PRGroups) error {
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
	card    *doc.Card
	control doc.Block
}

// writtenQuestions is what a write of blocks puts in front of Slack: each card it
// touched that asks one open question, unless its post already says the same,
// and a line for each it left on the board without the author saying so.
func writtenQuestions(before, after *sendBoard, blocks []doc.Block) ([]question, []string) {
	var picked []question
	var nudges []string
	seen := map[string]bool{}
	for _, blk := range blocks {
		loc, ok := doc.Locate(after.Doc, blk.BlockID())
		if !ok || seen[loc.TopID] {
			continue
		}
		seen[loc.TopID] = true
		top := after.Doc.Blocks[loc.TopIndex]
		if nudge := slackcards.Nudge(top); nudge != "" {
			nudges = append(nudges, nudge)
			continue
		}
		card, ok := top.(*doc.Card)
		if !ok || !card.Mirrors() {
			continue
		}
		q, ok := slackcards.Question(card)
		if ok && !settled(after, card, q) && !postedTheSame(before, card) {
			picked = append(picked, question{card, q})
		}
	}
	return picked, nudges
}

func postedTheSame(before *sendBoard, card *doc.Card) bool {
	if before.Doc == nil || card.Slack == nil {
		return false
	}
	loc, ok := doc.Locate(before.Doc, card.ID)
	if !ok {
		return false
	}
	prev, ok := loc.Block.(*doc.Card)
	return ok && prev.Slack != nil && slackcards.Same(prev, card)
}

func settled(b *sendBoard, card *doc.Card, q doc.Block) bool {
	return card.Status == slackcards.Resolved || answeredOnBoard(b, q) || frozen(b, card.ID)
}

// sendQuestions posts each question card, read off the board at rev, to Slack
// and records each post on the board.
func sendQuestions(ctx context.Context, cl *ccdaemon.Client, b board, rev int64, questions []question, ts string, w, warn io.Writer) (ccdaemon.StartResult, error) {
	var res ccdaemon.StartResult
	var errs []error
	for _, q := range questions {
		r, posted, err := mirror(ctx, cl, b, ccdaemon.Writer{Lane: b.writer.Lane, Rev: rev}, q.card, q.control, ts, warn)
		if err != nil {
			errs = append(errs, err)
			_, _ = fmt.Fprintf(w, "failed: %s: %v\n", q.card.ID, err)
			continue
		}
		res = r
		_, _ = fmt.Fprintf(w, "sent: %s ts=%s %s\n", q.card.ID, posted, q.card.Slack.URL)
	}
	return res, errors.Join(errs...)
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
		q, ok := slackcards.Question(card)
		switch {
		case len(named) == 0 && (!card.Mirrors() || !slackcards.Asks(card)):
			continue
		case card.Status == slackcards.Resolved:
			skipped = append(skipped, fmt.Sprintf("card %q is already answered", card.ID))
			continue
		case ok && answeredOnBoard(b, q):
			skipped = append(skipped, fmt.Sprintf("card %q is already answered on the board", card.ID))
			continue
		case frozen(b, card.ID):
			skipped = append(skipped, fmt.Sprintf("card %q sits in a closed round; update-block it to ask it again", card.ID))
			continue
		}
		q, err := slackcards.Check(card)
		switch {
		case err != nil && len(named) > 0:
			errs = append(errs, err)
		case err != nil:
			skipped = append(skipped, err.Error())
		default:
			picked = append(picked, question{card, q})
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

func answeredOnBoard(b *sendBoard, q doc.Block) bool {
	switch q.(type) {
	case *doc.Choice:
		sel := b.Interactions.Choices[q.BlockID()]
		return len(sel.OptionIDs) > 0 || sel.Other != ""
	case *doc.Approval:
		verdict := b.Interactions.Decisions[q.BlockID()].Verdict
		return verdict != "" && verdict != "cleared"
	case *doc.Input:
		return b.Interactions.Inputs[q.BlockID()].Text != ""
	}
	return false
}

func mirror(ctx context.Context, cl *ccdaemon.Client, b board, wr ccdaemon.Writer, card *doc.Card, q doc.Block, ts string, warn io.Writer) (ccdaemon.StartResult, string, error) {
	unposted, err := json.Marshal(card)
	if err != nil {
		return ccdaemon.StartResult{}, "", err
	}
	if err := cl.SlackCardCheck(ctx, b.session, b.scope, b.pid, unposted); err != nil {
		return ccdaemon.StartResult{}, "", fmt.Errorf("card %q, not posted to Slack: %w", card.ID, err)
	}
	_, port, err := cl.Resolve(ctx, b.session, b.scope, b.pid)
	if err != nil {
		return ccdaemon.StartResult{}, "", err
	}
	posted, err := postCard(ctx, b.session, port, card, q, ts, warn)
	if err != nil {
		return ccdaemon.StartResult{}, "", fmt.Errorf("card %q: %w", card.ID, err)
	}
	card.Slack = &doc.CardSlack{URL: posted.Permalink}
	blockJSON, err := json.Marshal(card)
	if err != nil {
		return ccdaemon.StartResult{}, "", err
	}
	res, err := cl.SlackCard(ctx, b.session, b.scope, b.pid, wr, blockJSON)
	if err != nil {
		return ccdaemon.StartResult{}, "", fmt.Errorf("card %q is posted in Slack at %s but not on the board: %w", card.ID, posted.Permalink, err)
	}
	return res, posted.TS, nil
}

type cardPost struct {
	Permalink string `json:"permalink"`
	TS        string `json:"ts"`
}

func postCard(ctx context.Context, session string, port int, card *doc.Card, q doc.Block, ts string, warn io.Writer) (cardPost, error) {
	sc, err := json.Marshal(slackcards.ToSlack(card, q))
	if err != nil {
		return cardPost{}, err
	}
	args := []string{"dm-status", "--card", string(sc), "--mirrored", "--session", session}
	if ts != "" {
		args = append(args, "--thread", ts)
	}
	dir, err := os.MkdirTemp("", "cc-present-images-")
	if err != nil {
		return cardPost{}, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	for i, image := range slackcards.Images(card) {
		b, err := imageBytes(ctx, port, image.Src)
		if err != nil {
			return cardPost{}, fmt.Errorf("image %q: %w", image.Title, err)
		}
		ext, ok := slackImageExts[http.DetectContentType(b)]
		if !ok {
			_, _ = fmt.Fprintf(warn, "warning: card %q: image %q stays on the board; Slack shows PNG, JPEG, or GIF, and it is %s\n", card.ID, image.Title, http.DetectContentType(b))
			continue
		}
		path := filepath.Join(dir, strconv.Itoa(i), imageFileName(image.Title)+ext)
		if err := os.Mkdir(filepath.Dir(path), 0o700); err != nil {
			return cardPost{}, err
		}
		if err := os.WriteFile(path, b, 0o600); err != nil {
			return cardPost{}, err
		}
		args = append(args, "--file", path)
	}
	out, err := slackcards.CCSlack(ctx, args...)
	if err != nil {
		return cardPost{}, err
	}
	var posted cardPost
	if err := json.Unmarshal(out, &posted); err != nil || posted.Permalink == "" {
		return cardPost{}, fmt.Errorf("cc-slack posted it but returned no permalink: %s", out)
	}
	return posted, nil
}

var slackImageExts = map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/gif": ".gif"}

const (
	maxImageName = 80
	imageTimeout = 30 * time.Second
)

func imageFileName(title string) string {
	name := []rune(strings.NewReplacer("/", "-", "\\", "-", "\n", " ").Replace(title))
	return string(name[:min(len(name), maxImageName)])
}

func imageBytes(ctx context.Context, port int, src string) ([]byte, error) {
	if body, ok := strings.CutPrefix(src, "data:"); ok {
		return dataURIBytes(body)
	}
	target := src
	if sha, ok := strings.CutPrefix(src, "asset:"); ok {
		target = fmt.Sprintf("http://127.0.0.1:%d/assets/%s", port, sha)
	}
	ctx, cancel := context.WithTimeout(ctx, imageTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req) //nolint:gosec // G704: the board's own image, on its daemon or the https address its author gave
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", target, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, assets.MaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > assets.MaxBytes {
		return nil, fmt.Errorf("GET %s: over %d bytes", target, assets.MaxBytes)
	}
	return b, nil
}

func dataURIBytes(body string) ([]byte, error) {
	meta, data, ok := strings.Cut(body, ",")
	if !ok {
		return nil, errors.New("data URI has no comma")
	}
	if strings.HasSuffix(meta, ";base64") {
		return base64.StdEncoding.DecodeString(data)
	}
	text, err := url.PathUnescape(data)
	return []byte(text), err
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
			_, err = cl.SlackCard(ctx, sess, mustCwd(cwd), pid, ccdaemon.Writer{}, blockJSON)
			return err
		},
	}
	c.Flags().StringVar(&session, "session", "", "Claude session id (defaults to $CLAUDE_CODE_SESSION_ID)")
	c.Flags().StringVar(&cwd, "cwd", "", "working directory (recorded on the request; artifacts are per-window, not resolved by directory)")
	return c
}

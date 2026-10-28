// Package slackcards maps a board card to the cc-slack ask card that asks the
// same question, and an answer on either surface to the other.
package slackcards

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/state"
)

const (
	answerSuffix    = "-answer"
	contextSuffix   = "-context"
	otherLabel      = "Other"
	explainLabel    = "Explain more"
	recommendedMark = "(Recommended)"
	maxTitle        = 140
)

// Resolved is the status a synced card takes once either surface answers it.
const Resolved = "resolved"

var slackLink = regexp.MustCompile(`<(https?://[^|>\s]+)\|([^>]+)>`)

var markdownLink = regexp.MustCompile(`\[([^\]]+)\]\((https?://[^)\s]+)\)`)

// Posted is a waiting card cc-slack posted, as `cc-present slack-sync posted` reads it.
type Posted struct {
	Waits       string   `json:"waits"`
	Title       string   `json:"title"`
	Body        string   `json:"body,omitempty"`
	Options     []string `json:"options"`
	Recommended string   `json:"recommended"`
	Permalink   string   `json:"permalink"`
	PRs         []PR     `json:"prs,omitempty"`
}

// PR is a GitHub pull request a card is about.
type PR struct {
	Repo   string `json:"repo"`
	Number int    `json:"number"`
}

// Answered is an answer given on Slack, as `cc-present slack-sync answered` reads it.
type Answered struct {
	Waits     string    `json:"waits"`
	Label     string    `json:"label"`
	Other     bool      `json:"other,omitempty"`
	At        time.Time `json:"at"`
	Permalink string    `json:"permalink"`
}

// StatusCard is the cc-slack dm-status card that asks a board card's question.
type StatusCard struct {
	Status      string   `json:"status"`
	Title       string   `json:"title"`
	Body        string   `json:"body,omitempty"`
	Options     []string `json:"options"`
	Recommended string   `json:"recommended"`
	Waits       string   `json:"waits"`
	Refs        []Ref    `json:"refs,omitempty"`
	Group       string   `json:"group,omitempty"`
	Rows        []Row    `json:"rows,omitempty"`
}

// Row is one pull request on a group card, with the board's verdict buttons.
type Row struct {
	ID      string   `json:"id"`
	Text    string   `json:"text"`
	Buttons []string `json:"buttons"`
}

// Decided is a tap on a group card's row, as `cc-present slack-sync decided` reads it.
type Decided struct {
	Row       string    `json:"row"`
	Label     string    `json:"label"`
	At        time.Time `json:"at"`
	Permalink string    `json:"permalink"`
}

// Ref is a pull request link the Slack card shows with its live state.
type Ref struct {
	URL string `json:"url"`
}

// SlackAnswer is cc-slack's reply to `cc-slack card-answer`: whether this answer
// won, and the label of the answer that did.
type SlackAnswer struct {
	Answered bool   `json:"answered"`
	Label    string `json:"label"`
	Other    bool   `json:"other,omitempty"`
}

// AnswerID is the id FromSlack gives the choice holding a card's answers.
func AnswerID(cardID string) string { return cardID + answerSuffix }

// Question returns a synced card's one choice, the question both surfaces ask.
func Question(c *doc.Card) (*doc.Choice, bool) {
	var question *doc.Choice
	for _, child := range c.Children {
		if ch, ok := child.(*doc.Choice); ok {
			if question != nil {
				return nil, false
			}
			question = ch
		}
	}
	return question, question != nil
}

// Check enforces the rules a card follows on both surfaces: one single-select
// question whose answers are distinct, exactly one of them recommended, and none
// of them Other or Explain more, since both surfaces draw those themselves.
// cc-slack holds the card to Slack's own limits when it posts it.
func Check(c *doc.Card) (*doc.Choice, error) {
	ch, ok := Question(c)
	if !ok {
		return nil, fmt.Errorf("card %q asks one question: exactly one choice child", c.ID)
	}
	if ch.Multi {
		return nil, fmt.Errorf("choice %q takes one answer; it must be single-select", ch.ID)
	}
	var labels []string
	recommended := 0
	for _, o := range ch.Options {
		switch {
		case o.Label == otherLabel || o.Label == explainLabel:
			return nil, fmt.Errorf("choice %q answer %q: both surfaces draw %s and %s themselves", ch.ID, o.ID, otherLabel, explainLabel)
		case strings.Contains(o.Label, recommendedMark):
			return nil, fmt.Errorf("choice %q answer %q: mark the recommended answer with recommended, never %s in its label", ch.ID, o.ID, recommendedMark)
		case slices.Contains(labels, o.Label):
			return nil, fmt.Errorf("choice %q repeats the answer %q", ch.ID, o.Label)
		}
		labels = append(labels, o.Label)
		if o.Recommended {
			recommended++
		}
	}
	if recommended != 1 {
		return nil, fmt.Errorf("choice %q recommends %d answers; a synced card recommends exactly one", ch.ID, recommended)
	}
	return ch, nil
}

// ToSlack is the cc-slack card for a card Check passed.
func ToSlack(c *doc.Card, ch *doc.Choice) StatusCard {
	sc := StatusCard{Status: "needs_you", Title: bounded(c.Title), Body: c.Summary, Waits: c.ID}
	for _, o := range ch.Options {
		sc.Options = append(sc.Options, o.Label)
		if o.Recommended {
			sc.Recommended = o.Label
		}
	}
	parts := []string{c.Summary}
	for _, child := range c.Children {
		switch b := child.(type) {
		case *doc.Markdown:
			if !b.Struck {
				parts = append(parts, markdownLink.ReplaceAllString(b.Md, "<$2|$1>"))
			}
		case *doc.PR:
			url := fmt.Sprintf("https://github.com/%s/pull/%d", b.Repo, b.Number)
			sc.Refs = append(sc.Refs, Ref{URL: url})
			_, name, _ := strings.Cut(b.Repo, "/")
			parts = append(parts, fmt.Sprintf("<%s|%s PR %d>", url, name, b.Number))
		}
	}
	sc.Body = strings.Join(slices.DeleteFunc(parts, func(s string) bool { return s == "" }), "\n")
	return sc
}

func bounded(title string) string {
	runes := []rune(title)
	if len(runes) <= maxTitle {
		return title
	}
	return string(runes[:maxTitle-1]) + "…"
}

// FromSlack is the board card for a waiting card cc-slack posted, as block JSON.
func FromSlack(p Posted) (json.RawMessage, error) {
	if p.Permalink == "" {
		return nil, errors.New("a posted Slack card carries its permalink")
	}
	options := make([]map[string]any, 0, len(p.Options))
	for i, label := range p.Options {
		options = append(options, map[string]any{"id": fmt.Sprintf("answer-%d", i+1), "label": label, "recommended": label == p.Recommended})
	}
	var children []map[string]any
	card := map[string]any{"id": p.Waits, "type": "card", "title": p.Title, "slack": map[string]string{"url": p.Permalink}}
	if strings.Contains(p.Body, "\n") || slackLink.MatchString(p.Body) {
		children = append(children, map[string]any{"id": p.Waits + contextSuffix, "type": "markdown", "md": slackLink.ReplaceAllString(p.Body, "[$2]($1)")})
	} else if p.Body != "" {
		card["summary"] = p.Body
	}
	for _, pr := range p.PRs {
		children = append(children, map[string]any{"id": fmt.Sprintf("%s-pr-%s-%d", p.Waits, strings.ReplaceAll(pr.Repo, "/", "-"), pr.Number), "type": "pr", "repo": pr.Repo, "number": pr.Number})
	}
	children = append(children, map[string]any{"id": AnswerID(p.Waits), "type": "choice", "options": options})
	card["children"] = children
	return json.Marshal(card)
}

// Selection is the board selection an answer given on Slack makes on ch: its
// option when the label names one, else a written-in answer.
func Selection(ch *doc.Choice, label string, other bool) ([]string, string) {
	if !other {
		for _, o := range ch.Options {
			if o.Label == label {
				return []string{o.ID}, ""
			}
		}
	}
	return []string{}, label
}

// Label is the answer a board selection gives cc-slack, and whether it is written in.
func Label(ch *doc.Choice, optionIDs []string, other string) (string, bool) {
	for _, o := range ch.Options {
		if len(optionIDs) > 0 && o.ID == optionIDs[0] {
			return o.Label, false
		}
	}
	return other, true
}

// CCSlack runs the cc-slack CLI with args and returns its stdout, or an error
// carrying its stderr.
func CCSlack(ctx context.Context, args ...string) ([]byte, error) {
	out, err := exec.CommandContext(ctx, "cc-slack", args...).Output() //nolint:gosec // G204: argv to the cc-slack binary on PATH, never a shell
	if err == nil {
		return out, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && len(exit.Stderr) > 0 {
		return nil, fmt.Errorf("cc-slack %s: %s", args[0], strings.TrimSpace(string(exit.Stderr)))
	}
	return nil, fmt.Errorf("cc-slack %s: %w", args[0], err)
}

const (
	approveLabel = "Approve"
	holdLabel    = "Hold"
	shortHead    = 7
	rowHead      = 12
	maxRowText   = 3000
	maxBody      = 2800
	maxMore      = 32
	maxGroupRows = 20
)

var rowVerdicts = map[string]string{approveLabel: "approved", holdLabel: "rejected"}

var rowFacts = []struct {
	label string
	value func(doc.PRRow) string
}{
	{"What changes", func(r doc.PRRow) string { return r.Change }},
	{"Against the plan", func(r doc.PRRow) string { return r.Plan }},
	{"Risk", func(r doc.PRRow) string { return r.Risk }},
	{"Review and CI", func(r doc.PRRow) string { return r.Review }},
	{"Blocks landing", func(r doc.PRRow) string { return r.Blocker }},
}

var mrkdwnEscapes = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "**", "*")

// GroupCard is the cc-slack cards for one group of a pr-groups block, one per
// maxGroupRows rows: a row with buttons for each pull request whose verdict does
// not stand at its head, and a line naming the verdict for each one whose verdict
// does. A group that left the board (live false) keeps no buttons: each row is its
// last verdict or that it left. A page the group's rows or its held rows from the
// last push could fill but its rows no longer reach settles with no buttons.
func GroupCard(g *doc.PRGroups, group doc.PRGroup, verdicts map[string]state.Decision, live bool, held int) []StatusCard {
	var rows []Row
	var settled []string
	for _, r := range group.PRs {
		d, decided := verdicts[doc.RowID(r.Number)]
		switch {
		case decided && (r.Stands(d.Head) || !live):
			settled = append(settled, prLink(g.Repo, r)+" · "+verdictLine(d))
		case !live:
			settled = append(settled, prLink(g.Repo, r)+" · left the board")
		default:
			rows = append(rows, Row{
				ID:      fmt.Sprintf("%s:%d:%s", g.ID, r.Number, r.Head[:min(len(r.Head), rowHead)]),
				Text:    rowText(g.Repo, r, d, decided),
				Buttons: []string{approveLabel, holdLabel},
			})
		}
	}
	pages := slices.Collect(slices.Chunk(rows, maxGroupRows))
	key := g.ID + ":" + group.ID
	cards := make([]StatusCard, max(1, len(pages), (max(len(group.PRs), held)+maxGroupRows-1)/maxGroupRows))
	for i := range cards {
		cards[i] = StatusCard{Status: "done", Title: bounded(group.Title), Group: key}
		if i > 0 {
			cards[i].Group = fmt.Sprintf("%s:%d", key, i+1)
		}
		if i >= len(pages) {
			cards[i].Body = "No row on this card waits on you now."
			continue
		}
		cards[i].Rows, cards[i].Status = pages[i], "needs_you"
		if len(pages) > 1 {
			cards[i].Title = bounded(fmt.Sprintf("%s (%d of %d)", group.Title, i+1, len(pages)))
		}
	}
	cards[0].Body = groupBody(group.Summary, settled)
	return cards
}

func groupBody(summary string, settled []string) string {
	body := string([]rune(summary)[:min(len([]rune(summary)), maxBody-maxMore)])
	for i, line := range settled {
		more := fmt.Sprintf("\n+%d more answered", len(settled)-i)
		if len([]rune(body+"\n"+line+more)) > maxBody {
			return body + more
		}
		body += "\n" + line
	}
	return body
}

func prLink(repo string, r doc.PRRow) string {
	_, name, _ := strings.Cut(repo, "/")
	return fmt.Sprintf("*<https://github.com/%s/pull/%d|%s PR %d>* %s `%s`", repo, r.Number, name, r.Number, mrkdwn(r.Title), r.Head[:shortHead])
}

func rowText(repo string, r doc.PRRow, d state.Decision, decided bool) string {
	lines := []string{prLink(repo, r)}
	for _, f := range rowFacts {
		lines = append(lines, "*"+f.label+":* "+mrkdwn(f.value(r)))
	}
	if decided {
		lines = append(lines, "_"+verdictLine(d)+"; new head "+r.Head[:shortHead]+" needs your tap._")
	}
	text := []rune(strings.Join(lines, "\n"))
	if len(text) <= maxRowText {
		return string(text)
	}
	return string(text[:maxRowText-1]) + "…"
}

func verdictLine(d state.Decision) string {
	word := "Held"
	if d.Verdict == "approved" {
		word = "Approved"
	}
	if tapped := cmp.Or(d.CarriedFrom, d.Head); tapped != "" {
		word += " at " + tapped[:min(len(tapped), shortHead)]
	}
	if at, err := time.Parse(time.RFC3339, d.At); err == nil {
		word += " on " + at.Local().Format("Jan 2 3:04 PM")
	}
	if d.Note != "" && d.Verdict != "approved" {
		word += ": " + mrkdwn(d.Note)
	}
	return word
}

func mrkdwn(md string) string {
	return markdownLink.ReplaceAllString(mrkdwnEscapes.Replace(md), "<$2|$1>")
}

// Verdict is the board verdict a row tap gives: the pr-groups block, the row's
// verdict key, the head the row was asked at, and approved or rejected.
func (d Decided) Verdict() (block, row, head, verdict string, err error) {
	verdict, ok := rowVerdicts[d.Label]
	if !ok {
		return "", "", "", "", fmt.Errorf("a row tap answers %s or %s, not %q", approveLabel, holdLabel, d.Label)
	}
	rest, head, ok := cutLast(d.Row)
	if !ok {
		return "", "", "", "", fmt.Errorf("row %q is not <block>:<number>:<head>", d.Row)
	}
	block, row, ok = cutLast(rest)
	if !ok || block == "" {
		return "", "", "", "", fmt.Errorf("row %q is not <block>:<number>:<head>", d.Row)
	}
	if _, err := doc.ParseHead(head); err != nil {
		return "", "", "", "", fmt.Errorf("row %q: %w", d.Row, err)
	}
	return block, row, head, verdict, nil
}

func cutLast(s string) (string, string, bool) {
	i := strings.LastIndex(s, ":")
	if i < 0 {
		return "", "", false
	}
	return s[:i], s[i+1:], true
}

// GroupCards is every card a pr-groups block's push edits: a card with no
// buttons for each group of before that it dropped, then one per group it carries.
func GroupCards(before, g *doc.PRGroups, verdicts map[string]state.Decision) []StatusCard {
	kept := map[string]bool{}
	for _, group := range g.Groups {
		kept[group.ID] = true
	}
	var cards []StatusCard
	if before != nil {
		for _, group := range before.Groups {
			if !kept[group.ID] {
				cards = append(cards, GroupCard(before, group, verdicts, false, 0)...)
			}
		}
	}
	for _, group := range g.Groups {
		held := 0
		if before != nil {
			if i := slices.IndexFunc(before.Groups, func(b doc.PRGroup) bool { return b.ID == group.ID }); i >= 0 {
				held = len(before.Groups[i].PRs)
			}
		}
		cards = append(cards, GroupCard(g, group, verdicts, true, held)...)
	}
	return cards
}

// PostGroupCard posts or edits a group card in the DM status thread it is bound
// to, and returns its permalink; a card cc-slack never posted and has nothing
// to ask returns none.
func PostGroupCard(ctx context.Context, session, thread string, card StatusCard) (string, error) {
	raw, err := json.Marshal(card)
	if err != nil {
		return "", err
	}
	out, err := CCSlack(ctx, "dm-status", "--card", string(raw), "--session", session, "--thread", thread)
	if err != nil {
		return "", fmt.Errorf("group %q: %w", card.Group, err)
	}
	var posted struct {
		Permalink string `json:"permalink"`
	}
	if err := json.Unmarshal(out, &posted); err != nil {
		return "", fmt.Errorf("group %q: decode cc-slack reply %q: %w", card.Group, out, err)
	}
	return posted.Permalink, nil
}

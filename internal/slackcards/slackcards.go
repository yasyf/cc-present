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
	"reflect"
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
	maxLabel        = 200
	maxAnswers      = 4
)

// Resolved is the status a synced card takes once either surface answers it.
const Resolved = "resolved"

var slackLink = regexp.MustCompile(`<(https?://[^|>\s]+)\|([^>]+)>`)

var markdownLink = regexp.MustCompile(`\[([^\]]+)\]\((https?://[^)\s]+)\)`)

var cardLink = regexp.MustCompile(`\[([^\]]*)\]\(card:([^)\s]+)\)`)

// Posts maps a board card's id to its Slack message, the one place a card:ID
// link goes when it is drawn in Slack.
type Posts map[string]string

// PostsOf reads each card's Slack message from the board doc.
func PostsOf(d *doc.Doc) Posts {
	posts := Posts{}
	for _, b := range d.Blocks {
		if c, ok := b.(*doc.Card); ok && c.Slack != nil {
			posts[c.ID] = c.Slack.URL
		}
	}
	return posts
}

func (p Posts) text(md string) string {
	return slackText(cardLink.ReplaceAllStringFunc(md, func(link string) string {
		found := cardLink.FindStringSubmatch(link)
		if url := p[found[2]]; url != "" {
			return "[" + cmp.Or(found[1], url) + "](" + url + ")"
		}
		return found[1]
	}))
}

var (
	refWithURL = regexp.MustCompile(`\[([^\]]+)\]\(<(https?://[^>\s]+)>(?:\s+"ref:[0-9a-f]{64}")?\)`)
	refNoURL   = regexp.MustCompile(`\[([^\]]+)\]\(<>\s+"ref:[0-9a-f]{64}"\)`)
)

func withoutRefs(md string) string {
	return refNoURL.ReplaceAllString(refWithURL.ReplaceAllString(md, "[$1]($2)"), "$1")
}

func slackText(md string) string {
	return markdownLink.ReplaceAllString(withoutRefs(md), "<$2|$1>")
}

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
	Supersedes  []string `json:"supersedes,omitempty"`
	Replaces    string   `json:"replaces,omitempty"`
	PRs         []CardPR `json:"prs,omitempty"`
	// Hints are the words to read for an option, keyed by the option.
	Hints map[string]string `json:"hints,omitempty"`
}

// Row is one item on a group card, with the board's verdict buttons. A pull
// request's row carries it as PR, which cc-slack draws, and any line more as Text.
type Row struct {
	ID      string   `json:"id"`
	Text    string   `json:"text,omitempty"`
	Buttons []string `json:"buttons"`
	PR      *CardPR  `json:"pr,omitempty"`
}

// CardPR is a pull request as cc-slack draws it: its link and live state, its
// four plain lines as fields, and its description, which cc-slack reads itself.
type CardPR struct {
	URL    string `json:"url"`
	Title  string `json:"title,omitempty"`
	Head   string `json:"head,omitempty"`
	Review string `json:"review,omitempty"`
	Change string `json:"change"`
	Plan   string `json:"plan"`
	Risk   string `json:"risk"`
	Blocks string `json:"blocks"`
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

func answerBlocks(c *doc.Card) []doc.Block {
	var found []doc.Block
	for _, child := range c.Children {
		switch child.(type) {
		case *doc.Choice, *doc.Approval, *doc.Input, *doc.Triage:
			found = append(found, child)
		}
	}
	return found
}

// Asks reports whether a card takes any answer from the human.
func Asks(c *doc.Card) bool { return len(answerBlocks(c)) > 0 }

// Question returns a synced card's one question, the choice, approval, input,
// or triage both surfaces ask. A card with more than one has none.
func Question(c *doc.Card) (doc.Block, bool) {
	found := answerBlocks(c)
	if len(found) != 1 {
		return nil, false
	}
	return found[0], true
}

func prompt(q doc.Block) string {
	switch q := q.(type) {
	case *doc.Choice:
		return q.Prompt
	case *doc.Approval:
		return q.Prompt
	case *doc.Input:
		return q.Label
	case *doc.Triage:
		return q.Prompt
	}
	return ""
}

// Check enforces the rules a card follows on both surfaces: it asks one
// question, in words, and a choice takes one of at most four distinct answers,
// at most one of them recommended and none of them Other, since both surfaces
// draw it themselves. An Explain more option is no answer: cc-slack draws its
// own, so it never counts or reaches Slack. A triage asks each item as its own
// row, so its card and row ids fit cc-slack's ids. cc-slack holds the card to
// Slack's own limits when it posts it.
func Check(c *doc.Card) (doc.Block, error) {
	if !c.Mirrors() {
		return nil, fmt.Errorf("card %q says mirror false, so it stays on the board", c.ID)
	}
	q, ok := Question(c)
	if !ok {
		return nil, fmt.Errorf("card %q asks one question in Slack: exactly one choice, approval, or input child", c.ID)
	}
	if cmp.Or(c.Title, prompt(q)) == "" {
		return nil, fmt.Errorf("card %q needs a title, which Slack shows as the question", c.ID)
	}
	if tr, ok := q.(*doc.Triage); ok {
		return tr, checkRows(c, tr)
	}
	ch, ok := q.(*doc.Choice)
	if !ok {
		return q, nil
	}
	if ch.Multi {
		return nil, fmt.Errorf("choice %q takes one answer in Slack; it must be single-select", ch.ID)
	}
	answers := Answers(ch)
	if len(answers) > maxAnswers {
		return nil, fmt.Errorf("choice %q offers %d answers; Slack shows at most %d", ch.ID, len(answers), maxAnswers)
	}
	var labels []string
	recommended := 0
	for _, o := range answers {
		label := clip(o.Label, maxLabel)
		switch {
		case o.Label == otherLabel:
			return nil, fmt.Errorf("choice %q answer %q: both surfaces draw %s themselves", ch.ID, o.ID, otherLabel)
		case strings.Contains(o.Label, recommendedMark):
			return nil, fmt.Errorf("choice %q answer %q: mark the recommended answer with recommended, never %s in its label", ch.ID, o.ID, recommendedMark)
		case slices.Contains(labels, label):
			return nil, fmt.Errorf("choice %q repeats the answer %q", ch.ID, label)
		}
		labels = append(labels, label)
		if o.Recommended {
			recommended++
		}
	}
	if recommended > 1 {
		return nil, fmt.Errorf("choice %q recommends %d answers; a synced card recommends at most one", ch.ID, recommended)
	}
	return ch, nil
}

func checkRows(c *doc.Card, tr *doc.Triage) error {
	if len(c.ID)+len(":1") > maxSlackID {
		return fmt.Errorf("card %q asks per row in Slack, which names each page of it by its id and :N; keep the id under %d characters", c.ID, maxSlackID-1)
	}
	for _, item := range tr.Items {
		if !rowPart.MatchString(tr.ID) || !rowPart.MatchString(item.ID) || len(tr.ID)+len(":")+len(item.ID) > maxSlackID {
			return fmt.Errorf("triage %q item %q: Slack names a row <triage>:<item>, at most %d letters, digits, dots, underscores, or hyphens", tr.ID, item.ID, maxSlackID)
		}
	}
	return nil
}

// Answers is a choice's options minus its Explain more, the answers Slack
// shows as buttons beside its own Other and Explain more.
func Answers(ch *doc.Choice) []doc.Option {
	return slices.DeleteFunc(slices.Clone(ch.Options), explains)
}

// ExplainsMore reports whether a board selection on ch picks its Explain more
// option, which asks the session to say more and answers nothing.
func ExplainsMore(ch *doc.Choice, optionIDs []string) bool {
	return len(optionIDs) > 0 && slices.ContainsFunc(ch.Options, func(o doc.Option) bool { return o.ID == optionIDs[0] && explains(o) })
}

func explains(o doc.Option) bool { return o.Label == explainLabel }

// Nudge says why a top-level block that asks something stays off Slack and what
// its author should do about it, or "" when it reaches Slack or says it will not.
func Nudge(b doc.Block) string {
	switch b := b.(type) {
	case *doc.Card:
		if !b.Mirrors() || !Asks(b) {
			return ""
		}
		if _, err := Check(b); err != nil {
			return fmt.Sprintf(`%v; it stays on the board only, so set "mirror": false on it`, err)
		}
	case *doc.Choice, *doc.Approval, *doc.Input, *doc.Triage:
		return fmt.Sprintf("%s %q stays on the board only; put it in a card to reach Slack", b.BlockType(), b.BlockID())
	}
	return ""
}

// ToSlack is the cc-slack card for a card Check passed.
func ToSlack(c *doc.Card, q doc.Block, posts Posts) StatusCard {
	title := cmp.Or(c.Title, prompt(q))
	sc := StatusCard{Status: "needs_you", Title: clip(title, maxTitle), Waits: c.ID, Supersedes: c.Supersedes, Replaces: c.Replaces}
	parts := []string{posts.text(c.Summary)}
	if sc.Title != title {
		parts = slices.Insert(parts, 0, "*"+title+"*")
	}
	if c.Title != "" {
		parts = append(parts, prompt(q))
	}
	switch q := q.(type) {
	case *doc.Approval:
		sc.Options = []string{approveLabel, rejectLabel}
	case *doc.Choice:
		for _, o := range Answers(q) {
			label := clip(o.Label, maxLabel)
			sc.Options = append(sc.Options, label)
			if o.Recommended {
				sc.Recommended = label
			}
			if hint := optionHint(o, label); hint != "" {
				if sc.Hints == nil {
					sc.Hints = map[string]string{}
				}
				sc.Hints[label] = hint
			}
		}
	}
	for _, child := range c.Children {
		switch b := child.(type) {
		case *doc.Markdown:
			if !b.Struck {
				parts = append(parts, posts.text(withoutLead(b.Md, c.Summary)))
			}
		case *doc.PR:
			url := fmt.Sprintf("https://github.com/%s/pull/%d", b.Repo, b.Number)
			if b.Change != "" || b.Plan != "" || b.Risk != "" || b.Blocker != "" {
				sc.PRs = append(sc.PRs, CardPR{URL: url, Title: b.Title, Head: b.Head, Change: b.Change, Plan: b.Plan, Risk: b.Risk, Blocks: b.Blocker})
				continue
			}
			sc.Refs = append(sc.Refs, Ref{URL: url})
			_, name, _ := strings.Cut(b.Repo, "/")
			parts = append(parts, fmt.Sprintf("<%s|%s PR %d>", url, name, b.Number))
		}
	}
	sc.Body = strings.Join(slices.DeleteFunc(parts, func(s string) bool { return s == "" }), "\n")
	return sc
}

// Cards is the cc-slack cards that ask a card Check passed: its one waiting card,
// or for a triage one group card per maxGroupRows items, each item still open a
// row with the board's Approve and Reject, and each decided one a line naming
// its verdict.
func Cards(c *doc.Card, q doc.Block, verdicts map[string]state.Decision, posts Posts) []StatusCard {
	sc := ToSlack(c, q, posts)
	tr, ok := q.(*doc.Triage)
	if !ok {
		return []StatusCard{sc}
	}
	pages := slices.Collect(slices.Chunk(tr.Items, maxGroupRows))
	cards := make([]StatusCard, len(pages))
	for i, items := range pages {
		page := StatusCard{Status: "done", Title: sc.Title, Group: c.ID}
		summary := ""
		if i == 0 {
			summary, page.Refs, page.Supersedes = sc.Body, sc.Refs, sc.Supersedes
		} else {
			page.Group = fmt.Sprintf("%s:%d", c.ID, i+1)
		}
		if len(pages) > 1 {
			page.Title = clip(fmt.Sprintf("%s (%d of %d)", cmp.Or(c.Title, prompt(q)), i+1, len(pages)), maxTitle)
		}
		var settled []string
		for _, item := range items {
			if d, decided := verdicts[item.ID]; decided {
				settled = append(settled, "*"+mrkdwn(item.Label)+"* · "+verdictLine(d, "Rejected"))
				continue
			}
			page.Rows = append(page.Rows, Row{ID: tr.ID + ":" + item.ID, Text: itemText(item), Buttons: []string{approveLabel, rejectLabel}})
		}
		if len(page.Rows) > 0 {
			page.Status = "needs_you"
		}
		page.Body = groupBody(summary, settled)
		cards[i] = page
	}
	return cards
}

func withoutLead(md, summary string) string {
	lead, rest, _ := strings.Cut(strings.TrimSpace(md), "\n\n")
	if strings.Join(strings.Fields(lead), " ") != strings.Join(strings.Fields(summary), " ") {
		return md
	}
	return strings.TrimSpace(rest)
}

func itemText(item doc.Item) string {
	lines := []string{"*" + mrkdwn(item.Label) + "*"}
	if item.Hint != "" {
		lines = append(lines, mrkdwn(item.Hint))
	}
	for _, f := range item.Facts {
		if f.Label != "" {
			lines = append(lines, "*"+mrkdwn(f.Label)+":* "+mrkdwn(f.Value))
		} else {
			lines = append(lines, mrkdwn(f.Value))
		}
	}
	if item.Md != "" {
		lines = append(lines, mrkdwn(item.Md))
	}
	return strings.Join(lines, "\n")
}

func optionHint(o doc.Option, label string) string {
	var lines []string
	if label != o.Label {
		lines = append(lines, o.Label)
	}
	for _, effect := range []string{o.Hint, o.Md} {
		if effect != "" {
			lines = append(lines, slackText(effect))
		}
	}
	if o.Hint == "" && o.Md == "" && o.Detail != nil && o.Detail.Md != "" {
		lines = append(lines, slackText(o.Detail.Md))
	}
	for _, f := range o.Facts {
		if f.Label == "" {
			lines = append(lines, mrkdwn(f.Value))
			continue
		}
		lines = append(lines, "*"+mrkdwn(f.Label)+":* "+mrkdwn(f.Value))
	}
	switch {
	case o.Build == nil:
	case o.Build.None != "":
		lines = append(lines, "Builds nothing: "+o.Build.None)
	default:
		lines = append(lines, fmt.Sprintf("Built on <%s|%s %s>", o.Build.URL(), o.Build.Repo, o.Build.Branch))
	}
	return strings.Join(lines, "\n")
}

func clip(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit-1]) + "…"
}

// Image is a picture a card shows, and the words that say which one it is.
type Image struct {
	Src   string
	Title string
}

// Images is every picture a card shows: its image children, and the image each
// option carries, named for that option.
func Images(c *doc.Card) []Image {
	var images []Image
	for _, child := range c.Children {
		switch b := child.(type) {
		case *doc.Image:
			images = append(images, Image{b.Src, cmp.Or(b.Caption, b.Alt)})
		case *doc.Choice:
			for _, o := range b.Options {
				if v, ok := o.Visual.(*doc.Image); ok {
					images = append(images, Image{v.Src, o.Label})
				}
			}
		}
	}
	return images
}

// Same reports whether two cards ask the same thing in Slack, so posting the
// second over the first would change nothing.
func Same(a, b *doc.Card) bool {
	qa, oka := Question(a)
	qb, okb := Question(b)
	return oka && okb && reflect.DeepEqual(Cards(a, qa, nil, nil), Cards(b, qb, nil, nil)) && slices.Equal(Images(a), Images(b))
}

// Installed reports whether cc-slack is here to carry a card to Slack.
func Installed() bool {
	_, err := exec.LookPath("cc-slack")
	return err == nil
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
			if clip(o.Label, maxLabel) == label {
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
			return clip(o.Label, maxLabel), false
		}
	}
	return other, true
}

// ApprovalVerdict is the verdict an answer given on Slack makes on an approval,
// and its note: words written in instead of a button reject, with those words.
func ApprovalVerdict(label string, other bool) (verdict, note string) {
	switch {
	case other:
		return "rejected", label
	case label == approveLabel:
		return "approved", ""
	}
	return "rejected", ""
}

// ApprovalLabel is the button a board verdict presses in Slack.
func ApprovalLabel(verdict string) string {
	if verdict == "approved" {
		return approveLabel
	}
	return rejectLabel
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
	rejectLabel  = "Reject"
	holdLabel    = "Hold"
	shortHead    = 7
	rowHead      = 12
	maxBody      = 40000
	maxMore      = 32
	maxGroupRows = 20
	maxPRRows    = 8
	maxSlackID   = 64
)

var rowPart = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

var (
	rowVerdicts  = map[string]string{approveLabel: "approved", holdLabel: "rejected"}
	itemVerdicts = map[string]string{approveLabel: "approved", rejectLabel: "rejected"}
)

var mrkdwnEscapes = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "**", "*")

// GroupCard is the cc-slack cards for one group of a pr-groups block, one per
// maxPRRows rows: a row with buttons for each pull request whose verdict does
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
			settled = append(settled, prLink(g.Repo, r)+" · "+verdictLine(d, "Held"))
		case !live:
			settled = append(settled, prLink(g.Repo, r)+" · left the board")
		default:
			rows = append(rows, Row{
				ID:      fmt.Sprintf("%s:%d:%s", g.ID, r.Number, r.Head[:min(len(r.Head), rowHead)]),
				Text:    rowNote(g.Repo, r, d, decided),
				Buttons: []string{approveLabel, holdLabel},
				PR: &CardPR{
					URL: fmt.Sprintf("https://github.com/%s/pull/%d", g.Repo, r.Number), Title: r.Title, Head: r.Head, Review: r.Review,
					Change: r.Change, Plan: r.Plan, Risk: r.Risk, Blocks: r.Blocker,
				},
			})
		}
	}
	pages := slices.Collect(slices.Chunk(rows, maxPRRows))
	key := g.ID + ":" + group.ID
	cards := make([]StatusCard, max(1, len(pages), (max(len(group.PRs), held)+maxPRRows-1)/maxPRRows))
	for i := range cards {
		cards[i] = StatusCard{Status: "done", Title: clip(group.Title, maxTitle), Group: key}
		if i > 0 {
			cards[i].Group = fmt.Sprintf("%s:%d", key, i+1)
		}
		if i >= len(pages) {
			cards[i].Body = "No row on this card waits on you now."
			continue
		}
		cards[i].Rows, cards[i].Status = pages[i], "needs_you"
		if len(pages) > 1 {
			cards[i].Title = clip(fmt.Sprintf("%s (%d of %d)", group.Title, i+1, len(pages)), maxTitle)
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

func rowNote(repo string, r doc.PRRow, d state.Decision, decided bool) string {
	var lines []string
	if len(r.Stack) > 0 {
		lines = append(lines, "*One tap also decides:* "+stackLine(repo, r.Stack))
	}
	if decided {
		lines = append(lines, "_"+verdictLine(d, "Held")+"; new head "+r.Head[:shortHead]+" needs your tap._")
	}
	return strings.Join(lines, "\n")
}

func stackLine(repo string, stack []doc.StackMember) string {
	members := make([]string, len(stack))
	for i, m := range stack {
		members[i] = fmt.Sprintf("<https://github.com/%s/pull/%d|#%d> `%s`", repo, m.Number, m.Number, m.Head[:min(len(m.Head), shortHead)])
	}
	return strings.Join(members, ", ")
}

func verdictLine(d state.Decision, rejected string) string {
	word := rejected
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
	return markdownLink.ReplaceAllString(mrkdwnEscapes.Replace(withoutRefs(md)), "<$2|$1>")
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

// Item is the triage verdict a tap on a triage card's row gives: the triage
// block, its item, and approved or rejected. A tap with any other label still
// names its block and item, so the caller can tell a triage row's wrong label
// from another block's row.
func (d Decided) Item() (block, item, verdict string, err error) {
	block, item, _ = strings.Cut(d.Row, ":")
	verdict, ok := itemVerdicts[d.Label]
	if !ok {
		return block, item, "", fmt.Errorf("a triage row tap answers %s or %s, not %q", approveLabel, rejectLabel, d.Label)
	}
	return block, item, verdict, nil
}

func cutLast(s string) (string, string, bool) {
	i := strings.LastIndex(s, ":")
	if i < 0 {
		return "", "", false
	}
	return s[:i], s[i+1:], true
}

// GroupCards is every card a pr-groups block's push edits: a card with no
// buttons for each group of before that it dropped, listing only the pull
// requests no group it carries holds, then one per group it carries that
// changed.
func GroupCards(before, g *doc.PRGroups, verdicts map[string]state.Decision) []StatusCard {
	kept, carried := map[string]bool{}, map[int]bool{}
	for _, group := range g.Groups {
		kept[group.ID] = true
		for _, r := range group.PRs {
			carried[r.Number] = true
		}
	}
	var cards []StatusCard
	if before != nil {
		for _, group := range before.Groups {
			if !kept[group.ID] {
				group.PRs = slices.DeleteFunc(slices.Clone(group.PRs), func(r doc.PRRow) bool { return carried[r.Number] })
				cards = append(cards, GroupCard(before, group, verdicts, false, 0)...)
			}
		}
	}
	for _, group := range g.Groups {
		held := 0
		var was []StatusCard
		if before != nil {
			if i := slices.IndexFunc(before.Groups, func(b doc.PRGroup) bool { return b.ID == group.ID }); i >= 0 {
				if before.Repo == g.Repo && reflect.DeepEqual(before.Groups[i], group) {
					continue
				}
				held = len(before.Groups[i].PRs)
				was = GroupCard(before, before.Groups[i], verdicts, true, 0)
			}
		}
		now := GroupCard(g, group, verdicts, true, held)
		if was != nil && asksNothing(was) && asksNothing(now) {
			continue
		}
		cards = append(cards, now...)
	}
	return cards
}

func asksNothing(cards []StatusCard) bool {
	return !slices.ContainsFunc(cards, func(c StatusCard) bool { return len(c.Rows) > 0 })
}

// PostGroupCard posts or edits a group card at the DM root, keeping the message
// it is bound to watched, the session's own when thread is "", and returns its
// permalink; a card cc-slack never posted and has nothing to ask returns none.
func PostGroupCard(ctx context.Context, session, thread string, card StatusCard) (string, error) {
	raw, err := json.Marshal(card)
	if err != nil {
		return "", err
	}
	args := []string{"dm-status", "--card", string(raw), "--session", session}
	if thread != "" {
		args = append(args, "--thread", thread)
	}
	out, err := CCSlack(ctx, args...)
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

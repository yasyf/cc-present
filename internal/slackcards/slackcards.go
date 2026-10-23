// Package slackcards maps a board card to the cc-slack ask card that asks the
// same question, and an answer on either surface to the other.
package slackcards

import (
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

package daemon

import (
	"slices"
	"strings"

	"github.com/yasyf/cc-present/internal/doc"
)

// shownWords is what card said at the moment of an answer, on one line: its
// title, summary, markdown, prompts, and option labels and hints. The log
// keeps it on the answer because an agent can rewrite the card afterwards, so a
// reader can tell what the human read when they answered.
func shownWords(card *doc.Card) string {
	if card == nil {
		return ""
	}
	words := []string{card.Title, card.Summary}
	for _, child := range card.Children {
		switch b := child.(type) {
		case *doc.Markdown:
			if !b.Struck {
				words = append(words, b.Md)
			}
		case *doc.Approval:
			words = append(words, b.Prompt)
		case *doc.Choice:
			words = append(words, b.Prompt)
			for _, option := range b.Options {
				words = append(words, option.Label, option.Hint)
			}
		}
	}
	return strings.Join(strings.Fields(strings.Join(words, " ")), " ")
}

// pickedLabels names each picked option as ch showed it when picked.
func pickedLabels(ch *doc.Choice, ids []string) []string {
	labels := []string{}
	for _, option := range ch.Options {
		if slices.Contains(ids, option.ID) {
			labels = append(labels, option.Label)
		}
	}
	return labels
}

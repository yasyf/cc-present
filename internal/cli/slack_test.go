package cli

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/state"
)

const sendBoard = `{"version":1,"title":"Questions","blocks":[
  {"id":"intro","type":"markdown","md":"Read these first."},
  {"id":"merge","type":"card","title":"Merge #41 now?","children":[{"id":"merge-answer","type":"choice","options":[{"id":"a","label":"Merge","recommended":true},{"id":"b","label":"Hold"}]}]},
  {"id":"done","type":"card","title":"Ship it?","status":"resolved","slack":{"url":"https://example.slack.com/archives/D1/p1800000001000100"},"children":[{"id":"done-answer","type":"choice","options":[{"id":"a","label":"Ship","recommended":true}]}]},
  {"id":"picked","type":"card","title":"Rename it?","children":[{"id":"picked-answer","type":"choice","options":[{"id":"a","label":"Rename","recommended":true},{"id":"b","label":"Keep"}]}]},
  {"id":"notes","type":"card","title":"Notes","children":[{"id":"notes-md","type":"markdown","md":"No question here."}]},
  {"id":"loose","type":"card","title":"Which one?","children":[{"id":"loose-answer","type":"choice","options":[{"id":"a","label":"This"},{"id":"b","label":"That"}]}]},
  {"id":"mirrored","type":"card","title":"Cut the release?","slack":{"url":"https://example.slack.com/archives/D1/p1800000002000200"},"children":[{"id":"mirrored-answer","type":"choice","options":[{"id":"a","label":"Cut","recommended":true},{"id":"b","label":"Wait"}]}]}
]}`

func sendState(t *testing.T) (*doc.Doc, map[string]state.Selection) {
	t.Helper()
	var d doc.Doc
	if err := json.Unmarshal([]byte(sendBoard), &d); err != nil {
		t.Fatal(err)
	}
	return &d, map[string]state.Selection{"picked-answer": {OptionIDs: []string{"b"}}}
}

func ids(qs []question) []string {
	var out []string
	for _, q := range qs {
		out = append(out, q.card.ID)
	}
	return out
}

func TestSendTakesEveryOpenQuestionAndWarnsOnACardBreakingTheRules(t *testing.T) {
	d, choices := sendState(t)
	picked, skipped, err := openQuestions(d, choices, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ids(picked), []string{"merge", "mirrored"}; !slices.Equal(got, want) {
		t.Fatalf("sent %v, want %v: an answered, a resolved and a question-less card stay home, and a mirrored card is sent again to edit it in place", got, want)
	}
	if picked[0].choice.ID != "merge-answer" {
		t.Fatalf("question of merge is %q, want merge-answer", picked[0].choice.ID)
	}
	joined := strings.Join(skipped, "\n")
	for _, want := range []string{`"done" is already answered`, `"picked" is already answered on the board`, `"loose-answer" recommends 0 answers`} {
		if !strings.Contains(joined, want) {
			t.Errorf("warnings %q lack %q", joined, want)
		}
	}
	if strings.Contains(joined, "notes") {
		t.Errorf("a card with no question is not a warning: %q", joined)
	}
}

func TestSendWithBlocksTakesOnlyTheNamedCards(t *testing.T) {
	d, choices := sendState(t)
	picked, _, err := openQuestions(d, choices, []string{"mirrored"})
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(picked); !slices.Equal(got, []string{"mirrored"}) {
		t.Fatalf("sent %v, want only mirrored", got)
	}
}

func TestSendRefusesANamedCardItCannotSend(t *testing.T) {
	d, choices := sendState(t)
	_, _, err := openQuestions(d, choices, []string{"merge", "loose", "notes", "gone"})
	if err == nil {
		t.Fatal("named cards that break the rules or are missing sent without an error")
	}
	for _, want := range []string{`"loose-answer" recommends 0 answers`, `"notes" asks one question`, `no card "gone"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}

func TestSlackThreadReadsATsOrAPermalink(t *testing.T) {
	tests := map[string]string{
		"":                  "",
		"1800000001.000100": "1800000001.000100",
		"https://example.slack.com/archives/D1/p1800000001000100":                                  "1800000001.000100",
		"https://example.slack.com/archives/D1/p1800000009000900?thread_ts=1800000001.000100&cid=D1": "1800000001.000100",
	}
	for in, want := range tests {
		got, err := slackThread(in)
		if err != nil || got != want {
			t.Errorf("slackThread(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"D1/1800000001.000100", "https://example.slack.com/archives/D1", "tomorrow"} {
		if _, err := slackThread(bad); err == nil {
			t.Errorf("slackThread(%q) took a value that names no Slack message", bad)
		}
	}
}

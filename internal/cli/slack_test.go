package cli

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/state"
)

const boardDoc = `{"version":1,"title":"Questions","blocks":[
  {"id":"intro","type":"markdown","md":"Read these first."},
  {"id":"merge","type":"card","title":"Merge #41 now?","children":[{"id":"merge-answer","type":"choice","options":[{"id":"a","label":"Merge","recommended":true},{"id":"b","label":"Hold"}]}]},
  {"id":"done","type":"card","title":"Ship it?","status":"resolved","slack":{"url":"https://example.slack.com/archives/D1/p1800000001000100"},"children":[{"id":"done-answer","type":"choice","options":[{"id":"a","label":"Ship","recommended":true}]}]},
  {"id":"picked","type":"card","title":"Rename it?","children":[{"id":"picked-answer","type":"choice","options":[{"id":"a","label":"Rename","recommended":true},{"id":"b","label":"Keep"}]}]},
  {"id":"notes","type":"card","title":"Notes","children":[{"id":"notes-md","type":"markdown","md":"No question here."}]},
  {"id":"loose","type":"card","title":"Which one?","children":[{"id":"loose-answer","type":"choice","options":[{"id":"a","label":"This"},{"id":"b","label":"That"}]}]},
  {"id":"mirrored","type":"card","title":"Cut the release?","slack":{"url":"https://example.slack.com/archives/D1/p1800000002000200"},"children":[{"id":"mirrored-answer","type":"choice","options":[{"id":"a","label":"Cut","recommended":true},{"id":"b","label":"Wait"}]}]},
  {"id":"old","type":"card","title":"Keep the flag?","children":[{"id":"old-answer","type":"choice","options":[{"id":"a","label":"Keep","recommended":true},{"id":"b","label":"Drop"}]}]}
]}`

func sendState(t *testing.T) *sendBoard {
	t.Helper()
	var b sendBoard
	if err := json.Unmarshal([]byte(`{"doc":`+boardDoc+`}`), &b); err != nil {
		t.Fatal(err)
	}
	b.Interactions.Choices = map[string]state.Selection{"picked-answer": {OptionIDs: []string{"b"}}}
	b.Rounds.Current = 2
	b.Rounds.BlockRounds = map[string]int{"merge": 2, "old": 1}
	return &b
}

func ids(qs []question) []string {
	out := make([]string, 0, len(qs))
	for _, q := range qs {
		out = append(out, q.card.ID)
	}
	return out
}

func TestSendTakesEveryOpenQuestionAndWarnsOnACardBreakingTheRules(t *testing.T) {
	picked, skipped, err := openQuestions(sendState(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ids(picked), []string{"merge", "mirrored"}; !slices.Equal(got, want) {
		t.Fatalf("sent %v, want %v: an answered, a resolved and a question-less card stay home, and a card in a closed round stay home, and a mirrored card is sent again to edit it in place", got, want)
	}
	if picked[0].choice.ID != "merge-answer" {
		t.Fatalf("question of merge is %q, want merge-answer", picked[0].choice.ID)
	}
	joined := strings.Join(skipped, "\n")
	for _, want := range []string{`"done" is already answered`, `"picked" is already answered on the board`, `"old" sits in a closed round`, `"loose-answer" recommends 0 answers`} {
		if !strings.Contains(joined, want) {
			t.Errorf("warnings %q lack %q", joined, want)
		}
	}
	if strings.Contains(joined, "notes") {
		t.Errorf("a card with no question is not a warning: %q", joined)
	}
}

func TestSendWithBlocksTakesOnlyTheNamedCards(t *testing.T) {
	picked, _, err := openQuestions(sendState(t), []string{"mirrored"})
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(picked); !slices.Equal(got, []string{"mirrored"}) {
		t.Fatalf("sent %v, want only mirrored", got)
	}
}

func TestSendRefusesANamedCardItCannotSend(t *testing.T) {
	_, _, err := openQuestions(sendState(t), []string{"merge", "loose", "notes", "gone"})
	if err == nil {
		t.Fatal("named cards that break the rules or are missing sent without an error")
	}
	for _, want := range []string{`"loose-answer" recommends 0 answers`, `"notes" asks one question`, `no card "gone"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}

func TestSendRefusesAClosedBoard(t *testing.T) {
	b := sendState(t)
	b.Interactions.Closed.Value = true
	if picked, _, err := openQuestions(b, nil); err == nil || len(picked) > 0 {
		t.Fatalf("a closed board sent %v with error %v; sending would open a new board", ids(picked), err)
	}
}

func TestSlackThreadReadsATsOrAPermalink(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", ""},
		{"1800000001.000100", "1800000001.000100"},
		{"https://example.slack.com/archives/D1/p1800000001000100", "1800000001.000100"},
		{"https://example.slack.com/archives/D1/p1800000009000900?thread_ts=1800000001.000100&cid=D1", "1800000001.000100"},
	}
	for _, tt := range tests {
		got, err := slackThread(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("slackThread(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
		}
	}
	for _, bad := range []string{"D1/1800000001.000100", "https://example.slack.com/archives/D1", "tomorrow"} {
		if _, err := slackThread(bad); err == nil {
			t.Errorf("slackThread(%q) took a value that names no Slack message", bad)
		}
	}
}

func TestSlackGroupsFindsABlockInsideACard(t *testing.T) {
	const group = `{"id":"pg","type":"pr-groups","repo":"a/b",%s"groups":[{"id":"g","title":"G","summary":"S.","prs":[{"number":8,"head":"abc1234","title":"T","change":"C.","plan":"P.","risk":"R.","review":"V.","blocker":"B."}]}]}`
	tests := []struct {
		name string
		raw  string
		want int
	}{
		{"top level", fmt.Sprintf(group, `"slack":"1792842193.708029",`), 1},
		{"inside a card", `{"id":"c","type":"card","title":"Merges","children":[` + fmt.Sprintf(group, `"slack":"1792842193.708029",`) + `]}`, 1},
		{"not asked on Slack", fmt.Sprintf(group, ""), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blk, err := doc.DecodeBlock(json.RawMessage(tt.raw))
			if err != nil {
				t.Fatal(err)
			}
			if got := len(slackGroups([]doc.Block{blk})); got != tt.want {
				t.Fatalf("found %d slack groups, want %d", got, tt.want)
			}
		})
	}
}

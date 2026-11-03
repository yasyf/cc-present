package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/yasyf/cc-present/internal/assets"
	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/slackcards"
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
  {"id":"old","type":"card","title":"Keep the flag?","children":[{"id":"old-answer","type":"choice","options":[{"id":"a","label":"Keep","recommended":true},{"id":"b","label":"Drop"}]}]},
  {"id":"many","type":"card","title":"Which ones?","children":[{"id":"many-answer","type":"choice","multi":true,"options":[{"id":"a","label":"This"},{"id":"b","label":"That"}]}]},
  {"id":"boardonly","type":"card","mirror":false,"title":"Which ones?","children":[{"id":"boardonly-answer","type":"choice","multi":true,"options":[{"id":"a","label":"This"},{"id":"b","label":"That"}]}]}
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
	if got, want := ids(picked), []string{"merge", "loose", "mirrored"}; !slices.Equal(got, want) {
		t.Fatalf("sent %v, want %v: an answered, a resolved and a question-less card stay home, and a card in a closed round stay home, and a mirrored card is sent again to edit it in place", got, want)
	}
	if picked[0].control.BlockID() != "merge-answer" {
		t.Fatalf("question of merge is %q, want merge-answer", picked[0].control.BlockID())
	}
	joined := strings.Join(skipped, "\n")
	for _, want := range []string{`"done" is already answered`, `"picked" is already answered on the board`, `"old" sits in a closed round`, `"many-answer" takes one answer`} {
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
	_, _, err := openQuestions(sendState(t), []string{"merge", "many", "boardonly", "notes", "gone"})
	if err == nil {
		t.Fatal("named cards that break the rules or are missing sent without an error")
	}
	for _, want := range []string{`"many-answer" takes one answer`, `"boardonly" says mirror false`, `"notes" asks one question`, `no card "gone"`} {
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

func writtenBoard(t *testing.T, blocks string) *sendBoard {
	t.Helper()
	var b sendBoard
	if err := json.Unmarshal([]byte(`{"doc":{"version":1,"title":"T","blocks":[`+blocks+`]}}`), &b); err != nil {
		t.Fatal(err)
	}
	return &b
}

func TestAWriteSendsWhatItAskedAndLeavesTheRest(t *testing.T) {
	const (
		ship     = `{"id":"ship","type":"card","title":"Ship it?","children":[{"id":"ship-q","type":"approval"}]}`
		shipped  = `{"id":"ship","type":"card","title":"Ship it?","slack":{"url":"https://example.slack.com/archives/D1/p1800000001000100"},"children":[{"id":"ship-q","type":"approval"}]}`
		reworded = `{"id":"ship","type":"card","title":"Ship it now?","slack":{"url":"https://example.slack.com/archives/D1/p1800000001000100"},"children":[{"id":"ship-q","type":"approval"}]}`
		name     = `{"id":"name","type":"card","title":"Which name?","children":[{"id":"name-q","type":"input","label":"The name"}]}`
		notes    = `{"id":"notes","type":"card","title":"Notes","children":[{"id":"notes-md","type":"markdown","md":"No question."}]}`
		parts    = `{"id":"parts","type":"card","title":"Both?","children":[{"id":"p1","type":"approval","prompt":"One?"},{"id":"p2","type":"approval","prompt":"Two?"}]}`
		quiet    = `{"id":"quiet","type":"card","mirror":false,"title":"Both?","children":[{"id":"q1","type":"approval","prompt":"One?"},{"id":"q2","type":"approval","prompt":"Two?"}]}`
		bare     = `{"id":"bare","type":"approval"}`
	)
	blocks := func(raw ...string) []doc.Block {
		out := make([]doc.Block, 0, len(raw))
		for _, r := range raw {
			b, err := doc.DecodeBlock(json.RawMessage(r))
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, b)
		}
		return out
	}
	tests := []struct {
		name    string
		before  *sendBoard
		after   *sendBoard
		written []doc.Block
		want    []string
		nudges  []string
	}{
		{
			name:    "a new question goes",
			before:  writtenBoard(t, notes),
			after:   writtenBoard(t, notes+","+ship),
			written: blocks(ship),
			want:    []string{"ship"},
		},
		{
			name:    "an input goes",
			before:  &sendBoard{},
			after:   writtenBoard(t, name),
			written: blocks(name),
			want:    []string{"name"},
		},
		{
			name:    "a card that asks nothing stays quietly",
			before:  &sendBoard{},
			after:   writtenBoard(t, notes),
			written: blocks(notes),
		},
		{
			name:    "only what the write touched goes",
			before:  writtenBoard(t, ship),
			after:   writtenBoard(t, ship+","+notes),
			written: blocks(notes),
		},
		{
			name:    "a post that already says the same is left alone",
			before:  writtenBoard(t, shipped),
			after:   writtenBoard(t, shipped),
			written: blocks(shipped),
		},
		{
			name:    "a reworded question edits its post",
			before:  writtenBoard(t, shipped),
			after:   writtenBoard(t, reworded),
			written: blocks(reworded),
			want:    []string{"ship"},
		},
		{
			name:    "a child written into a card redraws that card",
			before:  writtenBoard(t, shipped),
			after:   writtenBoard(t, `{"id":"ship","type":"card","title":"Ship it?","slack":{"url":"https://example.slack.com/archives/D1/p1800000001000100"},"children":[{"id":"ship-why","type":"markdown","md":"CI is green."},{"id":"ship-q","type":"approval"}]}`),
			written: blocks(`{"id":"ship-why","type":"markdown","md":"CI is green."}`),
			want:    []string{"ship"},
		},
		{
			name:    "a multi-part card is nudged, not sent",
			before:  &sendBoard{},
			after:   writtenBoard(t, parts),
			written: blocks(parts),
			nudges:  []string{`"parts"`},
		},
		{
			name:    "a card that says mirror false stays quietly",
			before:  &sendBoard{},
			after:   writtenBoard(t, quiet),
			written: blocks(quiet),
		},
		{
			name:    "a question outside a card is nudged",
			before:  &sendBoard{},
			after:   writtenBoard(t, bare),
			written: blocks(bare),
			nudges:  []string{`"bare"`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			picked, nudges := writtenQuestions(tt.before, tt.after, tt.written)
			if got := ids(picked); !slices.Equal(got, tt.want) {
				t.Errorf("sent %v, want %v", got, tt.want)
			}
			if len(nudges) != len(tt.nudges) {
				t.Fatalf("nudges %q, want %q", nudges, tt.nudges)
			}
			for i, want := range tt.nudges {
				if !strings.Contains(nudges[i], want) {
					t.Errorf("nudge %q lacks %q", nudges[i], want)
				}
			}
		})
	}
}

func TestAnAnsweredQuestionIsNotSentAgain(t *testing.T) {
	const raw = `{"id":"ship","type":"card","title":"Ship it?","children":[{"id":"ship-q","type":"approval"}]},` +
		`{"id":"name","type":"card","title":"Which name?","children":[{"id":"name-q","type":"input","label":"The name"}]}`
	after := writtenBoard(t, raw)
	after.Interactions.Decisions = map[string]state.Decision{"ship-q": {Verdict: "approved"}}
	after.Interactions.Inputs = map[string]state.InputValue{"name-q": {Text: "Forge"}}
	picked, _ := writtenQuestions(&sendBoard{}, after, after.Doc.Blocks)
	if len(picked) != 0 {
		t.Fatalf("sent %v, want nothing: both were answered on the board", ids(picked))
	}
	after.Interactions.Decisions = map[string]state.Decision{"ship-q": {Verdict: "cleared"}}
	picked, _ = writtenQuestions(&sendBoard{}, after, after.Doc.Blocks)
	if got := ids(picked); !slices.Equal(got, []string{"ship"}) {
		t.Fatalf("sent %v, want ship: its verdict was cleared", got)
	}
}

const fakeCCSlack = `#!/bin/sh
log="$FAKE_CC_SLACK_DIR/args"
for arg in "$@"; do printf '%s\n' "$arg" >> "$log"; done
while [ $# -gt 0 ]; do
  if [ "$1" = "--file" ]; then cp "$2" "$FAKE_CC_SLACK_DIR/$(basename "$2")"; fi
  shift
done
printf '{"ts":"1800000001.000100","permalink":"https://example.slack.com/archives/D1/p1800000001000100"}'
`

func TestABoardCardReachesCCSlackWithItsButtonsAndImageBytes(t *testing.T) {
	dir := t.TempDir()
	//nolint:gosec // G306: the fake cc-slack on PATH has to be executable.
	if err := os.WriteFile(filepath.Join(dir, "cc-slack"), []byte(fakeCCSlack), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_CC_SLACK_DIR", dir)

	screenshot := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	sha := assets.SHA(screenshot)
	webp := []byte("RIFF\x00\x00\x00\x00WEBPVP8 ")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/assets/"+sha {
			_, _ = w.Write(screenshot)
			return
		}
		_, _ = w.Write(webp)
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}

	blk, err := doc.DecodeBlock(json.RawMessage(`{"id":"layout","type":"card","title":"Which layout?","children":[` +
		`{"id":"q","type":"choice","options":[` +
		`{"id":"a","label":"Feed first","recommended":true,"visual":{"id":"va","type":"image","src":"asset:` + sha + `","alt":"feed"}},` +
		`{"id":"b","label":"Pieces first","visual":{"id":"vb","type":"image","src":"asset:` + strings.Repeat("b", 64) + `","alt":"pieces"}}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	card := blk.(*doc.Card)
	q, err := slackcards.Check(card)
	if err != nil {
		t.Fatal(err)
	}
	var warn bytes.Buffer
	posted, err := postCard(t.Context(), "s1", port, card, q, "", &warn)
	if err != nil {
		t.Fatal(err)
	}
	if posted.TS != "1800000001.000100" || posted.Permalink == "" {
		t.Fatalf("posted = %+v", posted)
	}

	//nolint:gosec // G304: the test's own temp dir.
	raw, err := os.ReadFile(filepath.Join(dir, "args"))
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if !slices.Contains(args, "--mirrored") {
		t.Errorf("args %q lack --mirrored", args)
	}
	var sent slackcards.StatusCard
	if err := json.Unmarshal([]byte(args[slices.Index(args, "--card")+1]), &sent); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(sent.Options, []string{"Feed first", "Pieces first"}) || sent.Recommended != "Feed first" || sent.Waits != "layout" {
		t.Errorf("card %+v, want the board's answers as its buttons", sent)
	}
	//nolint:gosec // G304: the test's own temp dir.
	got, err := os.ReadFile(filepath.Join(dir, "Feed first.png"))
	if err != nil {
		t.Fatalf("the screenshot never reached cc-slack: %v (args %q)", err, args)
	}
	if !bytes.Equal(got, screenshot) {
		t.Errorf("cc-slack got %q, want the screenshot's bytes", got)
	}
	if n := strings.Count(string(raw), "--file\n"); n != 1 {
		t.Errorf("%d files sent, want the one Slack can show", n)
	}
	if !strings.Contains(warn.String(), `image "Pieces first" stays on the board`) {
		t.Errorf("warnings %q, want the WebP image named", warn.String())
	}
}

func TestADataImageDecodes(t *testing.T) {
	b, err := imageBytes(t.Context(), 0, "data:image/png;base64,iVBORw0KGgo=")
	if err != nil {
		t.Fatal(err)
	}
	if got := http.DetectContentType(b); got != "image/png" {
		t.Fatalf("data URI bytes sniff as %s", got)
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

func TestPRGroupIDsFindsEveryBlockAPushCarries(t *testing.T) {
	const group = `{"id":"pg","type":"pr-groups","repo":"a/b",%s"groups":[{"id":"g","title":"G","summary":"S.","prs":[{"number":8,"head":"abc1234","title":"T","change":"C.","plan":"P.","risk":"R.","review":"V.","blocker":"B."}]}]}`
	tests := []struct {
		name string
		raw  string
	}{
		{"top level", fmt.Sprintf(group, `"slack":"1792842193.708029",`)},
		{"inside a card", `{"id":"c","type":"card","title":"Merges","children":[` + fmt.Sprintf(group, "") + `]}`},
		{"stating no thread, so the board's binding decides", fmt.Sprintf(group, "")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blk, err := doc.DecodeBlock(json.RawMessage(tt.raw))
			if err != nil {
				t.Fatal(err)
			}
			if got := prGroupIDs([]doc.Block{blk}); !slices.Equal(got, []string{"pg"}) {
				t.Fatalf("ids = %v, want [pg]", got)
			}
		})
	}
}

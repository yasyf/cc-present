package slackcards_test

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/slackcards"
)

const permalink = "https://example.slack.com/archives/D1/p1800000001000100"

func card(t *testing.T, raw string) *doc.Card {
	t.Helper()
	blk, err := doc.DecodeBlock(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	c, ok := blk.(*doc.Card)
	if !ok {
		t.Fatalf("decoded a %s, want a card", blk.BlockType())
	}
	return c
}

func askCard(options string) string {
	return `{"id":"step-3:merge","type":"card","title":"Merge #41 now?","summary":"CI is green.","children":[{"id":"q","type":"choice","options":[` + options + `]}]}`
}

func TestCheckHoldsACardToTheRulesBothSurfacesShare(t *testing.T) {
	ok := `{"id":"o1","label":"Merge","recommended":true},{"id":"o2","label":"Hold"}`
	if _, err := slackcards.Check(card(t, askCard(ok))); err != nil {
		t.Fatalf("a card with one recommended answer: %v", err)
	}
	if _, err := slackcards.Check(card(t, askCard(`{"id":"o1","label":"Merge"},{"id":"o2","label":"Hold"}`))); err != nil {
		t.Fatalf("a card with no recommendation still asks a simple question: %v", err)
	}
	tests := map[string]string{
		"mirror false":        strings.Replace(askCard(ok), `"type":"card",`, `"type":"card","mirror":false,`, 1),
		"five answers":        askCard(`{"id":"o1","label":"A"},{"id":"o2","label":"B"},{"id":"o3","label":"C"},{"id":"o4","label":"D"},{"id":"o5","label":"E"}`),
		"two questions":       `{"id":"c","type":"card","title":"Both?","children":[{"id":"a","type":"approval"},{"id":"b","type":"input","label":"Why?"}]}`,
		"no words":            `{"id":"c","type":"card","children":[{"id":"a","type":"approval"}]}`,
		"two recommendations": askCard(`{"id":"o1","label":"Merge","recommended":true},{"id":"o2","label":"Hold","recommended":true}`),
		"an Other answer":     askCard(`{"id":"o1","label":"Merge","recommended":true},{"id":"o2","label":"Other"}`),
		"a marked label":      askCard(`{"id":"o1","label":"Merge (Recommended)","recommended":true}`),
		"a repeated answer":   askCard(`{"id":"o1","label":"Merge","recommended":true},{"id":"o2","label":"Merge"}`),
		"a multi-select":      strings.Replace(askCard(ok), `"type":"choice",`, `"type":"choice","multi":true,`, 1),
		"no question":         `{"id":"step-3:merge","type":"card","title":"Merge #41 now?","children":[]}`,
	}
	for name, raw := range tests {
		if _, err := slackcards.Check(card(t, raw)); err == nil {
			t.Errorf("%s: Check passed it", name)
		}
	}
}

func TestToSlackAsksTheSameQuestionAndLinksItsPRs(t *testing.T) {
	c := card(t, `{"id":"step-3:merge","type":"card","title":"Merge #41 now?","summary":"CI is green.","children":[`+
		`{"id":"pr","type":"pr","repo":"acme/app","number":41},`+
		`{"id":"q","type":"choice","options":[{"id":"o1","label":"Hold"},{"id":"o2","label":"Merge","recommended":true}]}]}`)
	ch, err := slackcards.Check(c)
	if err != nil {
		t.Fatal(err)
	}
	got := slackcards.ToSlack(c, ch)
	want := slackcards.StatusCard{
		Status: "needs_you", Title: "Merge #41 now?", Body: "CI is green.\n<https://github.com/acme/app/pull/41|app PR 41>",
		Options: []string{"Hold", "Merge"}, Recommended: "Merge", Waits: "step-3:merge",
		Refs: []slackcards.Ref{{URL: "https://github.com/acme/app/pull/41"}},
	}
	if g, w := string(mustJSON(t, got)), string(mustJSON(t, want)); g != w {
		t.Fatalf("ToSlack = %s\nwant %s", g, w)
	}
}

func TestFromSlackBuildsACardThatPassesCheckAndKeepsItsLinks(t *testing.T) {
	raw, err := slackcards.FromSlack(slackcards.Posted{
		Waits: "step-3:merge", Title: "Merge #41 now?", Body: "CI is green on <https://github.com/acme/app/pull/41|app PR 41>.",
		Options: []string{"Merge", "Hold"}, Recommended: "Merge", Permalink: permalink,
		PRs: []slackcards.PR{{Repo: "acme/app", Number: 41}},
	})
	if err != nil {
		t.Fatal(err)
	}
	c := card(t, string(raw))
	q, err := slackcards.Check(c)
	if err != nil {
		t.Fatalf("the mirrored card breaks the rules: %v", err)
	}
	ch := q.(*doc.Choice)
	if c.Slack == nil || c.Slack.URL != permalink {
		t.Fatalf("card links %+v, want the Slack permalink", c.Slack)
	}
	if ch.ID != slackcards.AnswerID("step-3:merge") || !ch.Options[0].Recommended || ch.Options[0].Label != "Merge" {
		t.Fatalf("question = %+v, want Merge recommended first", ch)
	}
	var context *doc.Markdown
	var prs []*doc.PR
	for _, child := range c.Children {
		switch b := child.(type) {
		case *doc.Markdown:
			context = b
		case *doc.PR:
			prs = append(prs, b)
		}
	}
	if context == nil || context.Md != "CI is green on [app PR 41](https://github.com/acme/app/pull/41)." {
		t.Errorf("context = %+v, want the Slack link as a markdown link", context)
	}
	if len(prs) != 1 || prs[0].Repo != "acme/app" || prs[0].Number != 41 {
		t.Errorf("PR cards = %+v, want acme/app 41", prs)
	}
}

func TestAnAnswerCrossesBetweenSurfacesUnchanged(t *testing.T) {
	c := card(t, askCard(`{"id":"o1","label":"Merge","recommended":true},{"id":"o2","label":"Hold"}`))
	q, _ := slackcards.Question(c)
	ch := q.(*doc.Choice)
	tests := []struct {
		label string
		other bool
		ids   []string
		text  string
	}{
		{"Hold", false, []string{"o2"}, ""},
		{"Merge after the freeze", true, []string{}, "Merge after the freeze"},
		{"Hold", true, []string{}, "Hold"},
	}
	for _, tt := range tests {
		ids, text := slackcards.Selection(ch, tt.label, tt.other)
		if !slices.Equal(ids, tt.ids) || text != tt.text {
			t.Errorf("Selection(%q, %v) = %v %q, want %v %q", tt.label, tt.other, ids, text, tt.ids, tt.text)
		}
		if label, other := slackcards.Label(ch, ids, text); label != tt.label || other != tt.other {
			t.Errorf("Label(%v, %q) = %q %v, want %q %v", ids, text, label, other, tt.label, tt.other)
		}
	}
}

func TestAnApprovalAndAnInputAskInSlack(t *testing.T) {
	approval := card(t, `{"id":"ship","type":"card","title":"Ship it?","children":[{"id":"a","type":"approval","prompt":"Approve the release"}]}`)
	q, err := slackcards.Check(approval)
	if err != nil {
		t.Fatal(err)
	}
	got := slackcards.ToSlack(approval, q)
	if !slices.Equal(got.Options, []string{"Approve", "Reject"}) || got.Recommended != "" || got.Body != "Approve the release" {
		t.Fatalf("approval = %+v, want Approve and Reject, with its prompt in the body", got)
	}

	input := card(t, `{"id":"name","type":"card","children":[{"id":"i","type":"input","label":"What should it be called?"}]}`)
	if q, err = slackcards.Check(input); err != nil {
		t.Fatal(err)
	}
	got = slackcards.ToSlack(input, q)
	if len(got.Options) != 0 || got.Title != "What should it be called?" || got.Waits != "name" {
		t.Fatalf("input = %+v, want its label as the question and no preset answers", got)
	}
}

func TestAnApprovalAnswerCrossesBetweenSurfaces(t *testing.T) {
	tests := []struct {
		label   string
		other   bool
		verdict string
		note    string
	}{
		{"Approve", false, "approved", ""},
		{"Reject", false, "rejected", ""},
		{"Only after the freeze", true, "rejected", "Only after the freeze"},
	}
	for _, tt := range tests {
		if verdict, note := slackcards.ApprovalVerdict(tt.label, tt.other); verdict != tt.verdict || note != tt.note {
			t.Errorf("ApprovalVerdict(%q, %v) = %q %q, want %q %q", tt.label, tt.other, verdict, note, tt.verdict, tt.note)
		}
	}
	if got := slackcards.ApprovalLabel("approved"); got != "Approve" {
		t.Errorf("ApprovalLabel(approved) = %q", got)
	}
	if got := slackcards.ApprovalLabel("rejected"); got != "Reject" {
		t.Errorf("ApprovalLabel(rejected) = %q", got)
	}
}

func TestALongAnswerIsCutForItsButtonAndStillNamesItsOption(t *testing.T) {
	long := strings.Repeat("Refuse with rebuild ", 5)
	c := card(t, askCard(`{"id":"o1","label":"`+long+`","hint":"keeps your line","recommended":true},{"id":"o2","label":"Hold","hint":"waits"}`))
	q, err := slackcards.Check(c)
	if err != nil {
		t.Fatal(err)
	}
	got := slackcards.ToSlack(c, q)
	button := got.Options[0]
	if n := utf8.RuneCountInString(button); n != 61 || !strings.HasSuffix(button, "…") || got.Recommended != button {
		t.Fatalf("button %q is %d characters, recommended %q; want it cut to 61 with an ellipsis", button, n, got.Recommended)
	}
	for _, want := range []string{"• *" + long + ":* keeps your line", "• *Hold:* waits"} {
		if !strings.Contains(got.Body, want) {
			t.Errorf("body %q lacks %q", got.Body, want)
		}
	}
	ch := q.(*doc.Choice)
	if ids, text := slackcards.Selection(ch, button, false); !slices.Equal(ids, []string{"o1"}) || text != "" {
		t.Errorf("a tap on the cut button selected %v %q, want o1", ids, text)
	}
	if label, other := slackcards.Label(ch, []string{"o1"}, ""); label != button || other {
		t.Errorf("a board pick answered Slack with %q %v, want the button's own words", label, other)
	}
}

func TestALongBodyIsCutToWhatSlackTakes(t *testing.T) {
	md := strings.Repeat("word ", 700)
	c := card(t, `{"id":"c","type":"card","title":"T?","children":[{"id":"m","type":"markdown","md":"`+md+`"},{"id":"a","type":"approval"}]}`)
	q, err := slackcards.Check(c)
	if err != nil {
		t.Fatal(err)
	}
	if n := utf8.RuneCountInString(slackcards.ToSlack(c, q).Body); n != 2800 {
		t.Fatalf("body is %d characters, want 2800", n)
	}
}

func TestImagesNamesEachPicture(t *testing.T) {
	sha := strings.Repeat("a", 64)
	c := card(t, `{"id":"layout","type":"card","title":"Which layout?","children":[`+
		`{"id":"now","type":"image","src":"asset:`+sha+`","alt":"the dashboard","caption":"Today"},`+
		`{"id":"plain","type":"image","src":"https://example.com/a.png","alt":"a chart"},`+
		`{"id":"q","type":"choice","options":[`+
		`{"id":"a","label":"Feed first","visual":{"id":"va","type":"image","src":"asset:`+sha+`","alt":"feed"}},`+
		`{"id":"b","label":"Pieces first"}]}]}`)
	want := []slackcards.Image{
		{Src: "asset:" + sha, Title: "Today"},
		{Src: "https://example.com/a.png", Title: "a chart"},
		{Src: "asset:" + sha, Title: "Feed first"},
	}
	if got := slackcards.Images(c); !slices.Equal(got, want) {
		t.Fatalf("Images = %+v, want %+v", got, want)
	}
}

func TestSameIsFalseWhenWhatSlackShowsChanges(t *testing.T) {
	base := `{"id":"c","type":"card","title":"Ship it?","children":[%s{"id":"a","type":"approval"}]}`
	plain := card(t, fmt.Sprintf(base, ""))
	if !slackcards.Same(plain, card(t, fmt.Sprintf(base, ""))) {
		t.Error("two identical cards differ")
	}
	if slackcards.Same(plain, card(t, fmt.Sprintf(base, `{"id":"m","type":"markdown","md":"New context."},`))) {
		t.Error("new context left the post unchanged")
	}
	if slackcards.Same(plain, card(t, fmt.Sprintf(base, `{"id":"i","type":"image","src":"https://example.com/a.png","alt":"a"},`))) {
		t.Error("a new image left the post unchanged")
	}
}

func TestNudgeNamesWhatStaysOffSlackUnlessTheAuthorSaidSo(t *testing.T) {
	block := func(raw string) doc.Block {
		b, err := doc.DecodeBlock(json.RawMessage(raw))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	twoParts := `{"id":"c","type":"card",%s"title":"Both?","children":[{"id":"a","type":"approval","prompt":"One?"},{"id":"b","type":"approval","prompt":"Two?"}]}`
	if got := slackcards.Nudge(block(fmt.Sprintf(twoParts, ""))); !strings.Contains(got, `"mirror": false`) {
		t.Errorf("a multi-part card: nudge %q, want it to name the mirror field", got)
	}
	if got := slackcards.Nudge(block(fmt.Sprintf(twoParts, `"mirror":false,`))); got != "" {
		t.Errorf("a card that says mirror false: nudge %q", got)
	}
	if got := slackcards.Nudge(block(`{"id":"q","type":"approval"}`)); !strings.Contains(got, "put it in a card") {
		t.Errorf("a bare question: nudge %q", got)
	}
	for _, simple := range []string{
		`{"id":"c","type":"card","title":"Ship?","children":[{"id":"a","type":"approval"}]}`,
		`{"id":"c","type":"card","title":"Notes","children":[{"id":"m","type":"markdown","md":"No question."}]}`,
		`{"id":"m","type":"markdown","md":"No question."}`,
	} {
		if got := slackcards.Nudge(block(simple)); got != "" {
			t.Errorf("%s: nudge %q", simple, got)
		}
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestASlackBodyCrossesToTheBoardAndBackUnchanged(t *testing.T) {
	body := "This also means:\n1. Nothing copies it to the laptop.\n2. See <https://github.com/acme/app/pull/41|app PR 41>."
	raw, err := slackcards.FromSlack(slackcards.Posted{Waits: "rule:5c0e9a1", Title: "Did I read it right?", Body: body, Options: []string{"Right", "Some wrong"}, Recommended: "Right", Permalink: permalink})
	if err != nil {
		t.Fatal(err)
	}
	c := card(t, string(raw))
	ch, err := slackcards.Check(c)
	if err != nil {
		t.Fatal(err)
	}
	if got := slackcards.ToSlack(c, ch).Body; got != body {
		t.Fatalf("ToSlack body = %q, want the Slack body back: %q", got, body)
	}
}

func TestStruckContextStaysOffTheSlackCard(t *testing.T) {
	c := card(t, `{"id":"step-3:merge","type":"card","title":"Merge #41 now?","children":[`+
		`{"id":"old","type":"markdown","md":"Wait for the freeze.","struck":true},{"id":"now","type":"markdown","md":"The freeze is over."},`+
		`{"id":"q","type":"choice","options":[{"id":"o1","label":"Merge","recommended":true},{"id":"o2","label":"Hold"}]}]}`)
	ch, err := slackcards.Check(c)
	if err != nil {
		t.Fatal(err)
	}
	if got := slackcards.ToSlack(c, ch).Body; got != "The freeze is over." {
		t.Fatalf("ToSlack body = %q, want only the live context", got)
	}
}

func TestALongTitleReachesSlackCutTo140CharactersWithAnEllipsis(t *testing.T) {
	ok := `{"id":"o1","label":"Merge","recommended":true}`
	long := card(t, askCard(ok))
	long.Title = "“" + strings.Repeat("é", 200) + "” right?"
	ch, err := slackcards.Check(long)
	if err != nil {
		t.Fatal(err)
	}
	got := slackcards.ToSlack(long, ch).Title
	if n := len([]rune(got)); n != 140 || !strings.HasSuffix(got, "…") || !strings.HasPrefix(got, "“éé") {
		t.Fatalf("a 209-character title reached Slack as %d characters: %q", n, got)
	}
	exact := card(t, askCard(ok))
	exact.Title = strings.Repeat("a", 140)
	if got := slackcards.ToSlack(exact, ch).Title; got != exact.Title {
		t.Fatalf("a 140-character title changed to %q", got)
	}
}

package slackcards_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

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
	tests := map[string]string{
		"no recommendation":   askCard(`{"id":"o1","label":"Merge"},{"id":"o2","label":"Hold"}`),
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
	ch, err := slackcards.Check(c)
	if err != nil {
		t.Fatalf("the mirrored card breaks the rules: %v", err)
	}
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
	ch, _ := slackcards.Question(c)
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

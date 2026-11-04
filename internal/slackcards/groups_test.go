package slackcards_test

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/slackcards"
	"github.com/yasyf/cc-present/internal/state"
)

func row(number int, head string, carried ...string) doc.PRRow {
	return doc.PRRow{
		Number: number, Head: head, Carried: carried, Title: "Fix the [login](https://x.test/l) <form>",
		Change: "Login works.", Plan: "Matches the plan.", Risk: "Low.", Review: "**Green**.", Blocker: "Nothing.",
	}
}

func merges(rows ...doc.PRRow) (*doc.PRGroups, doc.PRGroup) {
	group := doc.PRGroup{ID: "login", Title: "Login", Summary: "People can sign in again.", PRs: rows}
	thread := "1792842193.708029"
	return &doc.PRGroups{Repo: "acme/app", Slack: &thread, Groups: []doc.PRGroup{group}}, group
}

func rowIDs(sc slackcards.StatusCard) []string {
	ids := make([]string, 0, len(sc.Rows))
	for _, r := range sc.Rows {
		ids = append(ids, r.ID)
	}
	return ids
}

const decidedAt = "2026-10-27T00:34:57Z"

var decidedClock = time.Date(2026, 10, 27, 0, 34, 57, 0, time.UTC).Local().Format("Jan 2 3:04 PM")

func approvedAt(head string) state.Decision {
	return state.Decision{Verdict: "approved", Head: head, At: decidedAt}
}

func TestGroupCardAsksOnlyRowsWhoseVerdictDoesNotStand(t *testing.T) {
	tests := []struct {
		name     string
		row      doc.PRRow
		verdicts map[string]state.Decision
		want     []string
		wantBody string
	}{
		{"no verdict asks", row(41, "abc1234def"), nil, []string{"pg:41:abc1234def"}, ""},
		{"a verdict at the head reads as its verdict", row(41, "abc1234def"), map[string]state.Decision{"41": approvedAt("abc1234")}, nil, "Approved at abc1234 on " + decidedClock},
		{"a verdict at a carried head reads as its verdict", row(41, "abc1234def", "0001111"), map[string]state.Decision{"41": approvedAt("0001111")}, nil, "Approved at 0001111 on " + decidedClock},
		{"a hold reads with its note", row(41, "abc1234def"), map[string]state.Decision{"41": {Verdict: "rejected", Head: "abc1234", At: decidedAt, Note: "wait for **CI**"}}, nil, "Held at abc1234 on " + decidedClock + ": wait for *CI*"},
		{"a verdict at an old head asks again", row(41, "abc1234def"), map[string]state.Decision{"41": approvedAt("0001111")}, []string{"pg:41:abc1234def"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, group := merges(tt.row)
			g.ID = "pg"
			cards := slackcards.GroupCard(g, group, tt.verdicts, true, 0)
			if len(cards) != 1 {
				t.Fatalf("cards = %d, want 1", len(cards))
			}
			sc := cards[0]
			if got := rowIDs(sc); strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("rows = %v, want %v", got, tt.want)
			}
			if !strings.Contains(sc.Body, tt.wantBody) {
				t.Fatalf("body lacks %q:\n%s", tt.wantBody, sc.Body)
			}
			if sc.Group != "pg:login" {
				t.Fatalf("group = %q, want pg:login", sc.Group)
			}
			wantStatus := map[bool]string{true: "needs_you", false: "done"}[len(tt.want) > 0]
			if sc.Status != wantStatus {
				t.Fatalf("status = %q, want %q", sc.Status, wantStatus)
			}
		})
	}
}

func TestGroupCardRowCarriesTheFiveFactsInSlackMarkup(t *testing.T) {
	g, group := merges(row(41, "abc1234def"))
	g.ID = "pg"
	stale := map[string]state.Decision{"41": {Verdict: "rejected", Head: "0001111", At: decidedAt}}
	got := slackcards.GroupCard(g, group, stale, true, 0)[0].Rows[0]
	want := slackcards.CardPR{URL: "https://github.com/acme/app/pull/41", Title: "Fix the [login](https://x.test/l) <form>", Head: "abc1234def", Review: "**Green**.", Change: "Login works.", Plan: "Matches the plan.", Risk: "Low.", Blocks: "Nothing."}
	if got.PR == nil || !reflect.DeepEqual(*got.PR, want) {
		t.Errorf("row PR = %+v, want %+v, so cc-slack draws its link and four lines", got.PR, want)
	}
	if note := "_Held at 0001111 on " + decidedClock + "; new head abc1234 needs your tap._"; got.Text != note {
		t.Errorf("row text = %q, want %q", got.Text, note)
	}
}

func TestGroupCardsRetireAGroupThatLeftTheBoard(t *testing.T) {
	before, _ := merges(row(41, "abc1234def"), row(42, "fed4321abc"))
	before.ID = "pg"
	after := *before
	after.Groups = []doc.PRGroup{{ID: "signup", Title: "Signup", Summary: "People can sign up.", PRs: []doc.PRRow{row(43, "0123456789")}}}
	cards := slackcards.GroupCards(before, &after, map[string]state.Decision{"41": approvedAt("0001111")})
	if len(cards) != 2 || cards[0].Group != "pg:login" || cards[1].Group != "pg:signup" {
		t.Fatalf("cards = %+v, want the retired login card then the signup card", cards)
	}
	retired := cards[0]
	if len(retired.Rows) != 0 || retired.Status != "done" {
		t.Fatalf("retired card = %+v, want no buttons and done", retired)
	}
	for _, want := range []string{"app PR 41>* Fix the <https://x.test/l|login> &lt;form&gt; `abc1234` · Approved at 0001111", "app PR 42>* Fix the <https://x.test/l|login> &lt;form&gt; `fed4321` · left the board"} {
		if !strings.Contains(retired.Body, want) {
			t.Errorf("retired body lacks %q:\n%s", want, retired.Body)
		}
	}
}

func rows(n int) []doc.PRRow {
	prs := make([]doc.PRRow, 0, n)
	for i := range n {
		prs = append(prs, row(100+i, fmt.Sprintf("%010d", 100+i)))
	}
	return prs
}

type page struct {
	group, title, status string
	rows                 int
}

func pages(cards []slackcards.StatusCard) []page {
	got := make([]page, 0, len(cards))
	for _, c := range cards {
		got = append(got, page{c.Group, c.Title, c.Status, len(c.Rows)})
	}
	return got
}

func TestGroupCardPagesEightRowsACard(t *testing.T) {
	tests := []struct {
		name     string
		prs      int
		verdicts map[string]state.Decision
		held     int
		want     []page
	}{
		{"eight rows fit one card", 8, nil, 0, []page{{"pg:login", "Login", "needs_you", 8}}},
		{"nine rows page onto two cards", 9, nil, 0, []page{{"pg:login", "Login (1 of 2)", "needs_you", 8}, {"pg:login:2", "Login (2 of 2)", "needs_you", 1}}},
		{"a tap that settles a row empties the second card", 9, map[string]state.Decision{"100": approvedAt("0000000100")}, 0, []page{{"pg:login", "Login", "needs_you", 8}, {"pg:login:2", "Login", "done", 0}}},
		{"rows that left the group empty the second card", 7, nil, 9, []page{{"pg:login", "Login", "needs_you", 7}, {"pg:login:2", "Login", "done", 0}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, group := merges(rows(tt.prs)...)
			g.ID = "pg"
			if got := pages(slackcards.GroupCard(g, group, tt.verdicts, true, tt.held)); !slices.Equal(got, tt.want) {
				t.Fatalf("cards = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestGroupCardsEmptyThePageAGroupNoLongerFills(t *testing.T) {
	before, _ := merges(rows(9)...)
	before.ID = "pg"
	after := *before
	after.Groups = []doc.PRGroup{{ID: "login", Title: "Login", PRs: rows(8)}}
	want := []page{{"pg:login", "Login", "needs_you", 8}, {"pg:login:2", "Login", "done", 0}}
	if got := pages(slackcards.GroupCards(before, &after, nil)); !slices.Equal(got, want) {
		t.Fatalf("cards = %+v, want %+v", got, want)
	}
}

func TestDecidedNamesTheBoardVerdict(t *testing.T) {
	tests := []struct {
		name, row, label             string
		block, number, head, verdict string
	}{
		{"approve", "pg:41:abc1234", "Approve", "pg", "41", "abc1234", "approved"},
		{"hold", "pg:41:abc1234", "Hold", "pg", "41", "abc1234", "rejected"},
		{"a block id with colons", "step:3:merges:41:abc1234", "Approve", "step:3:merges", "41", "abc1234", "approved"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block, number, head, verdict, err := slackcards.Decided{Row: tt.row, Label: tt.label}.Verdict()
			if err != nil {
				t.Fatal(err)
			}
			if block != tt.block || number != tt.number || head != tt.head || verdict != tt.verdict {
				t.Fatalf("verdict = %s/%s@%s %s, want %s/%s@%s %s", block, number, head, verdict, tt.block, tt.number, tt.head, tt.verdict)
			}
		})
	}
	for name, d := range map[string]slackcards.Decided{
		"another label": {Row: "pg:41:abc1234", Label: "Merge"},
		"no head":       {Row: "41", Label: "Approve"},
		"no block":      {Row: ":41:abc1234", Label: "Approve"},
		"a short head":  {Row: "pg:41:a", Label: "Approve"},
	} {
		if _, _, _, _, err := d.Verdict(); err == nil {
			t.Errorf("%s: Verdict accepted %+v", name, d)
		}
	}
}

func TestGroupCardRowNamesTheStackItsTapDecides(t *testing.T) {
	r := row(41, "abc1234def")
	r.Stack = []doc.StackMember{{Number: 39, Head: "1112223aaa"}, {Number: 40, Head: "4445556bbb"}}
	g, group := merges(r)
	g.ID = "pg"
	cards := slackcards.GroupCard(g, group, nil, true, 0)
	want := "*One tap also decides:* <https://github.com/acme/app/pull/39|#39> `1112223`, <https://github.com/acme/app/pull/40|#40> `4445556`"
	if text := cards[0].Rows[0].Text; !strings.Contains(text, want) {
		t.Fatalf("row text lacks %q:\n%s", want, text)
	}
}

func TestGroupCardsWriteAGroupOnlyWhileItAsksAndChanged(t *testing.T) {
	before, _ := merges(row(41, "abc1234def"), row(42, "fed4321abc"))
	before.ID = "pg"
	reworded := *before
	reworded.Groups = []doc.PRGroup{before.Groups[0]}
	reworded.Groups[0].Summary = "Everyone can sign in again."
	approved := map[string]state.Decision{"41": approvedAt("abc1234def"), "42": approvedAt("fed4321abc")}
	tests := []struct {
		name     string
		after    *doc.PRGroups
		verdicts map[string]state.Decision
		want     []page
	}{
		{"an unchanged group writes nothing", before, nil, []page{}},
		{"a changed group that asks redraws", &reworded, nil, []page{{"pg:login", "Login", "needs_you", 2}}},
		{"a settled group writes nothing more", &reworded, approved, []page{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pages(slackcards.GroupCards(before, tt.after, tt.verdicts)); !slices.Equal(got, tt.want) {
				t.Fatalf("cards = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestARetiredGroupCardLeavesOutAPRThatMovedGroups(t *testing.T) {
	before, _ := merges(row(41, "abc1234def"), row(42, "fed4321abc"))
	before.ID = "pg"
	after := *before
	after.Groups = []doc.PRGroup{{ID: "signup", Title: "Signup", PRs: []doc.PRRow{row(42, "fed4321abc"), row(43, "0123456789")}}}
	cards := slackcards.GroupCards(before, &after, nil)
	if len(cards) != 2 || cards[0].Group != "pg:login" {
		t.Fatalf("cards = %+v, want the retired login card then the signup card", cards)
	}
	if retired := cards[0].Body; !strings.Contains(retired, "app PR 41>") || strings.Contains(retired, "app PR 42>") {
		t.Fatalf("retired body = %q, want PR 41 alone: PR 42 lives on in Signup", retired)
	}
}

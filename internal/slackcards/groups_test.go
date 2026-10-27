package slackcards_test

import (
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
	return &doc.PRGroups{Repo: "acme/app", Slack: "1792842193.708029", Groups: []doc.PRGroup{group}}, group
}

func rowIDs(sc slackcards.StatusCard) []string {
	ids := make([]string, 0, len(sc.Rows))
	for _, r := range sc.Rows {
		ids = append(ids, r.ID)
	}
	return ids
}

func TestGroupCardAsksOnlyRowsWhoseVerdictDoesNotStand(t *testing.T) {
	approvedAt := func(head string) state.Decision {
		return state.Decision{Verdict: "approved", Head: head, At: "2026-10-27T00:34:57Z"}
	}
	tests := []struct {
		name     string
		row      doc.PRRow
		verdicts map[string]state.Decision
		want     []string
	}{
		{"no verdict asks", row(41, "abc1234def"), nil, []string{"pg:41:abc1234def"}},
		{"a verdict at the head leaves", row(41, "abc1234def"), map[string]state.Decision{"41": approvedAt("abc1234")}, nil},
		{"a verdict at a carried head leaves", row(41, "abc1234def", "0001111"), map[string]state.Decision{"41": approvedAt("0001111")}, nil},
		{"a verdict at an old head asks again", row(41, "abc1234def"), map[string]state.Decision{"41": approvedAt("0001111")}, []string{"pg:41:abc1234def"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, group := merges(tt.row)
			g.ID = "pg"
			sc := slackcards.GroupCard(g, group, tt.verdicts)
			if got := rowIDs(sc); strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("rows = %v, want %v", got, tt.want)
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
	stale := map[string]state.Decision{"41": {Verdict: "rejected", Head: "0001111", At: "2026-10-27T00:34:57Z"}}
	text := slackcards.GroupCard(g, group, stale).Rows[0].Text
	for _, want := range []string{
		"*<https://github.com/acme/app/pull/41|app PR 41>* Fix the <https://x.test/l|login> &lt;form&gt; `abc1234`",
		"*What changes:* Login works.", "*Against the plan:* Matches the plan.", "*Risk:* Low.",
		"*Review and CI:* *Green*.", "*Blocks landing:* Nothing.",
		"_Held at 0001111 on " + time.Date(2026, 10, 27, 0, 34, 57, 0, time.UTC).Local().Format("Jan 2 3:04 PM") + "; new head abc1234 needs your tap._",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("row text lacks %q:\n%s", want, text)
		}
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

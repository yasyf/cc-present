package slackcards_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/slackcards"
	"github.com/yasyf/cc-present/internal/state"
)

func triageCard(id, triageID string, items ...string) string {
	rows := make([]string, len(items))
	for i, item := range items {
		rows[i] = fmt.Sprintf(`{"id":%q,"label":"Device %s","hint":"Seen **today**"}`, item, item)
	}
	return fmt.Sprintf(`{"id":%q,"type":"card","title":"Tag or remove each device?","summary":"Five devices joined.","children":[{"id":%q,"type":"triage","items":[%s]}]}`,
		id, triageID, strings.Join(rows, ","))
}

func TestCheckAsksATriageCardRowByRowWithinSlacksIDs(t *testing.T) {
	q, err := slackcards.Check(card(t, triageCard("devices", "tr", "mbp-1", "ipad.2")))
	if err != nil {
		t.Fatalf("a triage card: %v", err)
	}
	if _, ok := q.(*doc.Triage); !ok {
		t.Fatalf("question = %T, want the triage", q)
	}
	tests := map[string]string{
		"an item id Slack cannot name":     triageCard("devices", "tr", "mac book"),
		"a triage id with a colon":         triageCard("devices", "tr:1", "mbp-1"),
		"a row id past 64 characters":      triageCard("devices", "tr", strings.Repeat("d", 62)),
		"a card id with no room for pages": triageCard(strings.Repeat("c", 63), "tr", "mbp-1"),
	}
	for name, raw := range tests {
		if _, err := slackcards.Check(card(t, raw)); err == nil {
			t.Errorf("%s: Check passed it", name)
		}
	}
}

func TestCardsAsksEachOpenItemAsItsOwnRowAndNamesEachDecidedOne(t *testing.T) {
	c := card(t, triageCard("devices", "tr", "mbp-1", "ipad-2", "pi-3"))
	q, err := slackcards.Check(c)
	if err != nil {
		t.Fatal(err)
	}
	fresh := slackcards.Cards(c, q, nil, nil)
	if len(fresh) != 1 || fresh[0].Group != "devices" || fresh[0].Waits != "" || fresh[0].Status != "needs_you" {
		t.Fatalf("cards = %+v, want one group card named for the card", fresh)
	}
	if got := rowIDs(fresh[0]); !slices.Equal(got, []string{"tr:mbp-1", "tr:ipad-2", "tr:pi-3"}) {
		t.Fatalf("rows = %q, want one per item", got)
	}
	if r := fresh[0].Rows[0]; !slices.Equal(r.Buttons, []string{"Approve", "Reject"}) || r.Text != "*Device mbp-1*\nSeen *today*" {
		t.Errorf("row = %+v, want the board's Approve and Reject under the item's label and hint", r)
	}
	if !strings.Contains(fresh[0].Body, "Five devices joined.") {
		t.Errorf("body = %q, want the card's summary", fresh[0].Body)
	}

	partial := slackcards.Cards(c, q, map[string]state.Decision{"ipad-2": {Verdict: "rejected", Note: "not **ours**"}}, nil)
	if got := rowIDs(partial[0]); !slices.Equal(got, []string{"tr:mbp-1", "tr:pi-3"}) || partial[0].Status != "needs_you" {
		t.Fatalf("rows = %q status %q, want the two open items still asking", got, partial[0].Status)
	}
	if !strings.Contains(partial[0].Body, "*Device ipad-2* · Rejected: not *ours*") {
		t.Errorf("body = %q, want the decided item named with its verdict and note", partial[0].Body)
	}

	all := map[string]state.Decision{"mbp-1": {Verdict: "approved"}, "ipad-2": {Verdict: "rejected"}, "pi-3": {Verdict: "approved"}}
	if done := slackcards.Cards(c, q, all, nil); len(done[0].Rows) != 0 || done[0].Status != "done" {
		t.Errorf("card = %+v, want no buttons once every item has a verdict", done[0])
	}
}

func TestATriagePastTwentyRowsAsksOnOneCardPerTwenty(t *testing.T) {
	items := make([]string, doc.MaxTriageItems)
	for i := range items {
		items[i] = fmt.Sprintf("d%d", i+1)
	}
	c := card(t, triageCard("devices", "tr", items...))
	q, err := slackcards.Check(c)
	if err != nil {
		t.Fatal(err)
	}
	cards := slackcards.Cards(c, q, nil, nil)
	groups, titles := make([]string, 0, len(cards)), make([]string, 0, len(cards))
	rows := 0
	for _, sc := range cards {
		groups, titles, rows = append(groups, sc.Group), append(titles, sc.Title), rows+len(sc.Rows)
	}
	if !slices.Equal(groups, []string{"devices", "devices:2", "devices:3"}) || rows != doc.MaxTriageItems {
		t.Fatalf("groups %q with %d rows, want three pages carrying every item", groups, rows)
	}
	if titles[2] != "Tag or remove each device? (3 of 3)" || len(cards[1].Rows) != 20 || len(cards[2].Rows) != 10 {
		t.Errorf("titles %q, want each page numbered and twenty rows a page", titles)
	}
}

func TestATriageRowTapIsItsItemsVerdict(t *testing.T) {
	block, item, verdict, err := slackcards.Decided{Row: "tr:ipad.2", Label: "Reject"}.Item()
	if err != nil || block != "tr" || item != "ipad.2" || verdict != "rejected" {
		t.Fatalf("Item() = %q %q %q %v, want tr ipad.2 rejected", block, item, verdict, err)
	}
	if _, _, _, err := (slackcards.Decided{Row: "tr:ipad.2", Label: "Hold"}).Item(); err == nil {
		t.Error("a Hold tap on a triage row must be refused")
	}
}

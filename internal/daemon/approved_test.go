package daemon

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yasyf/cc-present/internal/doc"
)

func approvedBoard(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	if reply := handleStart(h.hc(body{Doc: json.RawMessage(prGroupsDoc)}), doc.NoPacks, nilDisplay); !reply.OK {
		t.Fatalf("start: %s", reply.Error)
	}
	tap := json.RawMessage(`[{"block":"pg","row":"8","verdict":"approved","at":"2026-10-31T09:30:00Z"}]`)
	if reply := handleImportVerdicts(h.hc(body{Verdicts: tap})); !reply.OK {
		t.Fatalf("import the owner's approval: %s", reply.Error)
	}
	return h
}

func movedHead(row string) string {
	return strings.Replace(prGroupsDoc, `"number":8,"head":"abc1234"`, `"number":8,"head":"1112223"`+row, 1)
}

func TestAPushNeverPutsAnApprovedPullRequestBackInFrontOfTheOwner(t *testing.T) {
	t.Run("a row at a later head the approval does not stand at is refused", func(t *testing.T) {
		h := approvedBoard(t)
		reply := handlePush(h.hc(body{Doc: json.RawMessage(movedHead(""))}), doc.NoPacks, nilDisplay)
		if reply.OK {
			t.Fatal("push of an approved pull request at a new head was admitted")
		}
		for _, want := range []string{"#8 was approved at abc1234", "--owner-reask 8"} {
			if !strings.Contains(reply.Error, want) {
				t.Fatalf("refusal %q lacks %q", reply.Error, want)
			}
		}
	})
	t.Run("update-block is refused the same way", func(t *testing.T) {
		h := approvedBoard(t)
		var d struct {
			Blocks []json.RawMessage `json:"blocks"`
		}
		if err := json.Unmarshal([]byte(movedHead("")), &d); err != nil {
			t.Fatal(err)
		}
		if reply := handleUpsertBlock(h.hc(body{Block: d.Blocks[0]}), doc.NoPacks); reply.OK || !strings.Contains(reply.Error, "--owner-reask 8") {
			t.Fatalf("update-block ok=%v err=%q, want an --owner-reask 8 refusal", reply.OK, reply.Error)
		}
	})
	t.Run("a row carrying the approved head is admitted", func(t *testing.T) {
		h := approvedBoard(t)
		if reply := handlePush(h.hc(body{Doc: json.RawMessage(movedHead(`,"carried":["abc1234"]`))}), doc.NoPacks, nilDisplay); !reply.OK {
			t.Fatalf("push carrying the approval: %s", reply.Error)
		}
	})
	t.Run("an owner reask with a reason puts it back", func(t *testing.T) {
		h := approvedBoard(t)
		reask := Reask{PRs: []string{"8"}, Reason: "the owner asked to see the new head"}
		if reply := handlePush(h.hc(body{Doc: json.RawMessage(movedHead("")), Reask: reask}), doc.NoPacks, nilDisplay); !reply.OK {
			t.Fatalf("owner reask: %s", reply.Error)
		}
	})
	t.Run("an owner reask with no reason is refused", func(t *testing.T) {
		h := approvedBoard(t)
		reply := handlePush(h.hc(body{Doc: json.RawMessage(movedHead("")), Reask: Reask{PRs: []string{"8"}}}), doc.NoPacks, nilDisplay)
		if reply.OK || !strings.Contains(reply.Error, "--owner-reask needs --reason") {
			t.Fatalf("reask without reason ok=%v err=%q", reply.OK, reply.Error)
		}
	})
}

func TestAVerdictWriteNeverLeavesAnApprovedRowAskingAgain(t *testing.T) {
	h := approvedBoard(t)
	moves := json.RawMessage(`[{"from":{"block":"pg","row":"8"},"to":{"block":"tr1","row":"i1"}}]`)
	reply := handleMoveVerdicts(h.hc(body{Verdicts: moves}))
	if reply.OK || !strings.Contains(reply.Error, `would put its row on "pg" back in front of the owner`) {
		t.Fatalf("move clearing an approved row ok=%v err=%q", reply.OK, reply.Error)
	}
	copies := json.RawMessage(`[{"from":{"block":"pg","row":"8"},"to":{"block":"tr1","row":"i1"},"keep":true}]`)
	if reply := handleMoveVerdicts(h.hc(body{Verdicts: copies})); !reply.OK {
		t.Fatalf("copy keeping the approval: %s", reply.Error)
	}
	carry := json.RawMessage(`{"block":"pg","row":"8","head":"9998887","note":"a replay"}`)
	if reply := handleCarryVerdict(h.hc(body{Verdicts: carry})); reply.OK || !strings.Contains(reply.Error, "#8 was approved at abc1234") {
		t.Fatalf("carry to a head the row does not stand at ok=%v err=%q", reply.OK, reply.Error)
	}
}

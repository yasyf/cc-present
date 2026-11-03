package state

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestWritesMatchBetweenAFullReplayAndACatchUp(t *testing.T) {
	event := func(seq int64, kind, payload string) Event {
		identified := fmt.Sprintf(`{"schemaVersion":%d,"type":%q,`, SchemaVersion, kind) + payload[1:]
		return Event{Seq: seq, Type: kind, Payload: json.RawMessage(identified)}
	}
	events := []Event{
		event(1, "doc.replaced", `{"revision":1,"lane":"a","doc":{"version":1,"title":"T","blocks":[{"id":"c1","type":"card","children":[]},{"id":"m1","type":"markdown","md":"x"}]}}`),
		event(2, "block.upserted", `{"lane":"b","takeOver":true,"block":{"id":"c1","type":"card","title":"New","children":[]}}`),
		event(3, "block.upserted", `{"lane":"c","keepOwner":true,"block":{"id":"c1","type":"card","title":"Newer","children":[]}}`),
		event(4, "doc.replaced", `{"revision":2,"lane":"d","retained":["c1"],"doc":{"version":1,"title":"Renamed","blocks":[{"id":"c1","type":"card","title":"Newer","children":[]}]}}`),
	}
	full, err := Reduce(events)
	if err != nil {
		t.Fatal(err)
	}
	split := New()
	for _, ev := range events {
		if err := split.Apply([]Event{ev}); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := fmt.Sprint(split.writes, split.docWrite), fmt.Sprint(full.writes, full.docWrite); got != want {
		t.Fatalf("catch-up writes = %s, full replay = %s", got, want)
	}
	if card := full.writes["c1"]; card.Owner != "b" || card.TakenFrom != "a" || card.Lane != "c" {
		t.Fatalf("c1 = %+v, want owner b, taken from a, last written by c", card)
	}
	if gone := full.writes["m1"]; !gone.Removed || gone.Lane != "d" {
		t.Fatalf("m1 = %+v, want removed by d", gone)
	}
	if full.docWrite.Rev != 4 || full.docWrite.Lane != "d" {
		t.Fatalf("docWrite = %+v, want the rename at rev 4 by d", full.docWrite)
	}
}

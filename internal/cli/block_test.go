package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	ccdaemon "github.com/yasyf/cc-present/internal/daemon"
)

func TestWriteBlockShow(t *testing.T) {
	at := time.Date(2026, 10, 28, 18, 42, 0, 0, time.Local)
	show := ccdaemon.BlockShow{
		Top: json.RawMessage(`{"id":"c1","type":"card","title":"Route?","summary":"Pick a layer.","children":[` +
			`{"id":"q1","type":"choice","prompt":"Which?","options":[{"id":"a","label":"A","hint":"local first","recommended":true},{"id":"b","label":"B"}]},` +
			`{"id":"ok","type":"approval","prompt":"Ship?"}]}`),
		Events: []ccdaemon.BlockEvent{
			{Type: ccdaemon.EventDecisionCreated, Origin: "agent", At: at, Payload: json.RawMessage(`{"blockId":"ok","verdict":"approved","note":"carried"}`)},
			{Type: ccdaemon.EventChoiceSelected, Origin: "human", At: at, Payload: json.RawMessage(`{"blockId":"q1","optionIds":[],"other":"same layer\nas before"}`)},
		},
	}
	var out strings.Builder
	if err := writeBlockShow(&out, "q1", show); err != nil {
		t.Fatal(err)
	}
	stamp := at.Format("Jan 2 3:04 PM MST")
	want := `c1: Route?
  Pick a layer.
  No longer on the board; shown as it last was.

q1: choice, pick one
  Which?
  - a: A (recommended)
    local first
  - b: B

Answers, newest first:
  ` + stamp + `  ok  approved: carried (recorded by the agent)
  ` + stamp + `  q1  wrote in: same layer
    as before
`
	if out.String() != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out.String(), want)
	}
}

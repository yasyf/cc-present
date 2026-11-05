package liveread_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/liveread"
	"github.com/yasyf/cc-present/internal/state"
)

func command(t *testing.T, script string) []string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "reader")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return []string{path}
}

func TestAHostWithNoCommandSaysSo(t *testing.T) {
	_, err := liveread.Read(context.Background(), nil, nil, nil)
	if !errors.Is(err, liveread.ErrNoCommand) {
		t.Fatalf("got %v", err)
	}
}

func TestOneCommandRunAnswersEveryBindingAtAnyN(t *testing.T) {
	for _, n := range []int{1, 1000} {
		runs := filepath.Join(t.TempDir(), "runs")
		reader := command(t, `echo run >> `+runs+`
python3 -c 'import json,sys; print(json.dumps([{"items":[],"total":0} for _ in json.load(sys.stdin)["bindings"]]))'`)
		bindings := make([]liveread.Binding, n)
		for i := range bindings {
			bindings[i] = liveread.Binding{View: "embed", Refs: []string{"pr:" + strconv.Itoa(i)}}
		}
		results, err := liveread.Read(context.Background(), reader, bindings, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != n {
			t.Fatalf("got %d results for %d bindings", len(results), n)
		}
		seen, err := os.ReadFile(runs)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(string(seen), "run") != 1 {
			t.Fatalf("at N=%d the command ran %q", n, seen)
		}
	}
}

func TestAShortAnswerIsAnError(t *testing.T) {
	reader := command(t, `echo '[]'`)
	_, err := liveread.Read(context.Background(), reader, []liveread.Binding{{View: "count", Source: "prs"}}, nil)
	if err == nil || !strings.Contains(err.Error(), "answered 0 bindings of 1") {
		t.Fatalf("got %v", err)
	}
}

func TestAFailedCommandSaysWhy(t *testing.T) {
	reader := command(t, `echo 'no source "bugs"' >&2; exit 3`)
	_, err := liveread.Read(context.Background(), reader, []liveread.Binding{{View: "count", Source: "bugs"}}, nil)
	if err == nil || !strings.Contains(err.Error(), `no source "bugs"`) {
		t.Fatalf("got %v", err)
	}
}

func TestCardsCarryTheAnswerAsThePickedWords(t *testing.T) {
	var d doc.Doc
	if err := json.Unmarshal([]byte(`{"version":1,"title":"t","blocks":[
		{"id":"card-open","type":"card","title":"Open?","children":[{"id":"q1","type":"choice","options":[{"id":"a","label":"Yes"},{"id":"b","label":"No"}]}]},
		{"id":"card-done","type":"card","title":"Done?","children":[{"id":"q2","type":"choice","options":[{"id":"c","label":"Read in place"},{"id":"d","label":"Copy"}]}]},
		{"id":"card-typed","type":"card","title":"Typed?","children":[{"id":"q3","type":"choice","options":[{"id":"e","label":"One"},{"id":"f","label":"Two"}]}]},
		{"id":"card-note","type":"card","title":"Note","children":[{"id":"m","type":"markdown","md":"words"}]}
	]}`), &d); err != nil {
		t.Fatal(err)
	}
	st := &state.State{Doc: &d, Interactions: state.Interactions{Choices: map[string]state.Selection{
		"q2": {OptionIDs: []string{"c"}},
		"q3": {Other: "Neither"},
	}}}
	cards := liveread.Cards(st)
	if len(cards) != 3 {
		t.Fatalf("a card that asks nothing is no answerable card: %+v", cards)
	}
	if cards[0].Picked != "" || len(cards[0].Options) != 2 {
		t.Fatalf("open: %+v", cards[0])
	}
	if cards[1].Picked != "Read in place" {
		t.Fatalf("done: %+v", cards[1])
	}
	if cards[2].Picked != "Neither" {
		t.Fatalf("typed: %+v", cards[2])
	}
}

// Package liveread gets a live block's rows from the host's reader command, so a
// board stores what to read and never a copy of it.
package liveread

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/state"
)

// ErrNoCommand is Read's answer on a host that names no live command.
var ErrNoCommand = errors.New("no liveCommand in the host config, so a live block has nothing to read from")

// Binding is what a live block stores, as the command reads it.
type Binding struct {
	View    string            `json:"view"`
	Refs    []string          `json:"refs,omitempty"`
	Source  string            `json:"source,omitempty"`
	Where   map[string]string `json:"where,omitempty"`
	Columns []string          `json:"columns,omitempty"`
	Limit   int               `json:"limit,omitempty"`
}

// Card is a board card as the board holds it, so the command never reads the
// board back. Picked is empty until the human answers.
type Card struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Picked  string   `json:"picked,omitempty"`
	Options []string `json:"options,omitempty"`
}

// Item is one row of a live block.
type Item struct {
	Ref     string            `json:"ref"`
	Title   string            `json:"title"`
	State   string            `json:"state"`
	Says    string            `json:"says"`
	Cells   map[string]string `json:"cells,omitempty"`
	Actions []string          `json:"actions,omitempty"`
}

// Result is one binding's rows. Total counts what matched before its limit.
type Result struct {
	Items []Item `json:"items"`
	Total int    `json:"total"`
}

// Bindings is what each live block in blocks stores, in order.
func Bindings(blocks []*doc.Live) []Binding {
	bindings := make([]Binding, len(blocks))
	for i, b := range blocks {
		bindings[i] = Binding{View: b.View, Refs: b.Refs, Source: b.Source, Where: b.Where, Columns: b.Columns, Limit: b.Limit}
	}
	return bindings
}

// Cards is every card on the board that asks one choice, with its answer.
func Cards(st *state.State) []Card {
	var cards []Card
	for _, block := range st.Doc.Blocks {
		card, isCard := block.(*doc.Card)
		if !isCard {
			continue
		}
		for _, child := range card.Children {
			choice, isChoice := child.(*doc.Choice)
			if isChoice {
				cards = append(cards, cardOf(card, choice, st.Interactions.Choices[choice.ID]))
				break
			}
		}
	}
	return cards
}

func cardOf(card *doc.Card, choice *doc.Choice, picked state.Selection) Card {
	out := Card{ID: card.ID, Title: card.Title}
	labels := make(map[string]string, len(choice.Options))
	for _, option := range choice.Options {
		labels[option.ID] = option.Label
		out.Options = append(out.Options, option.Label)
	}
	switch {
	case picked.Other != "":
		out.Picked = picked.Other
	case len(picked.OptionIDs) > 0:
		out.Picked = labels[picked.OptionIDs[0]]
	}
	return out
}

// HostCommand is the liveCommand of the host config under stateDir, read fresh so
// setting it needs no daemon restart.
func HostCommand(stateDir string) ([]string, error) {
	raw, err := os.ReadFile(filepath.Join(stateDir, "config.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var config struct {
		LiveCommand []string `json:"liveCommand"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return nil, fmt.Errorf("parse host config: %w", err)
	}
	return config.LiveCommand, nil
}

// Read asks command once for every binding, however many there are.
func Read(ctx context.Context, command []string, bindings []Binding, cards []Card) ([]Result, error) {
	if len(command) == 0 {
		return nil, ErrNoCommand
	}
	input, err := json.Marshal(struct {
		Bindings []Binding `json:"bindings"`
		Cards    []Card    `json:"cards"`
	}{bindings, cards})
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Stdin = bytes.NewReader(input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %s", command[0], err, strings.TrimSpace(stderr.String()))
	}
	var results []Result
	if err := json.Unmarshal(out, &results); err != nil {
		return nil, fmt.Errorf("decode %s: %w", command[0], err)
	}
	if len(results) != len(bindings) {
		return nil, fmt.Errorf("%s answered %d bindings of %d", command[0], len(results), len(bindings))
	}
	return results, nil
}

package doc_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/yasyf/cc-present/internal/doc"
)

const locationDoc = `{
  "version": 1,
  "title": "Locations",
  "blocks": [
    {
      "id": "top-choice",
      "type": "choice",
      "options": [
        {
          "id": "top-option",
          "label": "Top",
          "visual": {"id": "top-visual", "type": "code", "lang": "go", "code": "package top"}
        }
      ]
    },
    {
      "id": "card",
      "type": "card",
      "children": [
        {"id": "child-a", "type": "markdown", "md": "A"},
        {
          "id": "child-choice",
          "type": "choice",
          "options": [
            {
              "id": "child-option",
              "label": "Child",
              "visual": {"id": "child-visual", "type": "code", "lang": "go", "code": "package child"}
            }
          ]
        },
        {
          "id": "child-triage",
          "type": "triage",
          "items": [
            {
              "id": "child-item",
              "label": "Child item",
              "visual": {"id": "child-item-visual", "type": "code", "lang": "go", "code": "package childitem"}
            }
          ]
        }
      ]
    },
    {
      "id": "top-triage",
      "type": "triage",
      "items": [
        {
          "id": "top-item",
          "label": "Top item",
          "visual": {"id": "top-item-visual", "type": "code", "lang": "go", "code": "package topitem"}
        }
      ]
    }
  ]
}`

const transformDoc = `{
  "version": 1,
  "title": "Transforms",
  "blocks": [
    {"id": "before", "type": "markdown", "md": "Before"},
    {
      "id": "card",
      "type": "card",
      "children": [
        {"id": "child-a", "type": "markdown", "md": "A"},
        {"id": "child-b", "type": "markdown", "md": "B"}
      ]
    },
    {"id": "after", "type": "markdown", "md": "After"}
  ]
}`

func TestLocate(t *testing.T) {
	d := mustDocument(t, locationDoc)
	tests := []struct {
		name       string
		id         string
		found      bool
		kind       doc.LocationKind
		blockID    string
		topID      string
		topIndex   int
		childIndex int
		choiceID   string
		optionID   string
		triageID   string
		itemID     string
	}{
		{
			name:     "top level",
			id:       "top-choice",
			found:    true,
			kind:     doc.TopLevel,
			blockID:  "top-choice",
			topID:    "top-choice",
			topIndex: 0,
		},
		{
			name:       "card child",
			id:         "child-choice",
			found:      true,
			kind:       doc.CardChild,
			blockID:    "child-choice",
			topID:      "card",
			topIndex:   1,
			childIndex: 1,
		},
		{
			name:     "top-level choice visual",
			id:       "top-visual",
			found:    true,
			kind:     doc.OptionVisual,
			blockID:  "top-visual",
			topID:    "top-choice",
			topIndex: 0,
			choiceID: "top-choice",
			optionID: "top-option",
		},
		{
			name:     "card-child choice visual",
			id:       "child-visual",
			found:    true,
			kind:     doc.OptionVisual,
			blockID:  "child-visual",
			topID:    "card",
			topIndex: 1,
			choiceID: "child-choice",
			optionID: "child-option",
		},
		{
			name:     "top-level triage visual",
			id:       "top-item-visual",
			found:    true,
			kind:     doc.ItemVisual,
			blockID:  "top-item-visual",
			topID:    "top-triage",
			topIndex: 2,
			triageID: "top-triage",
			itemID:   "top-item",
		},
		{
			name:     "card-child triage visual",
			id:       "child-item-visual",
			found:    true,
			kind:     doc.ItemVisual,
			blockID:  "child-item-visual",
			topID:    "card",
			topIndex: 1,
			triageID: "child-triage",
			itemID:   "child-item",
		},
		{name: "absent", id: "missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := doc.Locate(d, tt.id)
			if ok != tt.found {
				t.Fatalf("Locate(%q) found = %t, want %t", tt.id, ok, tt.found)
			}
			if !tt.found {
				if got != (doc.Location{}) {
					t.Errorf("Locate(%q) = %#v, want zero Location", tt.id, got)
				}
				return
			}
			if got.Kind != tt.kind {
				t.Errorf("Locate(%q).Kind = %d, want %d", tt.id, got.Kind, tt.kind)
			}
			if got.Block.BlockID() != tt.blockID {
				t.Errorf("Locate(%q).Block.BlockID() = %q, want %q", tt.id, got.Block.BlockID(), tt.blockID)
			}
			if got.TopID != tt.topID {
				t.Errorf("Locate(%q).TopID = %q, want %q", tt.id, got.TopID, tt.topID)
			}
			if got.TopIndex != tt.topIndex {
				t.Errorf("Locate(%q).TopIndex = %d, want %d", tt.id, got.TopIndex, tt.topIndex)
			}
			if got.ChildIndex != tt.childIndex {
				t.Errorf("Locate(%q).ChildIndex = %d, want %d", tt.id, got.ChildIndex, tt.childIndex)
			}
			if got.ChoiceID != tt.choiceID {
				t.Errorf("Locate(%q).ChoiceID = %q, want %q", tt.id, got.ChoiceID, tt.choiceID)
			}
			if got.OptionID != tt.optionID {
				t.Errorf("Locate(%q).OptionID = %q, want %q", tt.id, got.OptionID, tt.optionID)
			}
			if got.TriageID != tt.triageID {
				t.Errorf("Locate(%q).TriageID = %q, want %q", tt.id, got.TriageID, tt.triageID)
			}
			if got.ItemID != tt.itemID {
				t.Errorf("Locate(%q).ItemID = %q, want %q", tt.id, got.ItemID, tt.itemID)
			}
		})
	}
}

func TestUpsertBlocks(t *testing.T) {
	tests := []struct {
		name       string
		block      string
		after      string
		want       []string
		copiesCard bool
	}{
		{
			name:  "existing top-level id replaces in place and ignores after",
			block: `{"id":"before","type":"approval","prompt":"Replace"}`,
			after: "child-a",
			want:  []string{"before:approval", "card:card[child-a:markdown,child-b:markdown]", "after:markdown"},
		},
		{
			name:       "existing child id replaces in place with a different type and ignores after",
			block:      `{"id":"child-a","type":"approval","prompt":"Replace"}`,
			after:      "after",
			want:       []string{"before:markdown", "card:card[child-a:approval,child-b:markdown]", "after:markdown"},
			copiesCard: true,
		},
		{
			name:  "new id after top-level id inserts at top level",
			block: `{"id":"new","type":"markdown","md":"New"}`,
			after: "before",
			want:  []string{"before:markdown", "new:markdown", "card:card[child-a:markdown,child-b:markdown]", "after:markdown"},
		},
		{
			name:  "new id after card id inserts at top level",
			block: `{"id":"new","type":"markdown","md":"New"}`,
			after: "card",
			want:  []string{"before:markdown", "card:card[child-a:markdown,child-b:markdown]", "new:markdown", "after:markdown"},
		},
		{
			name:       "new id after child id inserts into card",
			block:      `{"id":"new","type":"markdown","md":"New"}`,
			after:      "child-a",
			want:       []string{"before:markdown", "card:card[child-a:markdown,new:markdown,child-b:markdown]", "after:markdown"},
			copiesCard: true,
		},
		{
			name:  "new id without after appends at top level",
			block: `{"id":"new","type":"markdown","md":"New"}`,
			want:  []string{"before:markdown", "card:card[child-a:markdown,child-b:markdown]", "after:markdown", "new:markdown"},
		},
		{
			name:  "new id with unknown after appends at top level",
			block: `{"id":"new","type":"markdown","md":"New"}`,
			after: "missing",
			want:  []string{"before:markdown", "card:card[child-a:markdown,child-b:markdown]", "after:markdown", "new:markdown"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := mustDocument(t, transformDoc)
			original := mustJSON(t, d.Blocks)
			originalCard := d.Blocks[1].(*doc.Card)
			got := doc.UpsertBlocks(d.Blocks, mustBlock(t, tt.block), tt.after)

			if shape := blockShapes(got); !slices.Equal(shape, tt.want) {
				t.Errorf("UpsertBlocks() shape = %v, want %v", shape, tt.want)
			}
			if after := mustJSON(t, d.Blocks); string(after) != string(original) {
				t.Errorf("UpsertBlocks() mutated input:\n got %s\nwant %s", after, original)
			}
			if len(got) > 0 && &got[0] == &d.Blocks[0] {
				t.Error("UpsertBlocks() reused the input top-level slice")
			}
			gotCard := cardByID(t, got, "card")
			if tt.copiesCard {
				if gotCard == originalCard {
					t.Error("UpsertBlocks() reused the input card for a child transform")
				}
				if &gotCard.Children[0] == &originalCard.Children[0] {
					t.Error("UpsertBlocks() reused the input card's children slice")
				}
			} else if gotCard != originalCard {
				t.Error("UpsertBlocks() copied an untouched card")
			}
		})
	}
}

func TestRemoveBlock(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		want       []string
		wantTopID  string
		copiesTop  bool
		copiesCard bool
	}{
		{
			name:      "top level",
			id:        "before",
			want:      []string{"card:card[child-a:markdown,child-b:markdown]", "after:markdown"},
			copiesTop: true,
		},
		{
			name:       "card child",
			id:         "child-a",
			want:       []string{"before:markdown", "card:card[child-b:markdown]", "after:markdown"},
			wantTopID:  "card",
			copiesTop:  true,
			copiesCard: true,
		},
		{
			name: "absent",
			id:   "missing",
			want: []string{"before:markdown", "card:card[child-a:markdown,child-b:markdown]", "after:markdown"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := mustDocument(t, transformDoc)
			original := mustJSON(t, d.Blocks)
			originalCard := d.Blocks[1].(*doc.Card)
			got, topID := doc.RemoveBlock(d.Blocks, tt.id)

			if shape := blockShapes(got); !slices.Equal(shape, tt.want) {
				t.Errorf("RemoveBlock() shape = %v, want %v", shape, tt.want)
			}
			if topID != tt.wantTopID {
				t.Errorf("RemoveBlock() top id = %q, want %q", topID, tt.wantTopID)
			}
			if after := mustJSON(t, d.Blocks); string(after) != string(original) {
				t.Errorf("RemoveBlock() mutated input:\n got %s\nwant %s", after, original)
			}
			if tt.copiesTop && &got[0] == &d.Blocks[0] {
				t.Error("RemoveBlock() reused the input top-level slice")
			}
			gotCard := cardByID(t, got, "card")
			if tt.copiesCard {
				if gotCard == originalCard {
					t.Error("RemoveBlock() reused the input card for a child transform")
				}
				if &gotCard.Children[0] == &originalCard.Children[0] {
					t.Error("RemoveBlock() reused the input card's children slice")
				}
			} else if gotCard != originalCard {
				t.Error("RemoveBlock() copied an untouched card")
			}
		})
	}
}

func mustDocument(t *testing.T, data string) *doc.Doc {
	t.Helper()
	d, err := parse(data)
	if err != nil {
		t.Fatalf("parse document: %v", err)
	}
	return d
}

func mustBlock(t *testing.T, data string) doc.Block {
	t.Helper()
	b, err := doc.DecodeBlock(json.RawMessage(data))
	if err != nil {
		t.Fatalf("decode block: %v", err)
	}
	return b
}

func mustJSON(t *testing.T, blocks []doc.Block) []byte {
	t.Helper()
	data, err := json.Marshal(blocks)
	if err != nil {
		t.Fatalf("marshal value: %v", err)
	}
	return data
}

func blockShapes(blocks []doc.Block) []string {
	shapes := make([]string, len(blocks))
	for i, b := range blocks {
		shapes[i] = b.BlockID() + ":" + b.BlockType()
		children := doc.Children(b)
		if len(children) > 0 {
			shapes[i] += "[" + strings.Join(blockShapes(children), ",") + "]"
		}
	}
	return shapes
}

func cardByID(t *testing.T, blocks []doc.Block, id string) *doc.Card {
	t.Helper()
	for _, b := range blocks {
		if b.BlockID() == id {
			card, ok := b.(*doc.Card)
			if !ok {
				t.Fatalf("block %q has type %T, want *doc.Card", id, b)
			}
			return card
		}
	}
	t.Fatalf("no card %q", id)
	return nil
}

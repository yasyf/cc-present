// Package doc defines the typed-block document that an agent composes and a
// browser renders. The TypeScript declarations in web/src/schema.ts are the
// canonical schema; these structs mirror them with camelCase JSON tags. The doc
// carries only agent-owned display state (card status, progress); human verdicts
// live in a separate reduction of the event log (see package state).
package doc

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// MaxDocBytes and MaxDataURIBytes cap a serialized document and an inline image
// data URI, keeping the append-only log small enough for fresh-tab SSE replay.
const (
	MaxDocBytes     = 1 << 20  // 1 MiB
	MaxDataURIBytes = 32 << 10 // 32 KiB
)

var (
	validStatus        = map[string]bool{"open": true, "resolved": true, "redrafted": true}
	validTone          = map[string]bool{"default": true, "flag": true, "demo": true}
	validProgressState = map[string]bool{"active": true, "done": true, "error": true}
	assetSHAPattern    = regexp.MustCompile(`^asset:[0-9a-f]{64}$`)
	errEmptyBlockID    = errors.New("block id must not be empty")
	errEmptyTitle      = errors.New("doc title must not be empty")
)

// Block is a node in a document. The concrete types are Section, Card, and the
// nine leaf blocks. A section, card, or leaf may appear at the top level, while
// a card nests only leaf blocks. The interface is sealed to this package.
type Block interface {
	BlockID() string
	BlockType() string
	isBlock()
}

type base struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

func (b base) BlockID() string   { return b.ID }
func (b base) BlockType() string { return b.Type }
func (base) isBlock()            {}

// Stat is a headline metric shown in the document header.
type Stat struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// Submit configures the document's submit control.
type Submit struct {
	Label string `json:"label"`
	Note  string `json:"note,omitempty"`
}

// Doc is the document envelope: a flat list of top-level blocks plus header
// metadata. Version is the schema version and is always 1.
type Doc struct {
	Version int     `json:"version"`
	Title   string  `json:"title"`
	Intro   string  `json:"intro,omitempty"`
	Stats   []Stat  `json:"stats,omitempty"`
	Submit  *Submit `json:"submit,omitempty"`
	Blocks  []Block `json:"blocks"`
}

// Section is a top-level header marker with optional prose.
type Section struct {
	base
	Title string `json:"title"`
	Md    string `json:"md,omitempty"`
}

// Chip is a small labelled tag on a card.
type Chip struct {
	Label string `json:"label"`
	Tone  string `json:"tone,omitempty"`
}

// Card is a top-level container that nests one level of child blocks. Status is
// agent-owned display state; it never records a human verdict.
type Card struct {
	base
	Title    string  `json:"title,omitempty"`
	Chips    []Chip  `json:"chips,omitempty"`
	Flagged  bool    `json:"flagged,omitempty"`
	Status   string  `json:"status,omitempty"`
	Children []Block `json:"children"`
}

// Approval is an approve/reject control with optional free-text feedback.
type Approval struct {
	base
	Prompt        string `json:"prompt,omitempty"`
	AllowFeedback *bool  `json:"allowFeedback,omitempty"`
}

// Option is one selectable choice within a Choice block.
type Option struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Md    string `json:"md,omitempty"`
}

// Choice is a single- or multi-select control; selecting an option approves it.
type Choice struct {
	base
	Prompt  string   `json:"prompt,omitempty"`
	Multi   bool     `json:"multi,omitempty"`
	Options []Option `json:"options"`
}

// Input is a free-text field.
type Input struct {
	base
	Label       string `json:"label"`
	Placeholder string `json:"placeholder,omitempty"`
	Multiline   bool   `json:"multiline,omitempty"`
}

// Markdown is a rendered markdown block; Struck applies the "was:" treatment.
type Markdown struct {
	base
	Md     string `json:"md"`
	Struck bool   `json:"struck,omitempty"`
}

// Code is a syntax-highlighted code block.
type Code struct {
	base
	Lang  string `json:"lang"`
	Code  string `json:"code"`
	Title string `json:"title,omitempty"`
}

// Diff is a unified-diff block.
type Diff struct {
	base
	Diff  string `json:"diff"`
	Title string `json:"title,omitempty"`
}

// Image is an image reference; Src is an https:, asset:<sha256>, or data: URI.
type Image struct {
	base
	Src     string `json:"src"`
	Alt     string `json:"alt"`
	Caption string `json:"caption,omitempty"`
}

// Column describes one column of a Table.
type Column struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Align string `json:"align,omitempty"`
}

// Table is a columnar block; each row maps a column key to an inline-markdown cell.
type Table struct {
	base
	Columns []Column            `json:"columns"`
	Rows    []map[string]string `json:"rows"`
}

// Progress is a progress bar; State is agent-owned display state.
type Progress struct {
	base
	Label string `json:"label"`
	Value int    `json:"value"`
	Max   int    `json:"max"`
	State string `json:"state,omitempty"`
}

// UnmarshalJSON decodes the envelope and dispatches each block by its type tag.
func (d *Doc) UnmarshalJSON(data []byte) error {
	var raw struct {
		Version int               `json:"version"`
		Title   string            `json:"title"`
		Intro   string            `json:"intro"`
		Stats   []Stat            `json:"stats"`
		Submit  *Submit           `json:"submit"`
		Blocks  []json.RawMessage `json:"blocks"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshal doc: %w", err)
	}
	d.Version = raw.Version
	d.Title = raw.Title
	d.Intro = raw.Intro
	d.Stats = raw.Stats
	d.Submit = raw.Submit
	blocks, err := decodeBlocks(raw.Blocks)
	if err != nil {
		return err
	}
	d.Blocks = blocks
	return nil
}

// UnmarshalJSON decodes a card and dispatches its children by type tag.
func (c *Card) UnmarshalJSON(data []byte) error {
	var raw struct {
		base
		Title    string            `json:"title"`
		Chips    []Chip            `json:"chips"`
		Flagged  bool              `json:"flagged"`
		Status   string            `json:"status"`
		Children []json.RawMessage `json:"children"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshal card: %w", err)
	}
	c.base = raw.base
	c.Title = raw.Title
	c.Chips = raw.Chips
	c.Flagged = raw.Flagged
	c.Status = raw.Status
	children, err := decodeBlocks(raw.Children)
	if err != nil {
		return err
	}
	c.Children = children
	return nil
}

func decodeBlocks(raw []json.RawMessage) ([]Block, error) {
	blocks := make([]Block, 0, len(raw))
	for _, rb := range raw {
		b, err := DecodeBlock(rb)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, b)
	}
	return blocks, nil
}

// DecodeBlock decodes a single block from its JSON, dispatching on the type tag.
// An unknown type is an error naming the offending block id.
func DecodeBlock(data json.RawMessage) (Block, error) {
	var head base
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, fmt.Errorf("read block type: %w", err)
	}
	switch head.Type {
	case "section":
		return decodeInto(data, &Section{})
	case "card":
		return decodeInto(data, &Card{})
	case "approval":
		return decodeInto(data, &Approval{})
	case "choice":
		return decodeInto(data, &Choice{})
	case "input":
		return decodeInto(data, &Input{})
	case "markdown":
		return decodeInto(data, &Markdown{})
	case "code":
		return decodeInto(data, &Code{})
	case "diff":
		return decodeInto(data, &Diff{})
	case "image":
		return decodeInto(data, &Image{})
	case "table":
		return decodeInto(data, &Table{})
	case "progress":
		return decodeInto(data, &Progress{})
	default:
		return nil, fmt.Errorf("block %q: unknown type %q", head.ID, head.Type)
	}
}

func decodeInto[T Block](data json.RawMessage, dst T) (Block, error) {
	if err := json.Unmarshal(data, dst); err != nil {
		return nil, fmt.Errorf("decode %s block: %w", dst.BlockType(), err)
	}
	return dst, nil
}

// Validate reports the first structural violation in the document: version must
// be 1, the title non-empty, every block id globally unique, cards nesting
// exactly one level of leaf blocks (a card may not contain a section or another
// card), per-type required fields present, the serialized doc within
// MaxDocBytes, and image data URIs within MaxDataURIBytes.
func (d *Doc) Validate() error {
	if d.Version != 1 {
		return fmt.Errorf("doc version must be 1, got %d", d.Version)
	}
	if d.Title == "" {
		return errEmptyTitle
	}
	seen := map[string]bool{}
	for _, b := range d.Blocks {
		if err := registerID(seen, b.BlockID()); err != nil {
			return err
		}
		switch blk := b.(type) {
		case *Section:
			if err := validateSection(blk); err != nil {
				return err
			}
		case *Card:
			if err := validateCard(seen, blk); err != nil {
				return err
			}
		default:
			if err := validateLeaf(b); err != nil {
				return err
			}
		}
	}
	data, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("marshal doc: %w", err)
	}
	if len(data) > MaxDocBytes {
		return fmt.Errorf("doc is %d bytes, exceeds %d", len(data), MaxDocBytes)
	}
	return nil
}

func registerID(seen map[string]bool, id string) error {
	if id == "" {
		return errEmptyBlockID
	}
	if seen[id] {
		return fmt.Errorf("duplicate block id %q", id)
	}
	seen[id] = true
	return nil
}

func validateSection(s *Section) error {
	if s.Title == "" {
		return fmt.Errorf("section %q: title must not be empty", s.ID)
	}
	return nil
}

func validateCard(seen map[string]bool, c *Card) error {
	if c.Status != "" && !validStatus[c.Status] {
		return fmt.Errorf("card %q: invalid status %q", c.ID, c.Status)
	}
	for _, chip := range c.Chips {
		if chip.Tone != "" && !validTone[chip.Tone] {
			return fmt.Errorf("card %q: invalid chip tone %q", c.ID, chip.Tone)
		}
	}
	for _, child := range c.Children {
		if err := registerID(seen, child.BlockID()); err != nil {
			return err
		}
		if err := validateChild(c.ID, child); err != nil {
			return err
		}
	}
	return nil
}

func validateChild(cardID string, child Block) error {
	switch child.(type) {
	case *Section, *Card:
		return fmt.Errorf("card %q: child %q may not be a %s (cards allow exactly one nesting level)", cardID, child.BlockID(), child.BlockType())
	}
	return validateLeaf(child)
}

func validateLeaf(b Block) error {
	switch lb := b.(type) {
	case *Approval:
		return nil
	case *Choice:
		return validateChoice(lb)
	case *Input:
		if lb.Label == "" {
			return fmt.Errorf("input %q: label must not be empty", lb.ID)
		}
		return nil
	case *Markdown:
		if lb.Md == "" {
			return fmt.Errorf("markdown %q: md must not be empty", lb.ID)
		}
		return nil
	case *Code:
		return validateCode(lb)
	case *Diff:
		if lb.Diff == "" {
			return fmt.Errorf("diff %q: diff must not be empty", lb.ID)
		}
		return nil
	case *Image:
		return validateImage(lb)
	case *Table:
		return validateTable(lb)
	case *Progress:
		return validateProgress(lb)
	}
	panic(fmt.Sprintf("unreachable: unhandled block type %q", b.BlockType()))
}

func validateChoice(c *Choice) error {
	if len(c.Options) == 0 {
		return fmt.Errorf("choice %q: must have at least one option", c.ID)
	}
	optSeen := map[string]bool{}
	for _, o := range c.Options {
		if o.ID == "" {
			return fmt.Errorf("choice %q: option id must not be empty", c.ID)
		}
		if optSeen[o.ID] {
			return fmt.Errorf("choice %q: duplicate option id %q", c.ID, o.ID)
		}
		optSeen[o.ID] = true
		if o.Label == "" {
			return fmt.Errorf("choice %q: option %q label must not be empty", c.ID, o.ID)
		}
	}
	return nil
}

func validateCode(c *Code) error {
	if c.Lang == "" {
		return fmt.Errorf("code %q: lang must not be empty", c.ID)
	}
	if c.Code == "" {
		return fmt.Errorf("code %q: code must not be empty", c.ID)
	}
	return nil
}

func validateImage(i *Image) error {
	if i.Alt == "" {
		return fmt.Errorf("image %q: alt must not be empty", i.ID)
	}
	switch {
	case strings.HasPrefix(i.Src, "https://"):
		return nil
	case strings.HasPrefix(i.Src, "asset:"):
		if !assetSHAPattern.MatchString(i.Src) {
			return fmt.Errorf("image %q: asset src must be asset:<64-hex-sha256>, got %q", i.ID, i.Src)
		}
		return nil
	case strings.HasPrefix(i.Src, "data:"):
		if len(i.Src) > MaxDataURIBytes {
			return fmt.Errorf("image %q: data URI is %d bytes, exceeds %d", i.ID, len(i.Src), MaxDataURIBytes)
		}
		return nil
	default:
		return fmt.Errorf("image %q: src must be https://, asset:<sha256>, or data:, got %q", i.ID, i.Src)
	}
}

func validateTable(t *Table) error {
	if len(t.Columns) == 0 {
		return fmt.Errorf("table %q: must have at least one column", t.ID)
	}
	for _, col := range t.Columns {
		if col.Key == "" {
			return fmt.Errorf("table %q: column key must not be empty", t.ID)
		}
		if col.Label == "" {
			return fmt.Errorf("table %q: column %q label must not be empty", t.ID, col.Key)
		}
		if col.Align != "" && col.Align != "left" && col.Align != "right" {
			return fmt.Errorf("table %q: column %q invalid align %q", t.ID, col.Key, col.Align)
		}
	}
	return nil
}

func validateProgress(p *Progress) error {
	if p.Label == "" {
		return fmt.Errorf("progress %q: label must not be empty", p.ID)
	}
	if p.Max <= 0 {
		return fmt.Errorf("progress %q: max must be > 0, got %d", p.ID, p.Max)
	}
	if p.Value < 0 || p.Value > p.Max {
		return fmt.Errorf("progress %q: value %d out of range [0,%d]", p.ID, p.Value, p.Max)
	}
	if p.State != "" && !validProgressState[p.State] {
		return fmt.Errorf("progress %q: invalid state %q", p.ID, p.State)
	}
	return nil
}

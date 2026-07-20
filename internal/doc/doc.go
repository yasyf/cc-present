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
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// MaxDocBytes and MaxDataURIBytes cap a serialized document and an inline image
// data URI, keeping the append-only log small enough for fresh-tab SSE replay.
const (
	MaxDocBytes     = 1 << 20  // 1 MiB
	MaxDataURIBytes = 32 << 10 // 32 KiB
	MaxDiagramBytes = 8 << 10  // 8 KiB
	MaxTermBytes    = 32 << 10 // 32 KiB
	MaxChartSeries  = 6
	MaxChartPoints  = 100
	MaxChartAbs     = 1e15  // largest chart value magnitude the client scale renders without float overflow
	MinChartAbs     = 1e-15 // smallest non-zero magnitude before the client scale underflows to a zero step
	MaxTreeEntries  = 200
	MaxTreeDepth    = 32
	MaxRecordFacts  = 16
	MaxRecordChips  = 8
	MaxRecordLinks  = 8
)

var (
	validStatus        = map[string]bool{"open": true, "resolved": true, "redrafted": true}
	validTone          = map[string]bool{"default": true, "flag": true, "demo": true}
	validFactTone      = map[string]bool{"default": true, "good": true, "warn": true, "bad": true}
	validDetailMode    = map[string]bool{"inline": true, "modal": true}
	validProgressState = map[string]bool{"active": true, "done": true, "error": true}
	validPresentation  = map[string]bool{"focus": true, "board": true}
	validVisualType    = map[string]bool{"code": true, "diagram": true, "image": true, "diff": true, "chart": true, "term": true, "filetree": true, "record": true}
	validChartKind     = map[string]bool{"bar": true, "line": true}
	validTreeBadge     = map[string]bool{"added": true, "modified": true, "removed": true}
	assetSHAPattern    = regexp.MustCompile(`^asset:[0-9a-f]{64}$`)
	driveLetterPrefix  = regexp.MustCompile(`^[A-Za-z]:$`)
	errEmptyBlockID    = errors.New("block id must not be empty")
	errEmptyTitle      = errors.New("doc title must not be empty")
)

// Block is a node in a document. The concrete types are Section, Card, and the
// leaf blocks. A section, card, or leaf may appear at the top level, while
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
	Version      int     `json:"version"`
	Title        string  `json:"title"`
	Intro        string  `json:"intro,omitempty"`
	Stats        []Stat  `json:"stats,omitempty"`
	Submit       *Submit `json:"submit,omitempty"`
	Presentation *string `json:"presentation,omitempty"`
	Blocks       []Block `json:"blocks"`
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
	Summary  string  `json:"summary,omitempty"`
	Chips    []Chip  `json:"chips,omitempty"`
	Flagged  bool    `json:"flagged,omitempty"`
	Status   string  `json:"status,omitempty"`
	Children []Block `json:"children"`
}

// Approval is an approve/reject control with optional free-text feedback.
type Approval struct {
	base
	Prompt        string  `json:"prompt,omitempty"`
	AllowFeedback *bool   `json:"allowFeedback,omitempty"`
	Detail        *Detail `json:"detail,omitempty"`
}

// Fact is one scannable key/value in an option's up-front cluster.
type Fact struct {
	Label string `json:"label,omitempty"`
	Value string `json:"value"`
	Tone  string `json:"tone,omitempty"` // default|good|warn|bad
}

// Detail is an expandable drill-down: the tradeoffs and full rationale a human
// needs to decide, hidden until opened.
type Detail struct {
	Pros []string `json:"pros,omitempty"`
	Cons []string `json:"cons,omitempty"`
	Md   string   `json:"md,omitempty"`
	Mode string   `json:"mode,omitempty"` // inline|modal (default inline)
}

// Option is one selectable choice within a Choice block. Recommended marks the
// author's suggested pick (at most one per single-select choice). Visual is an
// optional restricted leaf — a code, diagram, image, or diff block — rendered in
// the option's visual stage rather than inside the row.
type Option struct {
	ID          string  `json:"id"`
	Label       string  `json:"label"`
	Hint        string  `json:"hint,omitempty"`
	Md          string  `json:"md,omitempty"`
	Facts       []Fact  `json:"facts,omitempty"`
	Detail      *Detail `json:"detail,omitempty"`
	Recommended bool    `json:"recommended,omitempty"`
	Visual      Block   `json:"visual,omitempty"`
}

// UnmarshalJSON decodes an option, dispatching its optional visual through
// DecodeBlock and rejecting any type outside the {code, diagram, image, diff,
// chart, term, filetree, record} allowlist so a disallowed visual fails loudly
// at decode. Marshaling needs no custom path: the concrete visual block marshals
// naturally and a nil interface omits.
func (o *Option) UnmarshalJSON(data []byte) error {
	var raw struct {
		ID          string          `json:"id"`
		Label       string          `json:"label"`
		Hint        string          `json:"hint"`
		Md          string          `json:"md"`
		Facts       []Fact          `json:"facts"`
		Detail      *Detail         `json:"detail"`
		Recommended bool            `json:"recommended"`
		Visual      json.RawMessage `json:"visual"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshal option: %w", err)
	}
	o.ID = raw.ID
	o.Label = raw.Label
	o.Hint = raw.Hint
	o.Md = raw.Md
	o.Facts = raw.Facts
	o.Detail = raw.Detail
	o.Recommended = raw.Recommended
	if len(raw.Visual) > 0 && string(raw.Visual) != "null" {
		v, err := DecodeBlock(raw.Visual)
		if err != nil {
			return fmt.Errorf("option %q visual: %w", raw.ID, err)
		}
		if !validVisualType[v.BlockType()] {
			return fmt.Errorf("option %q visual: type %q is not an allowed visual (code, diagram, image, diff, chart, term, filetree, record)", raw.ID, v.BlockType())
		}
		o.Visual = v
	}
	return nil
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

// Diagram is a text-to-diagram block rendered client-side; Kind is "mermaid".
type Diagram struct {
	base
	Kind   string `json:"kind"`
	Source string `json:"source"`
	Title  string `json:"title,omitempty"`
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

// Series is one data series in a Chart; Values align index-for-index to the
// chart's categories.
type Series struct {
	Label  string    `json:"label"`
	Values []float64 `json:"values"`
}

// Chart is a categorical bar or line chart rendered client-side into themed SVG.
type Chart struct {
	base
	Kind       string   `json:"kind"`
	Title      string   `json:"title,omitempty"`
	Unit       string   `json:"unit,omitempty"`
	Categories []string `json:"categories"`
	Series     []Series `json:"series"`
}

// Term is a terminal panel: an optional command line above its captured output.
type Term struct {
	base
	Command string `json:"command,omitempty"`
	Output  string `json:"output"`
	Title   string `json:"title,omitempty"`
}

// TreeEntry is one file path in a FileTree; directories are implicit from the
// path segments. Badge is a git-status change marker; Note is an inline
// annotation.
type TreeEntry struct {
	Path  string `json:"path"`
	Badge string `json:"badge,omitempty"`
	Note  string `json:"note,omitempty"`
}

// FileTree is a set of relative file paths rendered as a collapsible tree with
// change badges.
type FileTree struct {
	base
	Title   string      `json:"title,omitempty"`
	Entries []TreeEntry `json:"entries"`
}

// Link is a labelled https link in a Record.
type Link struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// Record is one entity's labelled profile: tone chips, keyed facts, and https
// links.
type Record struct {
	base
	Title string `json:"title,omitempty"`
	Chips []Chip `json:"chips,omitempty"`
	Facts []Fact `json:"facts"`
	Links []Link `json:"links,omitempty"`
}

// UnmarshalJSON decodes the envelope and dispatches each block by its type tag.
func (d *Doc) UnmarshalJSON(data []byte) error {
	var raw struct {
		Version      int               `json:"version"`
		Title        string            `json:"title"`
		Intro        string            `json:"intro"`
		Stats        []Stat            `json:"stats"`
		Submit       *Submit           `json:"submit"`
		Presentation *string           `json:"presentation"`
		Blocks       []json.RawMessage `json:"blocks"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshal doc: %w", err)
	}
	d.Version = raw.Version
	d.Title = raw.Title
	d.Intro = raw.Intro
	d.Stats = raw.Stats
	d.Submit = raw.Submit
	d.Presentation = raw.Presentation
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
		Summary  string            `json:"summary"`
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
	c.Summary = raw.Summary
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

// BlockList is a slice of blocks that decodes each element polymorphically by
// its type tag, mirroring Doc.Blocks; it lets a struct hold a block slice that
// round-trips through JSON without a bespoke decoder.
type BlockList []Block

// UnmarshalJSON decodes a JSON array, dispatching each element by its type tag.
func (bl *BlockList) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshal block list: %w", err)
	}
	blocks, err := decodeBlocks(raw)
	if err != nil {
		return err
	}
	*bl = blocks
	return nil
}

// DecodeBlock decodes a single block from its JSON, dispatching on the type tag.
// A well-formed dotted type decodes to *PackBlock unconditionally; a malformed
// dotted type and an unknown dot-free type are each an error naming the block id.
func DecodeBlock(data json.RawMessage) (Block, error) {
	var head base
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, fmt.Errorf("read block type: %w", err)
	}
	if strings.Contains(head.Type, ".") {
		if !packTypePattern.MatchString(head.Type) {
			return nil, fmt.Errorf("block %q: malformed pack type %q", head.ID, head.Type)
		}
		pb := &PackBlock{}
		if err := json.Unmarshal(data, pb); err != nil {
			return nil, fmt.Errorf("decode pack block %q: %w", head.ID, err)
		}
		return pb, nil
	}
	sp, ok := registry[head.Type]
	if !ok {
		return nil, fmt.Errorf("block %q: unknown type %q", head.ID, head.Type)
	}
	return sp.decode(data)
}

func decodeInto[T Block](data json.RawMessage, dst T) (Block, error) {
	if err := json.Unmarshal(data, dst); err != nil {
		return nil, fmt.Errorf("decode %s block: %w", dst.BlockType(), err)
	}
	return dst, nil
}

// Validate reports every structural violation in the document, joined via
// errors.Join so a whole board's problems surface in one pass: version must be 1,
// the title non-empty, presentation one of "focus" or "board" when set, every
// block id globally unique, cards nesting exactly one level of leaf blocks (a
// card may not contain a section or another card), per-type required fields
// present, the serialized doc within MaxDocBytes, and image data URIs within
// MaxDataURIBytes. Each block's validator still fails fast at its first
// violation; the per-block boundary is the collection granularity. Pack blocks
// are leaf-only and validated against pt.
func (d *Doc) Validate(pt PackTypes) error {
	var errs []error
	if d.Version != 1 {
		errs = append(errs, fmt.Errorf("doc version must be 1, got %d", d.Version))
	}
	if d.Title == "" {
		errs = append(errs, errEmptyTitle)
	}
	if d.Presentation != nil && !validPresentation[*d.Presentation] {
		errs = append(errs, fmt.Errorf("doc presentation must be focus or board, got %q", *d.Presentation))
	}
	seen := map[string]bool{}
	for _, b := range d.Blocks {
		if err := registerID(seen, b.BlockID()); err != nil {
			errs = append(errs, err)
		}
		if err := validateBlock(b, pt); err != nil {
			errs = append(errs, err)
		}
		for _, v := range Visuals(b) {
			if err := registerID(seen, v.BlockID()); err != nil {
				errs = append(errs, err)
			}
		}
		for _, child := range Children(b) {
			if err := registerID(seen, child.BlockID()); err != nil {
				errs = append(errs, err)
			}
			if err := validateChild(b, child, pt); err != nil {
				errs = append(errs, err)
			}
			for _, v := range Visuals(child) {
				if err := registerID(seen, v.BlockID()); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}
	if data, err := json.Marshal(d); err != nil {
		errs = append(errs, fmt.Errorf("marshal doc: %w", err))
	} else if len(data) > MaxDocBytes {
		errs = append(errs, fmt.Errorf("doc is %d bytes, exceeds %d", len(data), MaxDocBytes))
	}
	return errors.Join(errs...)
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

func validateCard(c *Card) error {
	if isMultiline(c.Summary) {
		return fmt.Errorf("card %q: summary must be a single line", c.ID)
	}
	if c.Status != "" && !validStatus[c.Status] {
		return fmt.Errorf("card %q: invalid status %q", c.ID, c.Status)
	}
	for _, chip := range c.Chips {
		if err := validateChip(chip); err != nil {
			return fmt.Errorf("card %q: %w", c.ID, err)
		}
	}
	return nil
}

func validateChip(c Chip) error {
	if c.Label == "" {
		return errors.New("chip label must not be empty")
	}
	if isMultiline(c.Label) {
		return errors.New("chip label must be a single line")
	}
	if c.Tone != "" && !validTone[c.Tone] {
		return fmt.Errorf("invalid chip tone %q", c.Tone)
	}
	return nil
}

// validateBlock validates a block against its own rules: a pack block against pt,
// any built-in against its registered validator.
func validateBlock(b Block, pt PackTypes) error {
	if pb, ok := b.(*PackBlock); ok {
		return pt.ValidateBlock(pb.Type, pb.PayloadJSON())
	}
	return registry[b.BlockType()].validate(b)
}

func validateChild(parent, child Block, pt PackTypes) error {
	if pb, ok := child.(*PackBlock); ok {
		return pt.ValidateBlock(pb.Type, pb.PayloadJSON())
	}
	sp := registry[child.BlockType()]
	if sp.topOnly {
		return fmt.Errorf("card %q: child %q may not be a %s (cards allow exactly one nesting level)", parent.BlockID(), child.BlockID(), child.BlockType())
	}
	return sp.validate(child)
}

func validateApproval(a *Approval) error {
	if err := validateDetail(a.Detail); err != nil {
		return fmt.Errorf("approval %q: %w", a.ID, err)
	}
	return nil
}

func validateFact(f Fact) error {
	if f.Value == "" {
		return errors.New("fact value must not be empty")
	}
	if isMultiline(f.Value) {
		return errors.New("fact value must be a single line")
	}
	if isMultiline(f.Label) {
		return errors.New("fact label must be a single line")
	}
	if f.Tone != "" && !validFactTone[f.Tone] {
		return fmt.Errorf("fact tone must be default, good, warn, or bad, got %q", f.Tone)
	}
	return nil
}

func validateDetail(d *Detail) error {
	if d == nil {
		return nil
	}
	if len(d.Pros) == 0 && len(d.Cons) == 0 && d.Md == "" {
		return errors.New("detail must set at least one of pros, cons, or md")
	}
	for _, p := range d.Pros {
		if p == "" {
			return errors.New("detail pro must not be empty")
		}
		if isMultiline(p) {
			return errors.New("detail pro must be a single line")
		}
	}
	for _, c := range d.Cons {
		if c == "" {
			return errors.New("detail con must not be empty")
		}
		if isMultiline(c) {
			return errors.New("detail con must be a single line")
		}
	}
	if d.Mode != "" && !validDetailMode[d.Mode] {
		return fmt.Errorf("detail mode must be inline or modal, got %q", d.Mode)
	}
	return nil
}

func validateInput(i *Input) error {
	if i.Label == "" {
		return fmt.Errorf("input %q: label must not be empty", i.ID)
	}
	return nil
}

func validateMarkdown(m *Markdown) error {
	if m.Md == "" {
		return fmt.Errorf("markdown %q: md must not be empty", m.ID)
	}
	return nil
}

func validateDiff(d *Diff) error {
	if d.Diff == "" {
		return fmt.Errorf("diff %q: diff must not be empty", d.ID)
	}
	return nil
}

// validateVisual validates an option-visual leaf against its own rules. The
// allowlist is enforced at decode (Option.UnmarshalJSON); the default arm guards
// a visual constructed outside the JSON path.
func validateVisual(b Block) error {
	switch v := b.(type) {
	case *Code:
		return validateCode(v)
	case *Diagram:
		return validateDiagram(v)
	case *Image:
		return validateImage(v)
	case *Diff:
		return validateDiff(v)
	case *Chart:
		return validateChart(v)
	case *Term:
		return validateTerm(v)
	case *FileTree:
		return validateFileTree(v)
	case *Record:
		return validateRecord(v)
	default:
		return fmt.Errorf("type %q is not an allowed visual (code, diagram, image, diff, chart, term, filetree, record)", b.BlockType())
	}
}

func validateDiagram(d *Diagram) error {
	if d.Kind != "mermaid" {
		return fmt.Errorf("diagram %q: kind must be mermaid, got %q", d.ID, d.Kind)
	}
	if d.Source == "" {
		return fmt.Errorf("diagram %q: source must not be empty", d.ID)
	}
	if len(d.Source) > MaxDiagramBytes {
		return fmt.Errorf("diagram %q: source is %d bytes, exceeds %d", d.ID, len(d.Source), MaxDiagramBytes)
	}
	if isMultiline(d.Title) {
		return fmt.Errorf("diagram %q: title must be a single line", d.ID)
	}
	return nil
}

func validateChoice(c *Choice) error {
	if len(c.Options) == 0 {
		return fmt.Errorf("choice %q: must have at least one option", c.ID)
	}
	optSeen := map[string]bool{}
	recommended := 0
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
		if isMultiline(o.Hint) {
			return fmt.Errorf("choice %q: option %q hint must be a single line", c.ID, o.ID)
		}
		for _, f := range o.Facts {
			if err := validateFact(f); err != nil {
				return fmt.Errorf("choice %q: option %q: %w", c.ID, o.ID, err)
			}
		}
		if err := validateDetail(o.Detail); err != nil {
			return fmt.Errorf("choice %q: option %q: %w", c.ID, o.ID, err)
		}
		if o.Visual != nil {
			if err := validateVisual(o.Visual); err != nil {
				return fmt.Errorf("choice %q: option %q visual: %w", c.ID, o.ID, err)
			}
		}
		if o.Recommended {
			recommended++
		}
	}
	if !c.Multi && recommended > 1 {
		return fmt.Errorf("choice %q: single-select allows at most one recommended option, got %d", c.ID, recommended)
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

func isMultiline(s string) bool {
	return strings.ContainsAny(s, "\r\n")
}

func validateChart(c *Chart) error {
	if !validChartKind[c.Kind] {
		return fmt.Errorf("chart %q: kind must be bar or line, got %q", c.ID, c.Kind)
	}
	if isMultiline(c.Title) {
		return fmt.Errorf("chart %q: title must be a single line", c.ID)
	}
	if isMultiline(c.Unit) {
		return fmt.Errorf("chart %q: unit must be a single line", c.ID)
	}
	if len(c.Categories) == 0 {
		return fmt.Errorf("chart %q: must have at least one category", c.ID)
	}
	if len(c.Categories) > MaxChartPoints {
		return fmt.Errorf("chart %q: %d categories exceeds %d", c.ID, len(c.Categories), MaxChartPoints)
	}
	seenCat := map[string]bool{}
	for _, cat := range c.Categories {
		if cat == "" {
			return fmt.Errorf("chart %q: category must not be empty", c.ID)
		}
		if isMultiline(cat) {
			return fmt.Errorf("chart %q: category must be a single line", c.ID)
		}
		if seenCat[cat] {
			return fmt.Errorf("chart %q: duplicate category %q", c.ID, cat)
		}
		seenCat[cat] = true
	}
	if len(c.Series) == 0 {
		return fmt.Errorf("chart %q: must have at least one series", c.ID)
	}
	if len(c.Series) > MaxChartSeries {
		return fmt.Errorf("chart %q: %d series exceeds %d", c.ID, len(c.Series), MaxChartSeries)
	}
	seenSeries := map[string]bool{}
	for _, s := range c.Series {
		if s.Label == "" {
			return fmt.Errorf("chart %q: series label must not be empty", c.ID)
		}
		if isMultiline(s.Label) {
			return fmt.Errorf("chart %q: series label must be a single line", c.ID)
		}
		if seenSeries[s.Label] {
			return fmt.Errorf("chart %q: duplicate series label %q", c.ID, s.Label)
		}
		seenSeries[s.Label] = true
		if len(s.Values) != len(c.Categories) {
			return fmt.Errorf("chart %q: series %q has %d values, want %d (one per category)", c.ID, s.Label, len(s.Values), len(c.Categories))
		}
		for _, v := range s.Values {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return fmt.Errorf("chart %q: series %q has a non-finite value", c.ID, s.Label)
			}
			if a := math.Abs(v); v != 0 && (a < MinChartAbs || a > MaxChartAbs) {
				return fmt.Errorf("chart %q: series %q value magnitude must be 0 or within [%g, %g]", c.ID, s.Label, MinChartAbs, MaxChartAbs)
			}
		}
	}
	return nil
}

func validateTerm(t *Term) error {
	if t.Output == "" {
		return fmt.Errorf("term %q: output must not be empty", t.ID)
	}
	if len(t.Output) > MaxTermBytes {
		return fmt.Errorf("term %q: output is %d bytes, exceeds %d", t.ID, len(t.Output), MaxTermBytes)
	}
	if isMultiline(t.Command) {
		return fmt.Errorf("term %q: command must be a single line", t.ID)
	}
	if isMultiline(t.Title) {
		return fmt.Errorf("term %q: title must be a single line", t.ID)
	}
	return nil
}

func validateFileTree(f *FileTree) error {
	if isMultiline(f.Title) {
		return fmt.Errorf("filetree %q: title must be a single line", f.ID)
	}
	if len(f.Entries) == 0 {
		return fmt.Errorf("filetree %q: must have at least one entry", f.ID)
	}
	if len(f.Entries) > MaxTreeEntries {
		return fmt.Errorf("filetree %q: %d entries exceeds %d", f.ID, len(f.Entries), MaxTreeEntries)
	}
	seen := map[string]bool{}
	for _, e := range f.Entries {
		if err := validateTreePath(e.Path); err != nil {
			return fmt.Errorf("filetree %q: %w", f.ID, err)
		}
		if seen[e.Path] {
			return fmt.Errorf("filetree %q: duplicate path %q", f.ID, e.Path)
		}
		seen[e.Path] = true
		if e.Badge != "" && !validTreeBadge[e.Badge] {
			return fmt.Errorf("filetree %q: invalid badge %q", f.ID, e.Badge)
		}
		if isMultiline(e.Note) {
			return fmt.Errorf("filetree %q: note must be a single line", f.ID)
		}
	}
	return nil
}

func validateTreePath(p string) error {
	if p == "" {
		return errors.New("path must not be empty")
	}
	if strings.HasPrefix(p, "/") {
		return fmt.Errorf("path %q must be relative", p)
	}
	if strings.Contains(p, `\`) {
		return fmt.Errorf("path %q must use forward slashes", p)
	}
	if strings.HasSuffix(p, "/") {
		return fmt.Errorf("path %q must not end with a slash", p)
	}
	segs := strings.Split(p, "/")
	if driveLetterPrefix.MatchString(segs[0]) {
		return fmt.Errorf("path %q must be relative", p)
	}
	if len(segs) > MaxTreeDepth {
		return fmt.Errorf("path %q exceeds depth %d", p, MaxTreeDepth)
	}
	for _, seg := range segs {
		if seg == "" || seg == "." || seg == ".." {
			return fmt.Errorf("path %q has an empty or dot segment", p)
		}
	}
	return nil
}

func validateRecord(r *Record) error {
	if isMultiline(r.Title) {
		return fmt.Errorf("record %q: title must be a single line", r.ID)
	}
	if len(r.Chips) > MaxRecordChips {
		return fmt.Errorf("record %q: %d chips exceeds %d", r.ID, len(r.Chips), MaxRecordChips)
	}
	for _, chip := range r.Chips {
		if err := validateChip(chip); err != nil {
			return fmt.Errorf("record %q: %w", r.ID, err)
		}
	}
	if len(r.Facts) == 0 {
		return fmt.Errorf("record %q: must have at least one fact", r.ID)
	}
	if len(r.Facts) > MaxRecordFacts {
		return fmt.Errorf("record %q: %d facts exceeds %d", r.ID, len(r.Facts), MaxRecordFacts)
	}
	for _, f := range r.Facts {
		if f.Label == "" {
			return fmt.Errorf("record %q: fact label must not be empty", r.ID)
		}
		if err := validateFact(f); err != nil {
			return fmt.Errorf("record %q: %w", r.ID, err)
		}
	}
	if len(r.Links) > MaxRecordLinks {
		return fmt.Errorf("record %q: %d links exceeds %d", r.ID, len(r.Links), MaxRecordLinks)
	}
	for _, l := range r.Links {
		if err := validateLink(l); err != nil {
			return fmt.Errorf("record %q: %w", r.ID, err)
		}
	}
	return nil
}

func validateLink(l Link) error {
	if l.Label == "" {
		return errors.New("link label must not be empty")
	}
	if isMultiline(l.Label) {
		return errors.New("link label must be a single line")
	}
	u, err := url.Parse(l.URL)
	if err != nil {
		return fmt.Errorf("link url %q is not a valid URL: %w", l.URL, err)
	}
	if u.Scheme != "https" || u.Hostname() == "" {
		return fmt.Errorf("link url must be https with a host, got %q", l.URL)
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return fmt.Errorf("link url has an invalid port %q (must be 1-65535)", port)
		}
	}
	return nil
}

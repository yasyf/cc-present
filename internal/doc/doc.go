// Package doc defines the typed-block document that an agent composes and a
// browser renders. The TypeScript declarations in web/src/schema.ts are the
// canonical schema; these structs mirror them with camelCase JSON tags. The doc
// carries only agent-owned display state (card status, progress); human verdicts
// live in a separate reduction of the event log (see package state).
package doc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"path"
	"regexp"
	"slices"
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
	MaxDraftBytes   = 64 << 10 // 64 KiB
	MaxChartSeries  = 6
	MaxChartPoints  = 100
	MaxChartAbs     = 1e15  // largest chart value magnitude the client scale renders without float overflow
	MinChartAbs     = 1e-15 // smallest non-zero magnitude before the client scale underflows to a zero step
	MaxTreeEntries  = 200
	MaxTreeDepth    = 32
	MaxRecordFacts  = 16
	MaxRecordChips  = 8
	MaxRecordLinks  = 8
	MaxTriageItems  = 50
	MaxCodeLine     = 1_000_000
	MaxPRFiles      = 50
	MaxPRGroups     = 30
	MaxPRGroupRows  = 150
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
	lineRangePattern   = regexp.MustCompile(`^([1-9][0-9]*)(?:-([1-9][0-9]*))?$`)
	codeShaPattern     = regexp.MustCompile(`^[0-9a-f]{7,40}(\+wt)?$`)
	sourceAliasPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	repoPattern        = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
	headPattern        = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
	slackTSPattern     = regexp.MustCompile(`^\d{10}\.\d{6}$`)
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
	Version      int               `json:"version"`
	Title        string            `json:"title"`
	Intro        string            `json:"intro,omitempty"`
	Stats        []Stat            `json:"stats,omitempty"`
	Submit       *Submit           `json:"submit,omitempty"`
	Presentation *string           `json:"presentation,omitempty"`
	Sources      map[string]string `json:"sources,omitempty"`
	Blocks       []Block           `json:"blocks"`
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
// agent-owned display state; it never records a human verdict. Slack, when set,
// links the cc-slack ask card that asks the same question; an answer on either
// surface answers both.
type Card struct {
	base
	Title    string     `json:"title,omitempty"`
	Summary  string     `json:"summary,omitempty"`
	Chips    []Chip     `json:"chips,omitempty"`
	Flagged  bool       `json:"flagged,omitempty"`
	Status   string     `json:"status,omitempty"`
	Slack    *CardSlack `json:"slack,omitempty"`
	Children []Block    `json:"children"`
}

// CardSlack is the Slack ask card a card is synced with: URL is its permalink.
type CardSlack struct {
	URL string `json:"url"`
}

// Approval is an approve/reject control whose verdict may carry a note.
type Approval struct {
	base
	Prompt string  `json:"prompt,omitempty"`
	Detail *Detail `json:"detail,omitempty"`
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

func decodeOption(data json.RawMessage, unmarshal unmarshalFunc) (Option, error) {
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
	if err := unmarshal(data, &raw); err != nil {
		return Option{}, fmt.Errorf("unmarshal option: %w", err)
	}
	v, err := decodeVisual(raw.Visual, fmt.Sprintf("option %q", raw.ID), unmarshal)
	if err != nil {
		return Option{}, err
	}
	return Option{
		ID:          raw.ID,
		Label:       raw.Label,
		Hint:        raw.Hint,
		Md:          raw.Md,
		Facts:       raw.Facts,
		Detail:      raw.Detail,
		Recommended: raw.Recommended,
		Visual:      v,
	}, nil
}

// decodeVisual decodes an optional option- or item-visual from raw, enforcing the
// {code, diagram, image, diff, chart, term, filetree, record} allowlist so a
// disallowed type fails loudly at decode. It returns a nil block for an absent or
// JSON-null visual; owner labels the enclosing element in any error.
func decodeVisual(raw json.RawMessage, owner string, unmarshal unmarshalFunc) (Block, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	v, err := decodeBlock(raw, unmarshal)
	if err != nil {
		return nil, fmt.Errorf("%s visual: %w", owner, err)
	}
	if !validVisualType[v.BlockType()] {
		return nil, fmt.Errorf("%s visual: type %q is not an allowed visual (code, diagram, image, diff, chart, term, filetree, record)", owner, v.BlockType())
	}
	return v, nil
}

// Choice is a single- or multi-select control; selecting an option approves it.
type Choice struct {
	base
	Prompt  string   `json:"prompt,omitempty"`
	Multi   bool     `json:"multi,omitempty"`
	Options []Option `json:"options"`
}

// Draft is an annotated document: a body of text a human marks up with anchored
// annotations. Lang tags the body for rendering; Title is an optional heading.
type Draft struct {
	base
	Lang  string `json:"lang"`
	Text  string `json:"text"`
	Title string `json:"title,omitempty"`
}

// Item is one entry in a Triage block, addressed by ID within its block. It is an
// Option without the Recommended flag; Visual is an optional restricted leaf from
// the same allowlist as an option's visual.
type Item struct {
	ID     string  `json:"id"`
	Label  string  `json:"label"`
	Hint   string  `json:"hint,omitempty"`
	Md     string  `json:"md,omitempty"`
	Facts  []Fact  `json:"facts,omitempty"`
	Detail *Detail `json:"detail,omitempty"`
	Visual Block   `json:"visual,omitempty"`
}

func decodeItem(data json.RawMessage, unmarshal unmarshalFunc) (Item, error) {
	var raw struct {
		ID     string          `json:"id"`
		Label  string          `json:"label"`
		Hint   string          `json:"hint"`
		Md     string          `json:"md"`
		Facts  []Fact          `json:"facts"`
		Detail *Detail         `json:"detail"`
		Visual json.RawMessage `json:"visual"`
	}
	if err := unmarshal(data, &raw); err != nil {
		return Item{}, fmt.Errorf("unmarshal item: %w", err)
	}
	v, err := decodeVisual(raw.Visual, fmt.Sprintf("item %q", raw.ID), unmarshal)
	if err != nil {
		return Item{}, err
	}
	return Item{
		ID:     raw.ID,
		Label:  raw.Label,
		Hint:   raw.Hint,
		Md:     raw.Md,
		Facts:  raw.Facts,
		Detail: raw.Detail,
		Visual: v,
	}, nil
}

// Triage is a multi-item accept/reject control; each item carries its own verdict,
// with optional per-item notes when AllowNotes permits (true when unset).
type Triage struct {
	base
	Prompt     string `json:"prompt,omitempty"`
	AllowNotes *bool  `json:"allowNotes,omitempty"`
	Items      []Item `json:"items"`
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

// Code is a syntax-highlighted code block. A block with Src is grounded in a
// repository file: the CLI fills Code, Start, Lang, and Sha from the Lines slice
// of Src on every push, and the browser renders a numbered gutter with Highlight
// ranges and Pins keyed to file line numbers.
type Code struct {
	base
	Lang      string    `json:"lang"`
	Code      string    `json:"code"`
	Title     string    `json:"title,omitempty"`
	Src       string    `json:"src,omitempty"`
	Lines     string    `json:"lines,omitempty"`
	Start     int       `json:"start,omitempty"`
	Highlight string    `json:"highlight,omitempty"`
	Pins      []CodePin `json:"pins,omitempty"`
	Sha       string    `json:"sha,omitempty"`
}

// CodePin is a numbered callout on one file line of a Code block.
type CodePin struct {
	Line  int    `json:"line"`
	Title string `json:"title"`
	Body  string `json:"body,omitempty"`
	Tone  string `json:"tone,omitempty"`
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

// PR is a GitHub pull request the daemon renders live from gh: Repo is
// owner/name, Title overrides the PR's own title, Summary is markdown under the
// heading, and Files narrows the changed-file list to matching paths.
type PR struct {
	base
	Repo    string   `json:"repo"`
	Number  int      `json:"number"`
	Title   string   `json:"title,omitempty"`
	Summary string   `json:"summary,omitempty"`
	Files   []string `json:"files,omitempty"`
}

// PRGroups is a set of GitHub pull requests in one repo, grouped by what they
// change, each carrying a head-pinned approve or hold verdict. Every row states
// what it changes for a user, how it differs from the plan, its risk, its review
// and CI state, and what blocks it from landing; a tap reads the PR live.
type PRGroups struct {
	base
	Repo   string    `json:"repo"`
	Prompt string    `json:"prompt,omitempty"`
	Slack  *string   `json:"slack,omitempty"`
	Groups []PRGroup `json:"groups"`
}

// PRGroup is one heading of related pull requests and one sentence on what the
// group changes for a user.
type PRGroup struct {
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	Summary string  `json:"summary"`
	PRs     []PRRow `json:"prs"`
}

// PRRow is one pull request in a group. Head is the commit the verdict is
// given at; Carried lists earlier heads whose verdict still stands for this
// head. Md is the row's full text, shown when the row is expanded.
type PRRow struct {
	Number  int      `json:"number"`
	Head    string   `json:"head"`
	Title   string   `json:"title"`
	Change  string   `json:"change"`
	Plan    string   `json:"plan"`
	Risk    string   `json:"risk"`
	Review  string   `json:"review"`
	Blocker string   `json:"blocker"`
	Md      string   `json:"md,omitempty"`
	Carried []string `json:"carried,omitempty"`
	Files   []string `json:"files,omitempty"`
}

// Thread is the Slack DM status thread the block's rows are asked in, or ""
// when the block is not bound to one.
func (g *PRGroups) Thread() string {
	if g.Slack == nil {
		return ""
	}
	return *g.Slack
}

// KeepSlack gives every incoming pr-groups block that states no slack the
// thread its namesake in cur is bound to, so a binding holds until a push
// states an empty slack. It reports whether it bound any block.
func KeepSlack(cur *Doc, blocks []Block) bool {
	kept := false
	for _, b := range blocks {
		Walk(b, func(b Block) {
			g, ok := b.(*PRGroups)
			if !ok || g.Slack != nil {
				return
			}
			loc, found := Locate(cur, g.ID)
			if !found {
				return
			}
			if prev, ok := loc.Block.(*PRGroups); ok && prev.Thread() != "" {
				g.Slack, kept = prev.Slack, true
			}
		})
	}
	return kept
}

// Stands reports whether a verdict given at head still stands for the row: it
// names the row's head or one the row carries.
func (r *PRRow) Stands(head string) bool {
	return SameHead(head, r.Head) || slices.ContainsFunc(r.Carried, func(c string) bool { return SameHead(head, c) })
}

// RowID is the verdict key of a pull request row: its number in decimal.
func RowID(number int) string { return strconv.Itoa(number) }

// Row returns the row whose verdict key is id.
func (g *PRGroups) Row(id string) (*PRRow, bool) {
	for i := range g.Groups {
		for j := range g.Groups[i].PRs {
			if r := &g.Groups[i].PRs[j]; RowID(r.Number) == id {
				return r, true
			}
		}
	}
	return nil, false
}

// ParseHead checks that s is a 7 to 40 character lowercase hex commit sha.
func ParseHead(s string) (string, error) {
	if !headPattern.MatchString(s) {
		return "", fmt.Errorf("head must be a 7 to 40 character lowercase hex sha, got %q", s)
	}
	return s, nil
}

// SameHead reports whether two commit shas name the same commit: the shorter
// is a prefix of the longer, so a 7-character head matches its full sha.
func SameHead(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if len(a) > len(b) {
		a, b = b, a
	}
	return strings.HasPrefix(b, a)
}

// UnmarshalJSON decodes the envelope and dispatches each block by its type tag.
func (d *Doc) UnmarshalJSON(data []byte) error {
	decoded, err := decodeDoc(data, decodeStrict)
	if err != nil {
		return err
	}
	*d = *decoded
	return nil
}

// DecodeStored decodes a document read back from the event log, ignoring any
// key the current schema does not define.
func DecodeStored(data []byte) (*Doc, error) {
	return decodeDoc(data, json.Unmarshal)
}

func decodeDoc(data []byte, unmarshal unmarshalFunc) (*Doc, error) {
	var raw struct {
		Version      int               `json:"version"`
		Title        string            `json:"title"`
		Intro        string            `json:"intro"`
		Stats        []Stat            `json:"stats"`
		Submit       *Submit           `json:"submit"`
		Presentation *string           `json:"presentation"`
		Sources      map[string]string `json:"sources"`
		Blocks       []json.RawMessage `json:"blocks"`
	}
	if err := unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal doc: %w (a doc's keys are version, title, intro, stats, submit, presentation, sources, and blocks; sections and cards are siblings in blocks)", err)
	}
	blocks, err := decodeBlocks(raw.Blocks, unmarshal)
	if err != nil {
		return nil, err
	}
	return &Doc{
		Version:      raw.Version,
		Title:        raw.Title,
		Intro:        raw.Intro,
		Stats:        raw.Stats,
		Submit:       raw.Submit,
		Presentation: raw.Presentation,
		Sources:      raw.Sources,
		Blocks:       blocks,
	}, nil
}

func decodeCard(data json.RawMessage, unmarshal unmarshalFunc) (Block, error) {
	var raw struct {
		base
		Title    string            `json:"title"`
		Summary  string            `json:"summary"`
		Chips    []Chip            `json:"chips"`
		Flagged  bool              `json:"flagged"`
		Status   string            `json:"status"`
		Slack    *CardSlack        `json:"slack"`
		Children []json.RawMessage `json:"children"`
	}
	if err := unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal card: %w", err)
	}
	children, err := decodeBlocks(raw.Children, unmarshal)
	if err != nil {
		return nil, err
	}
	return &Card{
		base:     raw.base,
		Title:    raw.Title,
		Summary:  raw.Summary,
		Chips:    raw.Chips,
		Flagged:  raw.Flagged,
		Status:   raw.Status,
		Slack:    raw.Slack,
		Children: children,
	}, nil
}

func decodeChoice(data json.RawMessage, unmarshal unmarshalFunc) (Block, error) {
	var raw struct {
		base
		Prompt  string            `json:"prompt"`
		Multi   bool              `json:"multi"`
		Options listField[Option] `json:"options"`
	}
	raw.Options = listField[Option]{unmarshal: unmarshal, decode: decodeOption}
	if err := unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return &Choice{base: raw.base, Prompt: raw.Prompt, Multi: raw.Multi, Options: raw.Options.values}, nil
}

func decodeTriage(data json.RawMessage, unmarshal unmarshalFunc) (Block, error) {
	var raw struct {
		base
		Prompt     string          `json:"prompt"`
		AllowNotes *bool           `json:"allowNotes"`
		Items      listField[Item] `json:"items"`
	}
	raw.Items = listField[Item]{unmarshal: unmarshal, decode: decodeItem}
	if err := unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return &Triage{base: raw.base, Prompt: raw.Prompt, AllowNotes: raw.AllowNotes, Items: raw.Items.values}, nil
}

type listField[T any] struct {
	unmarshal unmarshalFunc
	decode    func(json.RawMessage, unmarshalFunc) (T, error)
	values    []T
}

func (l *listField[T]) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw == nil {
		l.values = nil
		return nil
	}
	values := make([]T, 0, len(raw))
	for _, r := range raw {
		v, err := l.decode(r, l.unmarshal)
		if err != nil {
			return err
		}
		values = append(values, v)
	}
	l.values = values
	return nil
}

func decodeBlocks(raw []json.RawMessage, unmarshal unmarshalFunc) ([]Block, error) {
	blocks := make([]Block, 0, len(raw))
	for _, rb := range raw {
		b, err := decodeBlock(rb, unmarshal)
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
	blocks, err := decodeBlocks(raw, decodeStrict)
	if err != nil {
		return err
	}
	*bl = blocks
	return nil
}

// DecodeBlock decodes a single block from its JSON, dispatching on the type tag.
// A well-formed dotted type decodes to *PackBlock unconditionally; a malformed
// dotted type, an unknown dot-free type, and a field the type does not define are
// each an error naming the block id.
func DecodeBlock(data json.RawMessage) (Block, error) {
	return decodeBlock(data, decodeStrict)
}

// DecodeStoredBlock decodes a single block read back from the event log,
// ignoring any key its type does not define.
func DecodeStoredBlock(data json.RawMessage) (Block, error) {
	return decodeBlock(data, json.Unmarshal)
}

func decodeBlock(data json.RawMessage, unmarshal unmarshalFunc) (Block, error) {
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
	b, err := sp.decode(data, unmarshal)
	if err != nil {
		return nil, fmt.Errorf("block %q: decode %s block: %w", head.ID, head.Type, err)
	}
	return b, nil
}

func decodeLeaf[T any, B interface {
	*T
	Block
}](data json.RawMessage, unmarshal unmarshalFunc) (Block, error) {
	b := B(new(T))
	if err := unmarshal(data, b); err != nil {
		return nil, err
	}
	return b, nil
}

type unmarshalFunc func(data []byte, dst any) error

func decodeStrict(data []byte, dst any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
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
	for alias, src := range d.Sources {
		if !sourceAliasPattern.MatchString(alias) {
			errs = append(errs, fmt.Errorf("source alias %q must match %s", alias, sourceAliasPattern))
		}
		if src == "" {
			errs = append(errs, fmt.Errorf("source %q is empty; give a repo path@rev or an absolute file path", alias))
		}
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
	if c.Slack != nil && !strings.HasPrefix(c.Slack.URL, "https://") {
		return fmt.Errorf("card %q: slack url %q must be the Slack card's https permalink", c.ID, c.Slack.URL)
	}
	for _, chip := range c.Chips {
		if err := validateChip(chip); err != nil {
			return fmt.Errorf("card %q: %w", c.ID, err)
		}
	}
	return validateCardPrompts(c)
}

func validateCardPrompts(c *Card) error {
	var unprompted Block
	decisions := 0
	for _, child := range c.Children {
		var prompt string
		switch b := child.(type) {
		case *Choice:
			prompt = b.Prompt
		case *Approval:
			prompt = b.Prompt
		default:
			continue
		}
		decisions++
		if prompt == "" && unprompted == nil {
			unprompted = child
		}
	}
	if decisions < 2 || unprompted == nil {
		return nil
	}
	return fmt.Errorf("card %q: %s %q needs a prompt — a card with several decisions states each question in its own prompt, not in a record", c.ID, unprompted.BlockType(), unprompted.BlockID())
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

func validateDraft(d *Draft) error {
	if d.Lang == "" {
		return fmt.Errorf("draft %q: lang must not be empty", d.ID)
	}
	if d.Text == "" {
		return fmt.Errorf("draft %q: text must not be empty", d.ID)
	}
	if len(d.Text) > MaxDraftBytes {
		return fmt.Errorf("draft %q: text is %d bytes, exceeds %d", d.ID, len(d.Text), MaxDraftBytes)
	}
	if isMultiline(d.Title) {
		return fmt.Errorf("draft %q: title must be a single line", d.ID)
	}
	return nil
}

func validateTriage(t *Triage) error {
	if len(t.Items) == 0 {
		return fmt.Errorf("triage %q: must have at least one item", t.ID)
	}
	if len(t.Items) > MaxTriageItems {
		return fmt.Errorf("triage %q: %d items exceeds %d", t.ID, len(t.Items), MaxTriageItems)
	}
	seen := map[string]bool{}
	for _, item := range t.Items {
		if item.ID == "" {
			return fmt.Errorf("triage %q: item id must not be empty", t.ID)
		}
		if seen[item.ID] {
			return fmt.Errorf("triage %q: duplicate item id %q", t.ID, item.ID)
		}
		seen[item.ID] = true
		if item.Label == "" {
			return fmt.Errorf("triage %q: item %q label must not be empty", t.ID, item.ID)
		}
		if isMultiline(item.Hint) {
			return fmt.Errorf("triage %q: item %q hint must be a single line", t.ID, item.ID)
		}
		for _, f := range item.Facts {
			if err := validateFact(f); err != nil {
				return fmt.Errorf("triage %q: item %q: %w", t.ID, item.ID, err)
			}
		}
		if err := validateDetail(item.Detail); err != nil {
			return fmt.Errorf("triage %q: item %q: %w", t.ID, item.ID, err)
		}
		if item.Visual != nil {
			if err := validateVisual(item.Visual); err != nil {
				return fmt.Errorf("triage %q: item %q visual: %w", t.ID, item.ID, err)
			}
		}
	}
	return nil
}

func validateCode(c *Code) error {
	if c.Src != "" && c.Code == "" {
		return fmt.Errorf("code %q: src %q is unfilled; it is filled by push/update-block/start --doc", c.ID, c.Src)
	}
	if c.Lang == "" {
		return fmt.Errorf("code %q: lang must not be empty", c.ID)
	}
	if c.Code == "" {
		return fmt.Errorf("code %q: code must not be empty", c.ID)
	}
	if err := validateCodeSource(c); err != nil {
		return fmt.Errorf("code %q: %w", c.ID, err)
	}
	first, last, err := codeSpan(c)
	if err != nil {
		return fmt.Errorf("code %q: %w", c.ID, err)
	}
	if c.Highlight != "" {
		for r := range strings.SplitSeq(c.Highlight, ",") {
			a, b, err := ParseLineRange(r)
			if err != nil {
				return fmt.Errorf("code %q: highlight: %w", c.ID, err)
			}
			if a < first || b > last {
				return fmt.Errorf("code %q: highlight %q falls outside lines %d-%d", c.ID, r, first, last)
			}
		}
	}
	for _, p := range c.Pins {
		if p.Line < first || p.Line > last {
			return fmt.Errorf("code %q: pin on line %d falls outside lines %d-%d", c.ID, p.Line, first, last)
		}
		if p.Title == "" {
			return fmt.Errorf("code %q: pin on line %d: title must not be empty", c.ID, p.Line)
		}
		if isMultiline(p.Title) {
			return fmt.Errorf("code %q: pin on line %d: title must be a single line", c.ID, p.Line)
		}
		if p.Tone != "" && !validFactTone[p.Tone] {
			return fmt.Errorf("code %q: pin on line %d: tone must be default, good, warn, or bad, got %q", c.ID, p.Line, p.Tone)
		}
	}
	return nil
}

func validateCodeSource(c *Code) error {
	if c.Src == "" {
		switch {
		case c.Lines != "":
			return errors.New("lines requires src")
		case c.Sha != "":
			return errors.New("sha requires src")
		}
		return nil
	}
	if err := validateTreePath(c.Src); err != nil {
		return fmt.Errorf("src: %w", err)
	}
	if c.Sha != "" && !codeShaPattern.MatchString(c.Sha) {
		return fmt.Errorf("sha %q must be a hex commit, optionally suffixed +wt", c.Sha)
	}
	return nil
}

func codeSpan(c *Code) (int, int, error) {
	if c.Start < 0 || c.Start > MaxCodeLine {
		return 0, 0, fmt.Errorf("start %d must be between 1 and %d", c.Start, MaxCodeLine)
	}
	n := strings.Count(c.Code, "\n") + 1
	if c.Lines == "" {
		first := max(c.Start, 1)
		return first, first + n - 1, nil
	}
	a, b, err := ParseLineRange(c.Lines)
	if err != nil {
		return 0, 0, fmt.Errorf("lines: %w", err)
	}
	if c.Start != 0 && c.Start != a {
		return 0, 0, fmt.Errorf("start %d must match lines %q", c.Start, c.Lines)
	}
	if b-a+1 != n {
		return 0, 0, fmt.Errorf("lines %q spans %d lines but code has %d", c.Lines, b-a+1, n)
	}
	return a, b, nil
}

// ParseLineRange parses a 1-based inclusive line range, "40" or "40-72".
func ParseLineRange(s string) (int, int, error) {
	m := lineRangePattern.FindStringSubmatch(s)
	if m == nil {
		return 0, 0, fmt.Errorf("line range %q must look like 40 or 40-72", s)
	}
	a, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, 0, fmt.Errorf("line range %q: %w", s, err)
	}
	b := a
	if m[2] != "" {
		if b, err = strconv.Atoi(m[2]); err != nil {
			return 0, 0, fmt.Errorf("line range %q: %w", s, err)
		}
	}
	if b < a {
		return 0, 0, fmt.Errorf("line range %q ends before it starts", s)
	}
	if b > MaxCodeLine {
		return 0, 0, fmt.Errorf("line range %q exceeds line %d", s, MaxCodeLine)
	}
	return a, b, nil
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

func validatePRGroups(g *PRGroups) error {
	if !repoPattern.MatchString(g.Repo) {
		return fmt.Errorf("pr-groups %q: repo must be owner/name, got %q", g.ID, g.Repo)
	}
	if isMultiline(g.Prompt) {
		return fmt.Errorf("pr-groups %q: prompt must be a single line", g.ID)
	}
	if t := g.Thread(); t != "" && !slackTSPattern.MatchString(t) {
		return fmt.Errorf("pr-groups %q: slack must be the ts of a Slack DM status thread, or empty to unbind, got %q", g.ID, t)
	}
	if len(g.Groups) == 0 {
		return fmt.Errorf("pr-groups %q: must have at least one group", g.ID)
	}
	if len(g.Groups) > MaxPRGroups {
		return fmt.Errorf("pr-groups %q: %d groups exceeds %d", g.ID, len(g.Groups), MaxPRGroups)
	}
	groupIDs := map[string]bool{}
	numbers := map[int]bool{}
	for _, gr := range g.Groups {
		if gr.ID == "" {
			return fmt.Errorf("pr-groups %q: group id must not be empty", g.ID)
		}
		if groupIDs[gr.ID] {
			return fmt.Errorf("pr-groups %q: duplicate group id %q", g.ID, gr.ID)
		}
		groupIDs[gr.ID] = true
		if gr.Title == "" || isMultiline(gr.Title) {
			return fmt.Errorf("pr-groups %q: group %q title must be one non-empty line", g.ID, gr.ID)
		}
		if gr.Summary == "" || isMultiline(gr.Summary) {
			return fmt.Errorf("pr-groups %q: group %q summary must be one non-empty line saying what the group changes for a user", g.ID, gr.ID)
		}
		if len(gr.PRs) == 0 {
			return fmt.Errorf("pr-groups %q: group %q must have at least one pull request", g.ID, gr.ID)
		}
		for i := range gr.PRs {
			r := &gr.PRs[i]
			if r.Number < 1 {
				return fmt.Errorf("pr-groups %q: group %q: number must be a positive integer", g.ID, gr.ID)
			}
			if numbers[r.Number] {
				return fmt.Errorf("pr-groups %q: pull request #%d appears more than once", g.ID, r.Number)
			}
			numbers[r.Number] = true
			if err := validatePRRow(r); err != nil {
				return fmt.Errorf("pr-groups %q: #%d %w", g.ID, r.Number, err)
			}
		}
	}
	if len(numbers) > MaxPRGroupRows {
		return fmt.Errorf("pr-groups %q: %d pull requests exceeds %d", g.ID, len(numbers), MaxPRGroupRows)
	}
	return nil
}

func validatePRRow(r *PRRow) error {
	if _, err := ParseHead(r.Head); err != nil {
		return err
	}
	for _, c := range r.Carried {
		if _, err := ParseHead(c); err != nil {
			return fmt.Errorf("carried %w", err)
		}
	}
	fields := []struct{ name, value string }{
		{"title", r.Title}, {"change", r.Change}, {"plan", r.Plan}, {"risk", r.Risk}, {"review", r.Review}, {"blocker", r.Blocker},
	}
	for _, f := range fields {
		if f.value == "" {
			return fmt.Errorf("%s must not be empty", f.name)
		}
		if isMultiline(f.value) {
			return fmt.Errorf("%s must be a single line", f.name)
		}
	}
	if len(r.Files) > MaxPRFiles {
		return fmt.Errorf("%d files patterns exceeds %d", len(r.Files), MaxPRFiles)
	}
	for _, f := range r.Files {
		if _, err := path.Match(f, ""); f == "" || err != nil {
			return fmt.Errorf("files pattern %q is not a valid glob", f)
		}
	}
	return nil
}

func validatePR(p *PR) error {
	if !repoPattern.MatchString(p.Repo) {
		return fmt.Errorf("pr %q: repo must be owner/name, got %q", p.ID, p.Repo)
	}
	if p.Number < 1 {
		return fmt.Errorf("pr %q: number must be a positive integer", p.ID)
	}
	if isMultiline(p.Title) {
		return fmt.Errorf("pr %q: title must be a single line", p.ID)
	}
	if len(p.Files) > MaxPRFiles {
		return fmt.Errorf("pr %q: %d files patterns exceeds %d", p.ID, len(p.Files), MaxPRFiles)
	}
	for _, f := range p.Files {
		if _, err := path.Match(f, ""); f == "" || err != nil {
			return fmt.Errorf("pr %q: files pattern %q is not a valid glob", p.ID, f)
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

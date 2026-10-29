package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yasyf/cc-interact/cmd"
	ccevent "github.com/yasyf/cc-interact/event"

	ccdaemon "github.com/yasyf/cc-present/internal/daemon"
	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/state"
)

func newBlockCmd(d cmd.Deps) *cobra.Command {
	c := &cobra.Command{
		Use:   "block",
		Short: "Read one block of the board as plain text",
		Args:  cobra.NoArgs,
	}
	c.AddCommand(newBlockShowCmd(d))
	return c
}

func newBlockShowCmd(d cmd.Deps) *cobra.Command {
	var session, cwd string
	c := &cobra.Command{
		Use:   "show <blockId>",
		Short: "Print a block's question, its options, and the human's answers, newest first",
		Long: `Prints the block's card title and summary, its prompt and options, then every
answer, verdict, note and feedback on it, newest first with the time each was
given. A block no longer on the board prints as the board last showed it. A
card id prints every child and the answers on all of them.`,
		Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			cl, sess, pid, done, err := verdictClient(c, d, session)
			if err != nil {
				return err
			}
			defer done()
			show, err := cl.BlockShow(c.Context(), sess, mustCwd(cwd), pid, args[0])
			if err != nil {
				return err
			}
			return writeBlockShow(c.OutOrStdout(), args[0], show)
		},
	}
	verdictFlags(c, &session, &cwd)
	return c
}

func writeBlockShow(w io.Writer, id string, show ccdaemon.BlockShow) error {
	var b strings.Builder
	if string(show.Top) == "null" {
		fmt.Fprintf(&b, "%s: no write carried this block, only its answers\n", id)
	} else {
		top, err := doc.DecodeStoredBlock(show.Top)
		if err != nil {
			return err
		}
		if card, ok := top.(*doc.Card); ok {
			fmt.Fprintf(&b, "%s: %s\n", card.ID, card.Title)
			indent(&b, card.Summary)
		}
		if !show.OnBoard {
			b.WriteString("  No longer on the board; shown as it last was.\n")
		}
		for _, leaf := range append([]doc.Block{top}, doc.Children(top)...) {
			if leaf.BlockType() != "card" && (leaf.BlockID() == id || top.BlockID() == id) {
				writeLeaf(&b, leaf)
			}
		}
	}
	b.WriteString("\nAnswers, newest first:\n")
	if len(show.Events) == 0 {
		b.WriteString("  none\n")
	}
	for _, e := range show.Events {
		line, err := describeEvent(e)
		if err != nil {
			return err
		}
		fmt.Fprintf(&b, "  %s  %s\n", e.At.Local().Format("Jan 2 3:04 PM MST"), line)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func writeLeaf(b *strings.Builder, leaf doc.Block) {
	switch l := leaf.(type) {
	case *doc.Choice:
		pick := "pick one"
		if l.Multi {
			pick = "pick any"
		}
		fmt.Fprintf(b, "\n%s: choice, %s\n", l.ID, pick)
		indent(b, l.Prompt)
		for _, o := range l.Options {
			rec := ""
			if o.Recommended {
				rec = " (recommended)"
			}
			fmt.Fprintf(b, "  - %s: %s%s\n", o.ID, o.Label, rec)
			indent(b, "  "+o.Hint)
		}
	case *doc.Approval:
		fmt.Fprintf(b, "\n%s: approval\n", l.ID)
		indent(b, l.Prompt)
	case *doc.Triage:
		fmt.Fprintf(b, "\n%s: triage\n", l.ID)
		indent(b, l.Prompt)
		for _, it := range l.Items {
			fmt.Fprintf(b, "  - %s: %s\n", it.ID, it.Label)
			indent(b, "  "+it.Hint)
		}
	case *doc.Input:
		fmt.Fprintf(b, "\n%s: input\n", l.ID)
		indent(b, l.Label)
	default:
		fmt.Fprintf(b, "\n%s: %s\n", leaf.BlockID(), leaf.BlockType())
	}
}

func indent(b *strings.Builder, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	fmt.Fprintf(b, "  %s\n", strings.ReplaceAll(text, "\n", "\n  "))
}

func describeEvent(e ccdaemon.BlockEvent) (string, error) {
	var p struct {
		BlockID   string                    `json:"blockId"`
		Verdict   string                    `json:"verdict"`
		Note      string                    `json:"note"`
		OptionIDs []string                  `json:"optionIds"`
		Other     string                    `json:"other"`
		Text      string                    `json:"text"`
		Quote     string                    `json:"quote"`
		Verdicts  map[string]state.Decision `json:"verdicts"`
		Payload   json.RawMessage           `json:"payload"`
	}
	if err := json.Unmarshal(e.Payload, &p); err != nil {
		return "", fmt.Errorf("read %s: %w", e.Type, err)
	}
	var what string
	switch e.Type {
	case ccdaemon.EventDecisionCreated:
		what = withNote(p.Verdict, p.Note)
	case ccdaemon.EventChoiceSelected:
		parts := []string{}
		if len(p.OptionIDs) > 0 {
			parts = append(parts, "picked "+strings.Join(p.OptionIDs, ", "))
		}
		if p.Other != "" {
			parts = append(parts, "wrote in: "+p.Other)
		}
		if len(parts) == 0 {
			parts = append(parts, "cleared the pick")
		}
		what = strings.Join(parts, "; ")
	case ccdaemon.EventFeedbackCreated:
		what = "feedback: " + p.Text
	case ccdaemon.EventInputSubmitted:
		what = "answered: " + p.Text
	case ccdaemon.EventAnnotationCreated:
		what = fmt.Sprintf("annotated %q: %s", p.Quote, p.Text)
	case ccdaemon.EventTriageDecided:
		items := make([]string, 0, len(p.Verdicts))
		for _, item := range slices.Sorted(maps.Keys(p.Verdicts)) {
			items = append(items, item+" "+withNote(p.Verdicts[item].Verdict, p.Verdicts[item].Note))
		}
		what = strings.Join(items, "; ")
	default:
		what = "pack: " + string(p.Payload)
	}
	if e.Origin == ccevent.OriginAgent {
		what += " (recorded by the agent)"
	}
	return strings.ReplaceAll(p.BlockID+"  "+what, "\n", "\n    "), nil
}

func withNote(verdict, note string) string {
	if note == "" {
		return verdict
	}
	return verdict + ": " + note
}

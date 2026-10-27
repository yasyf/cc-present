package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yasyf/cc-interact/cmd"

	ccdaemon "github.com/yasyf/cc-present/internal/daemon"
)

// newVerdictCmd groups the agent-side verdict verbs: importing verdicts from an
// earlier board onto pr-groups rows, carrying a row's verdict to a new head that
// is the same change, and moving or copying a recorded verdict to another row so
// a regrouped board keeps the human's taps.
func newVerdictCmd(d cmd.Deps) *cobra.Command {
	c := &cobra.Command{
		Use:   "verdict",
		Short: "Import, carry, move, or copy the human's recorded verdicts",
		Args:  cobra.NoArgs,
	}
	c.AddCommand(newVerdictImportCmd(d), newVerdictCarryCmd(d), newVerdictMoveCmd(d, false), newVerdictMoveCmd(d, true))
	return c
}

func newVerdictImportCmd(d cmd.Deps) *cobra.Command {
	var session, cwd, file string
	var replace bool
	c := &cobra.Command{
		Use:   "import",
		Short: "Carry verdicts from an earlier board onto pr-groups rows",
		Long: `Reads a JSON array of {block, row, verdict, at, note?, head?, carriedFrom?,
carryNote?} and records each as the row's verdict: block is the pr-groups block
id, row the PR number, verdict "approved" or "rejected" (Hold), at the RFC 3339
time it was given. head defaults to the row's current head; an older head shows
on the board as needing a fresh tap. carriedFrom names the head the human
tapped when it differs from head, and carryNote says why the tap still stands.
A row that already has a verdict keeps it unless --replace is set.`,
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			if file == "" {
				return errors.New("verdict import requires --file (or - for stdin)")
			}
			raw, err := readInput(file, c.InOrStdin())
			if err != nil {
				return err
			}
			if !json.Valid(raw) {
				return errors.New("verdict import: --file is not valid JSON")
			}
			cl, sess, pid, done, err := verdictClient(c, d, session)
			if err != nil {
				return err
			}
			defer done()
			res, err := cl.ImportVerdicts(c.Context(), sess, mustCwd(cwd), pid, raw, replace)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(c.OutOrStdout(), "imported: %d\n", res.Imported)
			if len(res.Skipped) > 0 {
				_, _ = fmt.Fprintf(c.OutOrStdout(), "kept existing: %s\n", strings.Join(res.Skipped, " "))
			}
			if len(res.Replaced) > 0 {
				_, _ = fmt.Fprintf(c.OutOrStdout(), "replaced: %s\n", strings.Join(res.Replaced, " "))
			}
			return nil
		},
	}
	c.Flags().StringVar(&file, "file", "", "JSON array of prior verdicts, or - for stdin")
	c.Flags().BoolVar(&replace, "replace", false, "overwrite a row's existing verdict instead of keeping it")
	verdictFlags(c, &session, &cwd)
	return c
}

func newVerdictCarryCmd(d cmd.Deps) *cobra.Command {
	var session, cwd string
	var carry ccdaemon.VerdictCarry
	c := &cobra.Command{
		Use:   "carry",
		Short: "Carry a pr-groups row's verdict to a new head that is the same change",
		Long: `Records the verdict the human gave on a pr-groups row as standing at --head,
for a new commit the agent judged the same change (a patch-identical replay or
restack). The row keeps the head and time the human tapped, and the board shows
"your tap at <old> carries to <new>: <note>" instead of asking again. The
original tap stays in the board's history.`,
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			cl, sess, pid, done, err := verdictClient(c, d, session)
			if err != nil {
				return err
			}
			defer done()
			res, err := cl.CarryVerdict(c.Context(), sess, mustCwd(cwd), pid, carry)
			if err != nil {
				return err
			}
			if res.Already {
				_, _ = fmt.Fprintf(c.OutOrStdout(), "%s/%s: already stands at %s\n", carry.Block, carry.Row, res.To)
				return nil
			}
			_, _ = fmt.Fprintf(c.OutOrStdout(), "%s/%s: %s at %s (%s) carries to %s\n", carry.Block, carry.Row, res.Verdict, res.From, res.At, res.To)
			return nil
		},
	}
	c.Flags().StringVar(&carry.Block, "block", "", "pr-groups block id")
	c.Flags().StringVar(&carry.Row, "row", "", "pull request number")
	c.Flags().StringVar(&carry.Head, "head", "", "the new head the verdict stands at")
	c.Flags().StringVar(&carry.Note, "note", "", "why the tap still stands at the new head")
	for _, f := range []string{"block", "row", "head", "note"} {
		_ = c.MarkFlagRequired(f)
	}
	verdictFlags(c, &session, &cwd)
	return c
}

func newVerdictMoveCmd(d cmd.Deps, keep bool) *cobra.Command {
	var session, cwd string
	var from, to []string
	use, past, short := "move", "moved", "Move recorded verdicts to other rows, clearing each source"
	if keep {
		use, past, short = "copy", "copied", "Copy recorded verdicts to other rows, keeping each source"
	}
	c := &cobra.Command{
		Use:   use,
		Short: short,
		Long: short + `. Each --from <block>/<row> pairs with the --to <block>/<row> at the same
position. A row is a triage item id or a pr-groups PR number; the target block
must be live in the current round. A target that already has a verdict is
refused, and nothing is written unless every pair is valid.`,
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			moves, err := verdictMoves(from, to, keep)
			if err != nil {
				return err
			}
			raw, err := json.Marshal(moves)
			if err != nil {
				return err
			}
			cl, sess, pid, done, err := verdictClient(c, d, session)
			if err != nil {
				return err
			}
			defer done()
			if err := cl.MoveVerdicts(c.Context(), sess, mustCwd(cwd), pid, raw); err != nil {
				return err
			}
			for _, m := range moves {
				_, _ = fmt.Fprintf(c.OutOrStdout(), "%s: %s/%s -> %s/%s\n", past, m.From.Block, m.From.Row, m.To.Block, m.To.Row)
			}
			return nil
		},
	}
	c.Flags().StringArrayVar(&from, "from", nil, "source <block>/<row> (repeatable)")
	c.Flags().StringArrayVar(&to, "to", nil, "target <block>/<row>, paired with --from by position (repeatable)")
	verdictFlags(c, &session, &cwd)
	return c
}

func verdictMoves(from, to []string, keep bool) ([]ccdaemon.VerdictMove, error) {
	if len(from) == 0 || len(from) != len(to) {
		return nil, fmt.Errorf("give one --to for each --from (got %d --from, %d --to)", len(from), len(to))
	}
	moves := make([]ccdaemon.VerdictMove, len(from))
	for i := range from {
		src, err := verdictRef(from[i])
		if err != nil {
			return nil, err
		}
		dst, err := verdictRef(to[i])
		if err != nil {
			return nil, err
		}
		moves[i] = ccdaemon.VerdictMove{From: src, To: dst, Keep: keep}
	}
	return moves, nil
}

func verdictRef(s string) (ccdaemon.VerdictRef, error) {
	block, row, ok := strings.Cut(s, "/")
	if !ok || block == "" || row == "" {
		return ccdaemon.VerdictRef{}, fmt.Errorf("%q is not <block>/<row>", s)
	}
	return ccdaemon.VerdictRef{Block: block, Row: row}, nil
}

func verdictClient(c *cobra.Command, d cmd.Deps, session string) (*ccdaemon.Client, string, int, func(), error) {
	ctx := c.Context()
	if err := d.EnsureCurrent(ctx); err != nil {
		return nil, "", 0, nil, err
	}
	cl, err := client(ctx, d)
	if err != nil {
		return nil, "", 0, nil, err
	}
	sess, pid := artifactWindow(d, session)
	return cl, sess, pid, func() { _ = cl.CloseSession() }, nil
}

func verdictFlags(c *cobra.Command, session, cwd *string) {
	c.Flags().StringVar(session, "session", "", "Claude session id (defaults to $CLAUDE_CODE_SESSION_ID)")
	c.Flags().StringVar(cwd, "cwd", "", "working directory (recorded on the request; artifacts are per-window, not resolved by directory)")
}

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yasyf/cc-interact/cmd"
)

// newImportVerdictsCmd carries verdicts given on an earlier board onto the
// window's pr-groups rows, so the human is not asked again.
func newImportVerdictsCmd(d cmd.Deps) *cobra.Command {
	var session, cwd, file string
	c := &cobra.Command{
		Use:   "import-verdicts",
		Short: "Carry verdicts from an earlier board onto pr-groups rows",
		Long: `Reads a JSON array of {block, row, verdict, at, note?, head?} and records each
as the row's verdict: block is the pr-groups block id, row the PR number,
verdict "approved" or "rejected" (Hold), at the RFC 3339 time it was given.
head defaults to the row's current head; an older head shows on the board as
needing a fresh tap. A row that already has a verdict keeps it.`,
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			if file == "" {
				return errors.New("import-verdicts requires --file (or - for stdin)")
			}
			raw, err := readInput(file, c.InOrStdin())
			if err != nil {
				return err
			}
			if !json.Valid(raw) {
				return errors.New("import-verdicts: --file is not valid JSON")
			}
			ctx := c.Context()
			if err := d.EnsureCurrent(ctx); err != nil {
				return err
			}
			cl, err := client(ctx, d)
			if err != nil {
				return err
			}
			defer func() { _ = cl.CloseSession() }()
			sess, pid := artifactWindow(d, session)
			res, err := cl.ImportVerdicts(ctx, sess, mustCwd(cwd), pid, raw)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(c.OutOrStdout(), "imported: %d\n", res.Imported)
			if len(res.Skipped) > 0 {
				_, _ = fmt.Fprintf(c.OutOrStdout(), "kept existing: %s\n", strings.Join(res.Skipped, " "))
			}
			return nil
		},
	}
	c.Flags().StringVar(&file, "file", "", "JSON array of prior verdicts, or - for stdin")
	c.Flags().StringVar(&session, "session", "", "Claude session id (defaults to $CLAUDE_CODE_SESSION_ID)")
	c.Flags().StringVar(&cwd, "cwd", "", "working directory (recorded on the request; artifacts are per-window, not resolved by directory)")
	return c
}

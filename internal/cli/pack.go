package cli

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/yasyf/cc-present/internal/app"
	"github.com/yasyf/cc-present/internal/packs"
)

// newPackCmd groups the local pack-authoring commands: list and lint.
func newPackCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "pack",
		Short: "Inspect and lint block packs",
		Args:  cobra.NoArgs,
	}
	c.AddCommand(newPackListCmd(), newPackLintCmd())
	return c
}

// newPackListCmd prints the packs discovered from the host config and installed
// plugins, each block's dotted type, the reference-fragment path to Read, and the
// dropped candidates with their reasons.
func newPackListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List discovered block packs and dropped candidates",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			cfg, err := app.ReadConfig()
			if err != nil {
				return err
			}
			printPacks(c.OutOrStdout(), packs.Load(cfg.PackDirs, cfg.DisabledPacks))
			return nil
		},
	}
}

// newPackLintCmd validates a pack root fail-loud and prints a one-line summary;
// the first violation is returned so cc-present exits non-zero.
func newPackLintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lint <dir>",
		Short: "Validate a pack root fail-loud (manifest, schemas, examples)",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			p, err := packs.Lint(args[0])
			if err != nil {
				return fmt.Errorf("pack lint %s: %w", args[0], err)
			}
			_, _ = fmt.Fprintf(c.OutOrStdout(), "ok: %s %s (%d blocks)\n", p.Name, p.Version, len(p.Blocks))
			return nil
		},
	}
}

func printPacks(w io.Writer, reg *packs.Registry) {
	installed := reg.Packs()
	if len(installed) == 0 {
		_, _ = fmt.Fprintln(w, "no packs installed")
	}
	for _, p := range installed {
		_, _ = fmt.Fprintf(w, "%s %s\n", p.Name, p.Version)
		_, _ = fmt.Fprintf(w, "  dir: %s\n", p.Dir)
		if p.Reference != "" {
			abs, _ := filepath.Abs(filepath.Join(p.Dir, p.Reference))
			_, _ = fmt.Fprintf(w, "  reference: %s\n", abs)
		}
		_, _ = fmt.Fprintln(w, "  blocks:")
		for _, bt := range p.Blocks {
			marker := ""
			if bt.Interactive() {
				marker = " (interactive)"
			}
			_, _ = fmt.Fprintf(w, "    %s%s\n", bt.FullType(), marker)
		}
	}
	if len(reg.Dropped) > 0 {
		_, _ = fmt.Fprintln(w, "dropped:")
		for _, d := range reg.Dropped {
			_, _ = fmt.Fprintf(w, "  %s: %s\n", d.Dir, d.Reason)
		}
	}
}

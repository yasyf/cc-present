package cli

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/yasyf/cc-present/examples"
	"github.com/yasyf/cc-present/internal/app"
	"github.com/yasyf/cc-present/internal/packs"
)

// newPackCmd groups the local pack-authoring commands: list, lint, and init.
func newPackCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "pack",
		Short: "Inspect and lint block packs",
		Args:  cobra.NoArgs,
	}
	c.AddCommand(newPackListCmd(), newPackLintCmd(), newPackInitCmd())
	return c
}

// newPackInitCmd scaffolds a new block pack into a directory from the embedded
// reference pack, renaming it to the given (or dir-derived) name.
func newPackInitCmd() *cobra.Command {
	var name string
	c := &cobra.Command{
		Use:   "init [--name <n>] <dir>",
		Short: "Scaffold a new block pack from the reference pack",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			dir := args[0]
			if name == "" {
				name = filepath.Base(filepath.Clean(dir))
				if err := packs.ValidateName(name); err != nil {
					return fmt.Errorf("pack init %s: derived name %q is invalid (%w); pass --name", dir, name, err)
				}
			}
			written, err := packs.Scaffold(dir, name, examples.ExamplePack, examples.ExamplePackRoot)
			if err != nil {
				return fmt.Errorf("pack init %s: %w", dir, err)
			}
			abs, err := filepath.Abs(dir)
			if err != nil {
				return fmt.Errorf("pack init %s: %w", dir, err)
			}
			w := c.OutOrStdout()
			_, _ = fmt.Fprintf(w, "scaffolded pack %q into %s (%d files)\n\n", name, dir, len(written))
			_, _ = fmt.Fprintln(w, "next steps:")
			_, _ = fmt.Fprintf(w, "  cd %s\n", dir)
			_, _ = fmt.Fprintln(w, "  bun install")
			_, _ = fmt.Fprintln(w, "  bun run build          # builds dist/pack.js, which pack lint needs")
			_, _ = fmt.Fprintln(w, "  bun run smoke")
			_, _ = fmt.Fprintln(w, "  cc-present pack lint .")
			_, _ = fmt.Fprintln(w, "\nregister it for local dev by adding this absolute path to packDirs in ~/.cc-present/config.json:")
			_, _ = fmt.Fprintf(w, "  %s\n", abs)
			return nil
		},
	}
	c.Flags().StringVar(&name, "name", "", "pack name (defaults to the target dir's basename)")
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

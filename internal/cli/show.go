package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yasyf/cc-interact/cmd"
)

// newShowCmd opens a one-off HTML, SVG, or Markdown file as this window's
// artifact through the plugin's display pack, so a session can show a
// component without authoring a pack.
func newShowCmd(d cmd.Deps) *cobra.Command {
	var session, cwd, title string
	var height int
	var fresh, replace bool
	c := &cobra.Command{
		Use:   "show <file.html|file.svg|file.md>",
		Short: "Show a one-off HTML, SVG, or Markdown file as this window's artifact",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			//nolint:gosec // G304: reading the file the user named is the command's purpose.
			raw, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			docJSON, err := showDoc(args[0], raw, title, height)
			if err != nil {
				return err
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
			res, err := cl.Start(ctx, sessionOr(session), mustCwd(cwd), d.ClaudePID(), startMode(fresh, replace), "", docJSON)
			if err != nil {
				return err
			}
			printStart(c, res.SubjectID, res.URL, res.TailnetURLs, res.ChannelState)
			return nil
		},
	}
	c.Flags().StringVar(&session, "session", "", "Claude session id (defaults to $CLAUDE_CODE_SESSION_ID)")
	c.Flags().StringVar(&cwd, "cwd", "", "working directory (recorded on the request; artifacts are per-window, not resolved by directory)")
	c.Flags().StringVar(&title, "title", "", "artifact title (defaults to the file name)")
	c.Flags().IntVar(&height, "height", 0, "fixed frame height in pixels for HTML and SVG (default: size to content)")
	c.Flags().BoolVar(&fresh, "new", false, "start a fresh artifact; refuses while this session has an open one")
	c.Flags().BoolVar(&replace, "replace", false, "close this session's open artifact and start a fresh one")
	return c
}

func showDoc(path string, raw []byte, title string, height int) (json.RawMessage, error) {
	if title == "" {
		title = filepath.Base(path)
	}
	var block map[string]any
	switch ext := strings.ToLower(filepath.Ext(path)); ext {
	case ".html", ".htm", ".svg":
		kind := "html"
		if ext == ".svg" {
			kind = "svg"
		}
		block = map[string]any{"id": "artifact", "type": "display.artifact", "kind": kind, "source": string(raw)}
		if height != 0 {
			block["height"] = height
		}
	case ".md", ".markdown":
		if height != 0 {
			return nil, fmt.Errorf("show: --height applies to HTML and SVG files, not %s", ext)
		}
		block = map[string]any{"id": "page", "type": "display.page", "md": string(raw)}
	default:
		return nil, fmt.Errorf("show: unsupported file type %q (want .html, .htm, .svg, .md, or .markdown)", ext)
	}
	return json.Marshal(map[string]any{"version": 1, "title": title, "blocks": []any{block}})
}

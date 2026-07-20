package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/yasyf/cc-interact/cmd"
)

// sessionInfo is one row of the daemon's GET /api/sessions listing, decoded to
// the fields the sessions command renders.
type sessionInfo struct {
	Subject    string `json:"subject"`
	Slug       string `json:"slug"`
	SessionID  string `json:"sessionId"`
	Status     string `json:"status"`
	EventCount int    `json:"eventCount"`
}

// newSessionsCmd lists every artifact the live daemon knows — open and closed —
// with each one's bound session, status, event count, and URL, under a header
// naming the daemon build and port.
func newSessionsCmd(d cmd.Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "sessions",
		Short: "List every artifact the daemon knows, with status, events, and URL",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			ctx := c.Context()
			if err := d.EnsureCurrent(ctx); err != nil {
				return err
			}
			cl, err := client(ctx, d)
			if err != nil {
				return err
			}
			defer func() { _ = cl.CloseSession() }()
			_, port, err := cl.Resolve(ctx, sessionOr(""), mustCwd(""), d.ClaudePID())
			if err != nil {
				return err
			}
			version, err := daemonVersion(port)
			if err != nil {
				return err
			}
			sessions, err := fetchSessions(port)
			if err != nil {
				return err
			}
			renderSessions(c.OutOrStdout(), version, port, sessions)
			return nil
		},
	}
}

// daemonVersion reads the daemon's build version from its health endpoint.
func daemonVersion(port int) (string, error) {
	var out struct {
		Version string `json:"version"`
	}
	if err := getJSON(fmt.Sprintf("http://127.0.0.1:%d/api/health", port), &out); err != nil {
		return "", err
	}
	return out.Version, nil
}

// fetchSessions reads every artifact the daemon knows, closed ones included.
func fetchSessions(port int) ([]sessionInfo, error) {
	var out []sessionInfo
	if err := getJSON(fmt.Sprintf("http://127.0.0.1:%d/api/sessions?all=true", port), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// getJSON decodes the JSON body of a GET against the loopback daemon into out.
func getJSON(url string, out any) error {
	//nolint:gosec // G107: the URL targets the loopback daemon on a port the control plane just returned.
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GET %s: %s: %s", url, resp.Status, strings.TrimSpace(string(msg)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// renderSessions writes the header naming the daemon build and port, then a
// tab-aligned table of the artifacts, or a single empty-state line when none.
func renderSessions(w io.Writer, version string, port int, sessions []sessionInfo) {
	_, _ = fmt.Fprintf(w, "cc-present daemon %s · port %d\n\n", version, port)
	if len(sessions) == 0 {
		_, _ = fmt.Fprintln(w, "no artifacts")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "SUBJECT\tSLUG\tSESSION\tSTATUS\tEVENTS\tURL")
	for _, row := range sessionRows(port, sessions) {
		_, _ = fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
	_ = tw.Flush()
}

// sessionRows builds the table's data rows: each artifact's fields in column
// order, an empty session shown as "-" and the URL built from the daemon port.
func sessionRows(port int, sessions []sessionInfo) [][]string {
	rows := make([][]string, 0, len(sessions))
	for _, s := range sessions {
		session := s.SessionID
		if session == "" {
			session = "-"
		}
		rows = append(rows, []string{
			s.Subject,
			s.Slug,
			session,
			s.Status,
			strconv.Itoa(s.EventCount),
			fmt.Sprintf("http://127.0.0.1:%d/p/%s", port, s.Slug),
		})
	}
	return rows
}

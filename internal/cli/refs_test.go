package cli

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/yasyf/cc-present/internal/assets"
	"github.com/yasyf/cc-present/internal/doc"
)

var linkTitle = regexp.MustCompile(`\[([^\]]+)\]\(<([^>]*)> "ref:([0-9a-f]{64})"\)`)

const poolGo = "package pool\n\n// one\n// two\n// three\n// four\n// five\n// six\n// seven\n// eight\n// nine\n// ten\n"

type memStore map[string]assets.Snippet

func (m memStore) put(s assets.Snippet) (string, error) {
	sha, err := localSnippetStore(s)
	m[sha] = s
	return sha, err
}

func noNotes(context.Context, string, string) *assets.Snippet { return nil }

func refRepo(t *testing.T) (string, string) {
	t.Helper()
	dir, short := gitRepo(t, map[string]string{"go/pool/pool.go": poolGo, "docs/plan.md": "intro\nstep one\nstep two\nstep three\n"})
	//nolint:gosec // G204: the test's own git setup over a t.TempDir() repo.
	if out, err := exec.Command("git", "-C", dir, "remote", "add", "origin", "git@github.com:acme/widgets.git").CombinedOutput(); err != nil {
		t.Fatalf("git remote add: %v\n%s", err, out)
	}
	return dir, short
}

func linkMarkdown(t *testing.T, root string, sources map[string]string, md string) (string, []string, memStore) {
	t.Helper()
	b := &doc.Markdown{Md: md}
	b.ID, b.Type = "m", "markdown"
	store := memStore{}
	unresolved, err := linkRefsWith(t.Context(), root, sources, []doc.Block{b}, store.put, noNotes)
	if err != nil {
		t.Fatalf("linkRefs: %v", err)
	}
	return b.Md, unresolved, store
}

func TestLinkRefsCodeCites(t *testing.T) {
	root, short := refRepo(t)
	md := "length (`go/pool/pool.go:4` @ " + short + ", and `:9-10`); also `go/pool/pool.go`."
	out, unresolved, store := linkMarkdown(t, root, nil, md)
	if len(unresolved) != 0 {
		t.Fatalf("unresolved = %v, want none", unresolved)
	}
	links := linkTitle.FindAllStringSubmatch(out, -1)
	if len(links) != 3 {
		t.Fatalf("links = %d in %q, want 3", len(links), out)
	}
	if strings.Contains(out, "@ "+short) {
		t.Fatalf("sha suffix survived: %q", out)
	}
	first, cont, file := links[0], links[1], links[2]
	if first[1] != "pool.go:4" || cont[1] != ":9-10" || file[1] != "pool.go" {
		t.Fatalf("labels = %q %q %q", first[1], cont[1], file[1])
	}
	s := store[first[3]]
	if s.Kind != "code" || s.Path != "go/pool/pool.go" || s.Highlight != "4" || len(s.Sha) != 40 {
		t.Fatalf("snippet = %+v", s)
	}
	if len(s.Chunks) != 1 || s.Chunks[0].Start != 1 || !strings.HasSuffix(s.Chunks[0].Code, "// five") {
		t.Fatalf("chunks = %+v, want lines 1-7", s.Chunks)
	}
	if want := "https://github.com/acme/widgets/blob/" + s.Sha + "/go/pool/pool.go#L4"; first[2] != want {
		t.Fatalf("url = %q, want %q", first[2], want)
	}
	if c := store[cont[3]]; c.Path != "go/pool/pool.go" || c.Sha != s.Sha || c.Highlight != "9-10" {
		t.Fatalf("continuation snippet = %+v", c)
	}
}

func TestLinkRefsLeavesUnresolvedAndProtectedText(t *testing.T) {
	root, _ := refRepo(t)
	md := strings.Join([]string{
		"missing `go/pool/nope.go:3` and past the end `go/pool/pool.go:999`",
		"word `open` and a [link `go/pool/pool.go:4`](https://x.test)",
		"````md",
		"```",
		"`go/pool/pool.go:4`",
		"```",
		"````",
		"[`go/pool/pool.go:4`][docs]",
		"(0deadbe) is not a commit, (plan 2) has no declared source",
	}, "\n")
	out, unresolved, _ := linkMarkdown(t, root, nil, md)
	if out != md {
		t.Fatalf("markdown changed:\n%s\nwant\n%s", out, md)
	}
	want := []string{"`go/pool/nope.go:3`", "`go/pool/pool.go:999`", "0deadbe"}
	if strings.Join(unresolved, "|") != strings.Join(want, "|") {
		t.Fatalf("unresolved = %q, want %q", unresolved, want)
	}
}

func TestLinkRefsCommitAndRecordIDs(t *testing.T) {
	root, _ := refRepo(t)
	head, err := runGit(t.Context(), root, gitRevParse, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	md := "the pick (" + strings.TrimSpace(string(head)) + "; f00dcafe1)"
	b := &doc.Markdown{Md: md}
	b.ID, b.Type = "m", "markdown"
	store := memStore{}
	notes := func(_ context.Context, _, id string) *assets.Snippet {
		if id == "f00dcafe1" {
			return &assets.Snippet{Kind: "record", Label: id, Title: "Owner pick"}
		}
		return nil
	}
	unresolved, err := linkRefsWith(t.Context(), root, nil, []doc.Block{b}, store.put, notes)
	if err != nil || len(unresolved) != 0 {
		t.Fatalf("linkRefs = %v, %v", unresolved, err)
	}
	links := linkTitle.FindAllStringSubmatch(b.Md, -1)
	if len(links) != 2 {
		t.Fatalf("links = %d in %q, want 2", len(links), b.Md)
	}
	c := store[links[0][3]]
	if c.Kind != "commit" || c.Title != "init" || c.Author != "t" || !strings.HasPrefix(links[0][2], "https://github.com/acme/widgets/commit/") {
		t.Fatalf("commit snippet = %+v url %q", c, links[0][2])
	}
	if r := store[links[1][3]]; r.Kind != "record" || r.Title != "Owner pick" || links[1][2] != "" {
		t.Fatalf("record snippet = %+v url %q", r, links[1][2])
	}
}

func TestLinkRefsSourceAliases(t *testing.T) {
	root, short := refRepo(t)
	abs := filepath.Join(t.TempDir(), "plan.md")
	if err := os.WriteFile(abs, []byte("a\nb\nc\nd\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sources := map[string]string{"plan": "docs/plan.md@" + short, "notes": abs}
	out, unresolved, store := linkMarkdown(t, root, sources, "budget (plan 2, 4; notes 3) and (plan 9)")
	if strings.Join(unresolved, "|") != "plan 9" {
		t.Fatalf("unresolved = %q, want [plan 9]", unresolved)
	}
	links := linkTitle.FindAllStringSubmatch(out, -1)
	if len(links) != 3 {
		t.Fatalf("links = %d in %q, want 3", len(links), out)
	}
	if links[0][1] != "plan 2" || links[1][1] != "4" || links[2][1] != "notes 3" {
		t.Fatalf("labels = %q %q %q", links[0][1], links[1][1], links[2][1])
	}
	if s := store[links[0][3]]; s.Path != "docs/plan.md" || s.Highlight != "2" || !strings.Contains(links[0][2], "docs/plan.md#L2") {
		t.Fatalf("plan snippet = %+v url %q", s, links[0][2])
	}
	if s := store[links[2][3]]; s.Path != abs || links[2][2] != "" || s.Chunks[0].Code != "a\nb\nc\nd" {
		t.Fatalf("notes snippet = %+v url %q", s, links[2][2])
	}
}

func TestLinkRefsRewritesEveryMarkdownField(t *testing.T) {
	root, short := refRepo(t)
	cite := "`go/pool/pool.go:4` @ " + short
	raw := `{"version":1,"title":"t","blocks":[{"id":"c","type":"card","summary":"S ` + cite + `","children":[
		{"id":"ch","type":"choice","options":[{"id":"o","label":"L","hint":"H ` + cite + `","md":"M ` + cite + `","detail":{"md":"D ` + cite + `"}}]},
		{"id":"tb","type":"table","columns":[{"key":"k","label":"K"}],"rows":[{"k":"R ` + cite + `"}]}]}]}`
	raw = strings.ReplaceAll(raw, "`", "\\u0060")
	var dd doc.Doc
	if err := json.Unmarshal([]byte(raw), &dd); err != nil {
		t.Fatal(err)
	}
	store := memStore{}
	if _, err := linkRefsWith(t.Context(), root, nil, dd.Blocks, store.put, noNotes); err != nil {
		t.Fatal(err)
	}
	if got := len(doc.RefShas(dd.Blocks[0])); got != 5 {
		out, _ := json.Marshal(dd)
		t.Fatalf("RefShas = %d, want 5 across summary, hint, md, detail, cell: %s", got, out)
	}
}

func TestLinkRefsContinuationFollowsLatestSha(t *testing.T) {
	root, short := refRepo(t)
	out, unresolved, store := linkMarkdown(t, root, nil, "`go/pool/pool.go:4` @ "+short+", `:6` @ "+short+", `:9`")
	if len(unresolved) != 0 {
		t.Fatalf("unresolved = %v", unresolved)
	}
	links := linkTitle.FindAllStringSubmatch(out, -1)
	if len(links) != 3 || store[links[2][3]].Highlight != "9" || store[links[2][3]].Sha != store[links[0][3]].Sha {
		t.Fatalf("links = %q", links)
	}
}

func TestLinkRefsRefusesSecretSourcePaths(t *testing.T) {
	root, _ := refRepo(t)
	dir := filepath.Join(t.TempDir(), ".docker")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "config.json")
	if err := os.WriteFile(cfg, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, unresolved, _ := linkMarkdown(t, root, map[string]string{"cfg": cfg}, "(cfg 1)")
	if out != "(cfg 1)" || strings.Join(unresolved, "|") != "cfg 1" {
		t.Fatalf("out = %q unresolved = %q, want the secret-dir source left unresolved", out, unresolved)
	}
}

func TestBlobURLEscapesPath(t *testing.T) {
	if got, want := blobURL("o/r", "abc", "docs/plan#2.md", "3"), "https://github.com/o/r/blob/abc/docs/plan%232.md#L3"; got != want {
		t.Fatalf("blobURL = %q, want %q", got, want)
	}
}

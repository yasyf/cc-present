package daemon

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const designDocSecret = "TOPSECRET"

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	//nolint:gosec // G204: every argument is a fixture value from this test.
	out, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeDocFile(t *testing.T, repo, name, body string) {
	t.Helper()
	path := filepath.Join(repo, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func commitDocs(t *testing.T, repo, message string) {
	t.Helper()
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-q", "-m", message)
}

func newDesignDocsCheckout(t *testing.T) string {
	t.Helper()
	parent := t.TempDir()
	repo := filepath.Join(parent, "design-docs")
	if err := os.Mkdir(repo, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "secret.txt"), []byte(designDocSecret), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "init", "-q", "-b", "main")
	writeDocFile(t, repo, "design-docs/alpha/index.html", "<h1>alpha on main</h1>")
	writeDocFile(t, repo, "one-pagers/brief/index.html", "<h1>one pager</h1>")
	commitDocs(t, repo, "main docs")
	runGit(t, repo, "switch", "-q", "-c", "unpublished/beta")
	writeDocFile(t, repo, "design-docs/beta/index.html", "<h1>beta on its branch</h1>")
	writeDocFile(t, repo, "design-docs/beta/sysd.svg", "<svg/>")
	if err := os.Symlink("../../../secret.txt", filepath.Join(repo, "design-docs/beta/escape")); err != nil {
		t.Fatal(err)
	}
	commitDocs(t, repo, "beta doc")
	runGit(t, repo, "switch", "-q", "main")
	return repo
}

func designDocServer(t *testing.T, root string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	(&restServer{designDocsRoot: root, static: http.NotFoundHandler()}).routes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func getDesignDoc(t *testing.T, srv *httptest.Server, rawPath string) (int, string, http.Header) {
	t.Helper()
	//nolint:gosec // G107: the host is this test's own httptest server.
	resp, err := http.Get(srv.URL + rawPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(body), resp.Header
}

func designDocQuery(slug, branch string) string {
	query := url.Values{"slug": {slug}}
	if branch != "" {
		query.Set("branch", branch)
	}
	return "/api/design-doc?" + query.Encode()
}

func resolveDesignDocSrc(t *testing.T, srv *httptest.Server, slug, branch string) string {
	t.Helper()
	status, body, _ := getDesignDoc(t, srv, designDocQuery(slug, branch))
	if status != http.StatusOK {
		t.Fatalf("resolve %s@%s = %d: %s", slug, branch, status, body)
	}
	var resolved struct {
		Src string `json:"src"`
	}
	if err := json.Unmarshal([]byte(body), &resolved); err != nil {
		t.Fatal(err)
	}
	return resolved.Src
}

func TestDesignDocReadsAnUnpublishedBranchWithoutCheckingItOut(t *testing.T) {
	repo := newDesignDocsCheckout(t)
	srv := designDocServer(t, repo)

	src := resolveDesignDocSrc(t, srv, "beta", "unpublished/beta")
	status, body, header := getDesignDoc(t, srv, src)
	if status != http.StatusOK || !strings.Contains(body, "beta on its branch") {
		t.Fatalf("doc = %d %q", status, body)
	}
	if ct := header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("doc content type = %q", ct)
	}

	base, err := url.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	sibling, err := base.Parse("sysd.svg")
	if err != nil {
		t.Fatal(err)
	}
	status, body, header = getDesignDoc(t, srv, sibling.Path)
	if status != http.StatusOK || body != "<svg/>" || !strings.HasPrefix(header.Get("Content-Type"), "image/svg") {
		t.Fatalf("image from the same branch = %d %q %q", status, body, header.Get("Content-Type"))
	}
	if head := runGit(t, repo, "branch", "--show-current"); head != "main" {
		t.Fatalf("the read moved the checkout to %q", head)
	}
}

func TestDesignDocDefaultsToMainAndFindsEveryDocGroup(t *testing.T) {
	srv := designDocServer(t, newDesignDocsCheckout(t))
	for slug, want := range map[string]string{"alpha": "alpha on main", "brief": "one pager"} {
		_, body, _ := getDesignDoc(t, srv, resolveDesignDocSrc(t, srv, slug, ""))
		if !strings.Contains(body, want) {
			t.Fatalf("%s = %q, want %q", slug, body, want)
		}
	}
}

func TestDesignDocSaysWhichPieceIsMissing(t *testing.T) {
	srv := designDocServer(t, newDesignDocsCheckout(t))
	tests := []struct {
		slug, branch, want string
	}{
		{"alpha", "no-such-branch", `no branch "no-such-branch"`},
		{"gamma", "", `no design doc "gamma" on branch "main"`},
		{"beta", "main", `no design doc "beta" on branch "main"`},
	}
	for _, tt := range tests {
		status, body, _ := getDesignDoc(t, srv, designDocQuery(tt.slug, tt.branch))
		if status != http.StatusNotFound || !strings.Contains(body, tt.want) {
			t.Errorf("%s@%s = %d %q, want 404 naming %q", tt.slug, tt.branch, status, body, tt.want)
		}
	}
}

func TestDesignDocWithoutACheckoutNamesTheSetting(t *testing.T) {
	status, body, _ := getDesignDoc(t, designDocServer(t, ""), designDocQuery("alpha", ""))
	if status != http.StatusNotFound || !strings.Contains(body, "DESIGN_DOCS_ROOT") {
		t.Fatalf("no checkout = %d %q", status, body)
	}
}

func TestDesignDocFollowsABranchPushedFromAnotherMachine(t *testing.T) {
	origin := newDesignDocsCheckout(t)
	clone := filepath.Join(t.TempDir(), "design-docs")
	runGit(t, filepath.Dir(clone), "clone", "-q", origin, clone)
	srv := designDocServer(t, clone)
	if got := resolveDesignDocSrc(t, srv, "beta", "unpublished/beta"); !strings.Contains(got, runGit(t, origin, "rev-parse", "unpublished/beta")) {
		t.Fatalf("first read %q is not the pushed head", got)
	}

	runGit(t, origin, "switch", "-q", "-c", "pushed-later")
	writeDocFile(t, origin, "design-docs/delta/index.html", "<h1>delta</h1>")
	commitDocs(t, origin, "delta doc")
	runGit(t, origin, "switch", "-q", "main")

	_, body, _ := getDesignDoc(t, srv, resolveDesignDocSrc(t, srv, "delta", "pushed-later"))
	if !strings.Contains(body, "delta") {
		t.Fatalf("branch pushed after the clone = %q", body)
	}
}

func TestDesignDocPrefersTheNewerOfLocalAndOrigin(t *testing.T) {
	origin := newDesignDocsCheckout(t)
	clone := filepath.Join(t.TempDir(), "design-docs")
	runGit(t, filepath.Dir(clone), "clone", "-q", origin, clone)
	runGit(t, clone, "switch", "-q", "-c", "unpublished/beta", "--track", "origin/unpublished/beta")
	writeDocFile(t, clone, "design-docs/beta/index.html", "<h1>not pushed yet</h1>")
	runGit(t, clone, "add", "-A")
	//nolint:gosec // G204: clone is this test's own t.TempDir() path.
	cmd := exec.Command("git", "-C", clone, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "local edit")
	cmd.Env = append(os.Environ(), "GIT_COMMITTER_DATE=2090-01-01T00:00:00Z")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("commit: %v\n%s", err, out)
	}

	srv := designDocServer(t, clone)
	_, body, _ := getDesignDoc(t, srv, resolveDesignDocSrc(t, srv, "beta", "unpublished/beta"))
	if !strings.Contains(body, "not pushed yet") {
		t.Fatalf("doc = %q, want the unpushed local commit", body)
	}
}

func TestDesignDocRefusesAResolveThatLeavesTheCheckout(t *testing.T) {
	repo := newDesignDocsCheckout(t)
	srv := designDocServer(t, repo)
	marker := filepath.Join(t.TempDir(), "injected")
	tests := []struct {
		name, slug, branch string
	}{
		{"parent slug", "..", ""},
		{"traversing slug", "../secret.txt", ""},
		{"absolute slug", "/etc/passwd", ""},
		{"nested slug", "beta/../../secret.txt", ""},
		{"dotted segment slug", "beta/..", ""},
		{"empty slug", "", ""},
		{"option slug", "--output=" + marker, ""},
		{"traversing branch", "alpha", "../../secret.txt"},
		{"dotdot branch", "alpha", "main..unpublished/beta"},
		{"option branch", "alpha", "--upload-pack=touch " + marker},
		{"revision branch", "alpha", "main^{tree}"},
		{"ancestor branch", "alpha", "main~1"},
		{"tree path branch", "alpha", "main:design-docs"},
		{"glob branch", "alpha", "*"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body, _ := getDesignDoc(t, srv, designDocQuery(tt.slug, tt.branch))
			if status != http.StatusBadRequest {
				t.Fatalf("status = %d %q, want 400", status, body)
			}
			if strings.Contains(body, "/design-docs/") {
				t.Fatalf("refusal handed back a doc path: %q", body)
			}
		})
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("an option reached git: %v", err)
	}
}

func TestDesignDocRefusesAFileThatLeavesTheCheckout(t *testing.T) {
	repo := newDesignDocsCheckout(t)
	srv := designDocServer(t, repo)
	commit := runGit(t, repo, "rev-parse", "unpublished/beta")
	prefix := "/design-docs/" + commit + "/"
	tests := []struct {
		name, path string
	}{
		{"encoded parent", prefix + "%2e%2e/secret.txt"},
		{"encoded slash parent", prefix + "..%2f..%2fsecret.txt"},
		{"parent inside the doc", prefix + "design-docs/beta/%2e%2e/%2e%2e/%2e%2e/secret.txt"},
		{"absolute file", prefix + "%2fetc%2fpasswd"},
		{"git directory", prefix + ".git/config"},
		{"nul byte", prefix + "design-docs/beta/index.html%00.txt"},
		{"branch for commit", "/design-docs/main/design-docs/alpha/index.html"},
		{"short commit", "/design-docs/" + commit[:12] + "/design-docs/beta/index.html"},
		{"revision for commit", "/design-docs/" + commit + "~1/design-docs/alpha/index.html"},
		{"option for commit", "/design-docs/--output=x/design-docs/alpha/index.html"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body, _ := getDesignDoc(t, srv, tt.path)
			if status == http.StatusOK {
				t.Fatalf("status = 200 %q, want a refusal", body)
			}
			if strings.Contains(body, designDocSecret) {
				t.Fatal("the refusal leaked the secret beside the checkout")
			}
		})
	}
}

func TestDesignDocSymlinkNeverFollowsOutOfTheCheckout(t *testing.T) {
	repo := newDesignDocsCheckout(t)
	srv := designDocServer(t, repo)
	commit := runGit(t, repo, "rev-parse", "unpublished/beta")
	_, body, _ := getDesignDoc(t, srv, "/design-docs/"+commit+"/design-docs/beta/escape")
	if strings.Contains(body, designDocSecret) {
		t.Fatal("a symlink in the doc read the secret beside the checkout")
	}
}

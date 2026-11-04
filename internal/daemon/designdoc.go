package daemon

import (
	"cmp"
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"time"
)

const designDocFetchTimeout = 5 * time.Second

var (
	designDocSlug   = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_-]*(?:\.[A-Za-z0-9_-]+)*$`)
	designDocBranch = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_-]*(?:[./][A-Za-z0-9_-]+)*$`)
	designDocCommit = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

func (rs *restServer) handleDesignDoc(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	slug, branch := query.Get("slug"), cmp.Or(query.Get("branch"), "main")
	if !designDocSlug.MatchString(slug) || !designDocBranch.MatchString(branch) {
		http.Error(w, fmt.Sprintf("not a design doc slug and branch: %q, %q", slug, branch), http.StatusBadRequest)
		return
	}
	if rs.designDocsRoot == "" {
		http.Error(w, "no design-docs checkout: set DESIGN_DOCS_ROOT for the cc-present daemon", http.StatusNotFound)
		return
	}
	rs.fetchDesignDocBranch(r.Context(), branch)
	commit, err := rs.newestDesignDocCommit(r.Context(), branch)
	if err != nil {
		slog.Error("read design-docs refs", "branch", branch, "err", err)
		http.Error(w, "cannot read the design-docs checkout", http.StatusInternalServerError)
		return
	}
	if commit == "" {
		http.Error(w, fmt.Sprintf("no branch %q in the design-docs checkout", branch), http.StatusNotFound)
		return
	}
	dir, err := rs.designDocDir(r.Context(), commit, slug)
	if err != nil {
		slog.Error("read design-docs tree", "commit", commit, "err", err)
		http.Error(w, "cannot read the design-docs checkout", http.StatusInternalServerError)
		return
	}
	if dir == "" {
		http.Error(w, fmt.Sprintf("no design doc %q on branch %q", slug, branch), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"src": "/design-docs/" + commit + "/" + dir + "/index.html"})
}

func (rs *restServer) handleDesignDocFile(w http.ResponseWriter, r *http.Request) {
	commit, file := r.PathValue("commit"), r.PathValue("file")
	if rs.designDocsRoot == "" || !designDocCommit.MatchString(commit) || !fs.ValidPath(file) {
		http.NotFound(w, r)
		return
	}
	blob, err := rs.designDocGit(r.Context(), "cat-file", "blob", commit+":"+file).Output()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	if ct := packContentType(file); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	//nolint:gosec // G705: serving this committed HTML is the route's purpose; its commit and path passed their guards, and only the board's own auth reaches it.
	if _, err := w.Write(blob); err != nil {
		slog.Debug("write design doc file", "file", file, "err", err)
	}
}

func (rs *restServer) fetchDesignDocBranch(ctx context.Context, branch string) {
	ctx, cancel := context.WithTimeout(ctx, designDocFetchTimeout)
	defer cancel()
	fetch := rs.designDocGit(ctx, "fetch", "--quiet", "origin", "+refs/heads/"+branch+":refs/remotes/origin/"+branch)
	fetch.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if err := fetch.Run(); err != nil {
		slog.Debug("fetch design-docs branch", "branch", branch, "err", err)
	}
}

func (rs *restServer) newestDesignDocCommit(ctx context.Context, branch string) (string, error) {
	local, remote := "refs/heads/"+branch, "refs/remotes/origin/"+branch
	out, err := rs.designDocGit(ctx, "for-each-ref", "--sort=-committerdate", "--format=%(objectname) %(refname)", local, remote).Output()
	if err != nil {
		return "", fmt.Errorf("for-each-ref %s: %w", branch, err)
	}
	for line := range strings.Lines(string(out)) {
		commit, ref, _ := strings.Cut(strings.TrimSpace(line), " ")
		if ref == local || ref == remote {
			return commit, nil
		}
	}
	return "", nil
}

func (rs *restServer) designDocDir(ctx context.Context, commit, slug string) (string, error) {
	dirs := []string{"design-docs/" + slug, "one-pagers/" + slug, "incident-retros/" + slug, slug}
	args := append(make([]string, 0, 4+len(dirs)), "ls-tree", "--name-only", commit, "--")
	for _, dir := range dirs {
		args = append(args, dir+"/index.html")
	}
	out, err := rs.designDocGit(ctx, args...).Output()
	if err != nil {
		return "", fmt.Errorf("ls-tree %s %s: %w", commit, slug, err)
	}
	found := strings.Fields(string(out))
	for _, dir := range dirs {
		if slices.Contains(found, dir+"/index.html") {
			return dir, nil
		}
	}
	return "", nil
}

func (rs *restServer) designDocGit(ctx context.Context, args ...string) *exec.Cmd {
	//nolint:gosec // G204: slug, branch and commit pass their patterns first, so none can begin an option or name a revision expression.
	return exec.CommandContext(ctx, "git", append([]string{"-C", rs.designDocsRoot}, args...)...)
}

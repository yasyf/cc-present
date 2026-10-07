package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/yasyf/cc-present/internal/doc"
)

const (
	maxCodeSrcBytes  = 1 << 20
	longCodeSliceMin = 40
)

type secretPattern struct {
	class string
	re    *regexp.Regexp
}

var secretNames = []secretPattern{
	{"credential directory", regexp.MustCompile(`(?i)(^|/)\.(git|ssh|aws|azure|gnupg|kube|docker|password-store)(/|$)`)},
	{"dotfile credential", regexp.MustCompile(`(?i)(^|/)(\.env(\.[^/]*)?|\.netrc|\.npmrc|\.yarnrc(\.yml)?|\.pypirc|\.pgpass|\.my\.cnf|\.git-credentials|\.htpasswd)$`)},
	{"key or credentials file", regexp.MustCompile(`(?i)(^|/)(id_(rsa|dsa|ecdsa|ed25519)[^/]*|credentials[^/]*|secrets?(\.[^/]*)?|[^/]*_history|[^/]*\.local\.json)$`)},
	{"key, state, or database file", regexp.MustCompile(`(?i)\.(pem|key|p12|pfx|keystore|jks|tfvars|tfstate(\.backup)?|sqlite3?|db|kdbx|ovpn)$`)},
}

var secretTexts = []secretPattern{
	{"Anthropic API key", regexp.MustCompile(`(?i)sk-ant-`)},
	{"sk- API key", regexp.MustCompile(`(?i)sk-[A-Za-z0-9]{32,}`)},
	{"AWS access key id", regexp.MustCompile(`(?i)AKIA[0-9A-Z]{16}`)},
	{"private key block", regexp.MustCompile(`(?i)-----BEGIN [A-Z ]*PRIVATE KEY`)},
	{"GitHub token", regexp.MustCompile(`(?i)gh[pousr]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{30,}`)},
	{"Slack token", regexp.MustCompile(`(?i)xox[abeprs]-[A-Za-z0-9-]{10,}`)},
	{"Google API key", regexp.MustCompile(`(?i)AIza[0-9A-Za-z_-]{30,}`)},
	{"JWT", regexp.MustCompile(`(?i)eyJ[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{10,}\.`)},
	{"password or token assignment", regexp.MustCompile(`(?i)(password|passwd|secret|api[_-]?key|access[_-]?token|auth[_-]?token)["']?\s*[:=]\s*["'][^"'\s$<{]{12,}["']`)},
}

type gitPlumbing string

const (
	gitRevParse gitPlumbing = "rev-parse"
	gitCatFile  gitPlumbing = "cat-file"
)

func matchSecret(patterns []secretPattern, s string) string {
	for _, p := range patterns {
		if p.re.MatchString(s) {
			return p.class
		}
	}
	return ""
}

func runGit(ctx context.Context, dir string, sub gitPlumbing, args ...string) ([]byte, error) {
	argv := append([]string{"-c", "core.fsmonitor=false", "-c", "core.hooksPath=/dev/null", "-C", dir, string(sub)}, args...)
	//nolint:gosec // G204: the subcommand is a rev-parse/cat-file constant; args never reach a shell.
	cmd := exec.CommandContext(ctx, "git", argv...)
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "GIT_PAGER=cat")
	return cmd.Output()
}

func codeRoot(ctx context.Context, flag string) (string, error) {
	dir := flag
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		dir = cwd
		if top, err := runGit(ctx, cwd, gitRevParse, "--show-toplevel"); err == nil {
			dir = strings.TrimSpace(string(top))
		}
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	root, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("code root %q: %w", dir, err)
	}
	return root, nil
}

func readCodeSrc(root, src string) (string, []byte, error) {
	if class := matchSecret(secretNames, src); class != "" {
		return "", nil, fmt.Errorf("src %q looks like a secret file (%s); not reading it", src, class)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(src)))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil, fmt.Errorf("src %q not found under %s; pass --root for the checkout it lives in", src, root)
	}
	if err != nil {
		return "", nil, fmt.Errorf("src %q: %w", src, err)
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		return "", nil, fmt.Errorf("src %q: %w", src, err)
	}
	if !filepath.IsLocal(rel) {
		return "", nil, fmt.Errorf("src %q resolves outside %s; pass --root for the checkout it lives in", src, root)
	}
	rel = filepath.ToSlash(rel)
	if class := matchSecret(secretNames, rel); class != "" {
		return "", nil, fmt.Errorf("src %q resolves to %s, which looks like a secret file (%s); not reading it", src, rel, class)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", nil, fmt.Errorf("src %q: %w", src, err)
	}
	if !info.Mode().IsRegular() {
		return "", nil, fmt.Errorf("src %q is not a regular file", src)
	}
	if info.Size() > maxCodeSrcBytes {
		return "", nil, fmt.Errorf("src %q is %d bytes, exceeds %d", src, info.Size(), maxCodeSrcBytes)
	}
	//nolint:gosec // G304: resolved is fenced under root and past the secret-name check above.
	data, err := os.ReadFile(resolved)
	if err != nil {
		return "", nil, fmt.Errorf("read src %q: %w", src, err)
	}
	if !utf8.Valid(data) {
		return "", nil, fmt.Errorf("src %q is not UTF-8 text", src)
	}
	if class := matchSecret(secretTexts, string(data)); class != "" {
		return "", nil, fmt.Errorf("src %q holds a secret (%s); not reading any of it", src, class)
	}
	return rel, data, nil
}

func stampSha(ctx context.Context, root, rel string, data []byte) string {
	head, err := runGit(ctx, root, gitRevParse, "--short", "HEAD")
	if err != nil {
		return ""
	}
	sha := strings.TrimSpace(string(head))
	committed, err := runGit(ctx, root, gitCatFile, "blob", "HEAD:./"+rel)
	if err != nil || !bytes.Equal(committed, data) {
		return sha + "+wt"
	}
	return sha
}

func fillCodeBlock(ctx context.Context, root string, c *doc.Code) error {
	rel, data, err := readCodeSrc(root, c.Src)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	first, last := 1, len(lines)
	if c.Lines != "" {
		if first, last, err = doc.ParseLineRange(c.Lines); err != nil {
			return err
		}
		if last > len(lines) {
			return fmt.Errorf("lines %q but %s has %d lines; is --root at the commit you are citing?", c.Lines, c.Src, len(lines))
		}
	}
	c.Code = strings.Join(lines[first-1:last], "\n")
	c.Start = first
	if c.Lang == "" {
		c.Lang = langFromExt(rel)
	}
	c.Sha = stampSha(ctx, root, rel, data)
	return nil
}

func langFromExt(path string) string {
	if ext := strings.TrimPrefix(filepath.Ext(path), "."); ext != "" {
		return strings.ToLower(ext)
	}
	return "text"
}

func fillCode(ctx context.Context, blocks []doc.Block, rootFlag string) error {
	var grounded []*doc.Code
	for _, b := range blocks {
		doc.Walk(b, func(b doc.Block) {
			if c, ok := b.(*doc.Code); ok && c.Src != "" {
				grounded = append(grounded, c)
			}
		})
	}
	if len(grounded) == 0 {
		return nil
	}
	root, err := codeRoot(ctx, rootFlag)
	if err != nil {
		return err
	}
	var errs []error
	for _, c := range grounded {
		if err := fillCodeBlock(ctx, root, c); err != nil {
			errs = append(errs, fmt.Errorf("code %q: %w", c.ID, err))
		}
	}
	return errors.Join(errs...)
}

func longCodeNudge(blocks []doc.Block) string {
	var long []string
	for _, b := range blocks {
		doc.Walk(b, func(b doc.Block) {
			if c, ok := b.(*doc.Code); ok && c.Src != "" && strings.Count(c.Code, "\n") >= longCodeSliceMin {
				long = append(long, c.ID)
			}
		})
	}
	if len(long) == 0 {
		return ""
	}
	return fmt.Sprintf("hint: code %s shows more than %d lines; slice src with lines to the part that carries the point",
		strings.Join(long, ", "), longCodeSliceMin)
}

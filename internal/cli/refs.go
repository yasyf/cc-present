package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/sync/errgroup"

	"github.com/yasyf/cc-present/internal/assets"
	"github.com/yasyf/cc-present/internal/doc"
)

const (
	refContext       = 3
	refMaxLines      = 80
	refFileHeadLines = 30
	refBodyMax       = 1500
	refLookupWorkers = 8
)

var (
	linkPattern      = regexp.MustCompile(`\[[^\]\n]*\](?:\([^)\n]*\)|\[[^\]\n]*\])`)
	codeSpanPattern  = regexp.MustCompile("`([^`\n]+)`")
	citePattern      = regexp.MustCompile(`^([\w.@+-]+(?:/[\w.@+-]+)*)?:(\d+(?:-\d+)?(?:,\d+(?:-\d+)?)*)$`)
	filePathPattern  = regexp.MustCompile(`^[\w.@+-]+(?:/[\w.@+-]+)+\.\w+$`)
	shaSuffixPattern = regexp.MustCompile(`^ ?@ ?([0-9a-f]{7,40})\b`)
	parenPattern     = regexp.MustCompile(`\([^()\n]*\)`)
	hexIDPattern     = regexp.MustCompile(`\b[0-9a-f]{7,40}\b`)
	aliasRefPattern  = regexp.MustCompile(`\b([a-z][a-z0-9-]*) (\d+(?:, ?\d+)*)\b`)
	aliasNumPattern  = regexp.MustCompile(`\d+`)
	githubRemote     = regexp.MustCompile(`github\.com[:/]([\w.-]+/[\w.-]+?)(?:\.git)?/?$`)
)

type snippetStore func(assets.Snippet) (string, error)

func httpSnippetStore(port int) snippetStore {
	return func(s assets.Snippet) (string, error) {
		b, err := json.Marshal(s)
		if err != nil {
			return "", err
		}
		sha, err := postAsset(port, b, assets.SnippetType)
		if err != nil {
			return "", fmt.Errorf("store snippet %q: %w", s.Label, err)
		}
		return sha, nil
	}
}

func localSnippetStore(s assets.Snippet) (string, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return assets.SHA(b), nil
}

type sourceFile struct {
	path  string
	sha   string
	lines []string
}

type refLinker struct {
	root       string
	repo       string
	sources    map[string]string
	store      snippetStore
	showNote   func(ctx context.Context, root, id string) *assets.Snippet
	commits    map[string]string
	blobs      map[string][]string
	files      map[string]*sourceFile
	records    map[string]*assets.Snippet
	stored     map[string]string
	unresolved []string
}

type cite struct {
	path, sha string
}

type spanMatch struct {
	start, end int
	path, spec string
	sha        string
	cite       bool
}

type tokenMatch struct {
	start, end int
	id         string
	alias      string
	nums       [][2]int
}

// linkRefs rewrites each resolvable code cite, record id, and source-alias line
// in blocks' markdown into a link to its stored hover snippet, and returns the
// references left as plain text.
func linkRefs(ctx context.Context, rootFlag string, sources map[string]string, blocks []doc.Block, store snippetStore) ([]string, error) {
	return linkRefsWith(ctx, rootFlag, sources, blocks, store, ccNotesRecord)
}

func linkRefsWith(
	ctx context.Context, rootFlag string, sources map[string]string, blocks []doc.Block, store snippetStore,
	showNote func(ctx context.Context, root, id string) *assets.Snippet,
) ([]string, error) {
	root, err := codeRoot(ctx, rootFlag)
	if err != nil {
		return nil, err
	}
	l := &refLinker{
		root:     root,
		repo:     githubRepo(ctx, root),
		sources:  sources,
		store:    store,
		showNote: showNote,
		commits:  map[string]string{},
		blobs:    map[string][]string{},
		files:    map[string]*sourceFile{},
		records:  map[string]*assets.Snippet{},
		stored:   map[string]string{},
	}
	if err := l.prefetchRecords(ctx, blocks); err != nil {
		return nil, err
	}
	for _, b := range blocks {
		if err := doc.RewriteMarkdown(b, func(md string) (string, error) { return l.rewrite(ctx, md) }); err != nil {
			return nil, err
		}
	}
	return dedupe(l.unresolved), nil
}

func unresolvedRefsHint(refs []string) string {
	if len(refs) == 0 {
		return ""
	}
	return fmt.Sprintf("hint: %d reference(s) did not resolve and stay as plain text: %s",
		len(refs), strings.Join(refs, ", "))
}

func dedupe(xs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

func githubRepo(ctx context.Context, root string) string {
	out, err := runGit(ctx, root, gitConfig, "--get", "remote.origin.url")
	if err != nil {
		return ""
	}
	if m := githubRemote.FindStringSubmatch(strings.TrimSpace(string(out))); m != nil {
		return m[1]
	}
	return ""
}

func (l *refLinker) prefetchRecords(ctx context.Context, blocks []doc.Block) error {
	var ids []string
	for _, b := range blocks {
		doc.VisitMarkdown(b, func(md string) {
			proseLines(md, func(_ int, line string) {
				_, tokens := scanLine(line)
				for _, t := range tokens {
					if t.id != "" {
						ids = append(ids, t.id)
					}
				}
			})
		})
	}
	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(refLookupWorkers)
	for _, id := range dedupe(ids) {
		g.Go(func() error {
			s := l.showNote(gctx, l.root, id)
			mu.Lock()
			defer mu.Unlock()
			l.records[id] = s
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	for id, s := range l.records {
		if s == nil {
			l.records[id] = l.commitRecord(ctx, id)
		}
	}
	return nil
}

var fencePattern = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})")

// proseLines calls fn with the index and text of each line of md outside fenced
// code; a fence closes only on a run of its own character at least as long.
func proseLines(md string, fn func(int, string)) {
	open := ""
	for i, line := range strings.Split(md, "\n") {
		if m := fencePattern.FindStringSubmatch(line); m != nil {
			switch {
			case open == "":
				open = m[1]
			case m[1][0] == open[0] && len(m[1]) >= len(open) && strings.TrimSpace(line[len(m[0]):]) == "":
				open = ""
			}
			continue
		}
		if open == "" {
			fn(i, line)
		}
	}
}

func (l *refLinker) rewrite(ctx context.Context, md string) (string, error) {
	lines := strings.Split(md, "\n")
	var prev *cite
	var err error
	proseLines(md, func(i int, line string) {
		if err == nil {
			lines[i], err = l.rewriteLine(ctx, line, &prev)
		}
	})
	if err != nil {
		return "", err
	}
	return strings.Join(lines, "\n"), nil
}

type edit struct {
	start, end int
	text       string
}

func (l *refLinker) rewriteLine(ctx context.Context, line string, prev **cite) (string, error) {
	spans, tokens := scanLine(line)
	var edits []edit
	for i := range spans {
		sp := &spans[i]
		if !sp.cite {
			continue
		}
		continuation := sp.path == ""
		switch {
		case continuation && *prev == nil:
			l.unresolved = append(l.unresolved, line[sp.start:sp.end])
			continue
		case continuation:
			sp.path = (*prev).path
			if sp.sha == "" {
				sp.sha = (*prev).sha
			}
		}
		*prev = &cite{sp.path, sp.sha}
		link, ok, err := l.codeLink(ctx, sp, continuation)
		if err != nil {
			return "", err
		}
		if !ok {
			l.unresolved = append(l.unresolved, line[sp.start:sp.end])
			continue
		}
		edits = append(edits, edit{sp.start, sp.end, link})
	}
	sort.SliceStable(tokens, func(i, j int) bool { return tokens[i].alias != "" && tokens[j].alias == "" })
	for _, t := range tokens {
		if t.id != "" {
			s := l.records[t.id]
			if s == nil {
				l.unresolved = append(l.unresolved, t.id)
				continue
			}
			link, err := l.link(*s)
			if err != nil {
				return "", err
			}
			edits = append(edits, edit{t.start, t.end, link})
			continue
		}
		if _, declared := l.sources[t.alias]; !declared {
			continue
		}
		for i, n := range t.nums {
			num, _ := strconv.Atoi(line[n[0]:n[1]])
			label := line[n[0]:n[1]]
			if i == 0 {
				label = t.alias + " " + label
			}
			s, ok := l.sourceSnippet(ctx, t.alias, num, label)
			if !ok {
				l.unresolved = append(l.unresolved, t.alias+" "+line[n[0]:n[1]])
				continue
			}
			link, err := l.link(s)
			if err != nil {
				return "", err
			}
			start := n[0]
			if i == 0 {
				start = t.start
			}
			edits = append(edits, edit{start, n[1], link})
		}
	}
	var kept []edit
	for _, e := range edits {
		if !slices.ContainsFunc(kept, func(k edit) bool { return e.start < k.end && k.start < e.end }) {
			kept = append(kept, e)
		}
	}
	sort.Slice(kept, func(i, j int) bool { return kept[i].start > kept[j].start })
	for _, e := range kept {
		line = line[:e.start] + e.text + line[e.end:]
	}
	return line, nil
}

// scanLine finds code spans outside links, and record ids and alias references
// inside parentheses but outside spans, links, and cite sha suffixes.
func scanLine(line string) ([]spanMatch, []tokenMatch) {
	masked := []byte(line)
	mask := func(a, b int) {
		for i := a; i < b; i++ {
			masked[i] = 0
		}
	}
	links := linkPattern.FindAllStringIndex(line, -1)
	inLink := func(i int) bool {
		for _, r := range links {
			if i >= r[0] && i < r[1] {
				return true
			}
		}
		return false
	}
	for _, r := range links {
		mask(r[0], r[1])
	}
	var spans []spanMatch
	for _, m := range codeSpanPattern.FindAllStringSubmatchIndex(line, -1) {
		if inLink(m[0]) {
			continue
		}
		sp := spanMatch{start: m[0], end: m[1]}
		text := line[m[2]:m[3]]
		if c := citePattern.FindStringSubmatch(text); c != nil && (c[1] == "" || strings.ContainsAny(c[1], "/.")) {
			sp.cite, sp.path, sp.spec = true, c[1], c[2]
		} else if filePathPattern.MatchString(text) {
			sp.cite, sp.path = true, text
		}
		if sp.cite {
			if s := shaSuffixPattern.FindStringSubmatch(line[sp.end:]); s != nil {
				sp.sha = s[1]
				sp.end += len(s[0])
			}
		}
		mask(sp.start, sp.end)
		spans = append(spans, sp)
	}
	var tokens []tokenMatch
	m := string(masked)
	for _, p := range parenPattern.FindAllStringIndex(m, -1) {
		inner := m[p[0]:p[1]]
		for _, h := range hexIDPattern.FindAllStringIndex(inner, -1) {
			id := inner[h[0]:h[1]]
			if strings.ContainsAny(id, "abcdef") && strings.ContainsAny(id, "0123456789") {
				tokens = append(tokens, tokenMatch{start: p[0] + h[0], end: p[0] + h[1], id: id})
			}
		}
		for _, a := range aliasRefPattern.FindAllStringSubmatchIndex(inner, -1) {
			t := tokenMatch{start: p[0] + a[0], end: p[0] + a[1], alias: inner[a[2]:a[3]]}
			for _, n := range aliasNumPattern.FindAllStringIndex(inner[a[4]:a[5]], -1) {
				t.nums = append(t.nums, [2]int{p[0] + a[4] + n[0], p[0] + a[4] + n[1]})
			}
			tokens = append(tokens, t)
		}
	}
	return spans, tokens
}

func (l *refLinker) link(s assets.Snippet) (string, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	sha, ok := l.stored[string(b)]
	if !ok {
		if sha, err = l.store(s); err != nil {
			return "", err
		}
		l.stored[string(b)] = sha
	}
	return fmt.Sprintf("[%s](<%s> %q)", s.Label, s.URL, doc.RefTitle(sha)), nil
}

func (l *refLinker) commit(ctx context.Context, rev string) (string, bool) {
	if full, ok := l.commits[rev]; ok {
		return full, full != ""
	}
	out, err := runGit(ctx, l.root, gitRevParse, "--verify", "--quiet", rev+"^{commit}")
	full := ""
	if err == nil {
		full = strings.TrimSpace(string(out))
	}
	l.commits[rev] = full
	return full, full != ""
}

func (l *refLinker) blob(ctx context.Context, sha, rel string) ([]string, bool) {
	key := sha + ":" + rel
	if lines, ok := l.blobs[key]; ok {
		return lines, lines != nil
	}
	var lines []string
	if matchSecret(secretNames, rel) == "" {
		out, err := runGit(ctx, l.root, gitCatFile, "blob", key)
		if err == nil && len(out) <= maxCodeSrcBytes && utf8.Valid(out) {
			lines = strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
		}
	}
	l.blobs[key] = lines
	return lines, lines != nil
}

func (l *refLinker) codeLink(ctx context.Context, sp *spanMatch, continuation bool) (string, bool, error) {
	rev := sp.sha
	if rev == "" {
		rev = "HEAD"
	}
	full, ok := l.commit(ctx, rev)
	if !ok {
		return "", false, nil
	}
	lines, ok := l.blob(ctx, full, sp.path)
	if !ok {
		return "", false, nil
	}
	label := path.Base(sp.path)
	if sp.spec != "" {
		label += ":" + sp.spec
	}
	if continuation {
		label = ":" + sp.spec
	}
	s, ok := l.excerpt(lines, sp.spec, assets.Snippet{
		Kind: "code", Label: label, Title: sp.path, Path: sp.path, Sha: full, Lang: langFromExt(sp.path),
	})
	if !ok {
		return "", false, nil
	}
	if l.repo != "" {
		s.URL = blobURL(l.repo, full, sp.path, sp.spec)
	}
	link, err := l.link(s)
	return link, err == nil, err
}

// excerpt fills s with the cited ranges plus context, merged and capped; an
// empty spec takes the file head. It fails past the file's end or on a secret.
func (l *refLinker) excerpt(lines []string, spec string, s assets.Snippet) (assets.Snippet, bool) {
	var spans [][2]int
	if spec == "" {
		spans = [][2]int{{1, min(len(lines), refFileHeadLines)}}
	} else {
		for r := range strings.SplitSeq(spec, ",") {
			a, b, err := doc.ParseLineRange(r)
			if err != nil || b > len(lines) {
				return s, false
			}
			spans = append(spans, [2]int{max(1, a-refContext), min(len(lines), b+refContext)})
		}
		sort.Slice(spans, func(i, j int) bool { return spans[i][0] < spans[j][0] })
		s.Highlight = spec
	}
	var merged [][2]int
	for _, sp := range spans {
		if n := len(merged); n > 0 && sp[0] <= merged[n-1][1]+1 {
			merged[n-1][1] = max(merged[n-1][1], sp[1])
			continue
		}
		merged = append(merged, sp)
	}
	budget := refMaxLines
	for _, sp := range merged {
		if budget <= 0 {
			break
		}
		end := min(sp[1], sp[0]+budget-1)
		code := strings.Join(lines[sp[0]-1:end], "\n")
		if secretInText(code) != "" {
			return s, false
		}
		s.Chunks = append(s.Chunks, assets.Chunk{Start: sp[0], Code: code})
		budget -= end - sp[0] + 1
	}
	return s, true
}

func blobURL(repo, sha, rel, spec string) string {
	segs := strings.Split(rel, "/")
	for i, seg := range segs {
		segs[i] = url.PathEscape(seg)
	}
	u := fmt.Sprintf("https://github.com/%s/blob/%s/%s", repo, sha, strings.Join(segs, "/"))
	if spec == "" {
		return u
	}
	first, _, _ := strings.Cut(spec, ",")
	a, b, err := doc.ParseLineRange(first)
	if err != nil {
		return u
	}
	if a == b {
		return fmt.Sprintf("%s#L%d", u, a)
	}
	return fmt.Sprintf("%s#L%d-L%d", u, a, b)
}

func (l *refLinker) source(ctx context.Context, alias string) (*sourceFile, bool) {
	if f, ok := l.files[alias]; ok {
		return f, f != nil
	}
	f := l.loadSource(ctx, l.sources[alias])
	l.files[alias] = f
	return f, f != nil
}

func (l *refLinker) loadSource(ctx context.Context, spec string) *sourceFile {
	if spec == "" {
		return nil
	}
	if filepath.IsAbs(spec) {
		resolved, err := filepath.EvalSymlinks(spec)
		if err != nil || matchSecret(secretNames, filepath.ToSlash(spec)) != "" || matchSecret(secretNames, filepath.ToSlash(resolved)) != "" {
			return nil
		}
		dir := filepath.Dir(resolved)
		_, data, err := readCodeSrc(dir, filepath.Base(resolved))
		if err != nil {
			return nil
		}
		return &sourceFile{path: spec, lines: strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")}
	}
	rel, rev, found := strings.Cut(spec, "@")
	if !found {
		rev = "HEAD"
	}
	full, ok := l.commit(ctx, rev)
	if !ok {
		return nil
	}
	lines, ok := l.blob(ctx, full, rel)
	if !ok {
		return nil
	}
	return &sourceFile{path: rel, sha: full, lines: lines}
}

func (l *refLinker) sourceSnippet(ctx context.Context, alias string, line int, label string) (assets.Snippet, bool) {
	f, ok := l.source(ctx, alias)
	if !ok {
		return assets.Snippet{}, false
	}
	spec := strconv.Itoa(line)
	s, ok := l.excerpt(f.lines, spec, assets.Snippet{
		Kind: "code", Label: label, Title: f.path, Path: f.path, Sha: f.sha, Lang: langFromExt(f.path),
	})
	if ok && f.sha != "" && l.repo != "" {
		s.URL = blobURL(l.repo, f.sha, f.path, spec)
	}
	return s, ok
}

func ccNotesRecord(ctx context.Context, root, id string) *assets.Snippet {
	//nolint:gosec // G204: id matched [0-9a-f]{7,40}; args never reach a shell.
	out, err := exec.CommandContext(ctx, "cc-notes", "show", id, "--json", "-R", root).Output()
	if err != nil {
		return nil
	}
	var rec struct {
		Title     string   `json:"title"`
		Body      string   `json:"body"`
		Tags      []string `json:"tags"`
		CreatedAt string   `json:"created_at"`
	}
	if json.Unmarshal(out, &rec) != nil || rec.Title == "" || secretInText(rec.Title+"\n"+rec.Body) != "" {
		return nil
	}
	date, _, _ := strings.Cut(rec.CreatedAt, "T")
	return &assets.Snippet{
		Kind: "record", Label: id, Title: rec.Title, Body: truncateRunes(rec.Body, refBodyMax), Tags: rec.Tags, Date: date,
	}
}

func (l *refLinker) commitRecord(ctx context.Context, id string) *assets.Snippet {
	full, ok := l.commit(ctx, id)
	if !ok {
		return nil
	}
	out, err := runGit(ctx, l.root, gitCatFile, "commit", full)
	if err != nil {
		return nil
	}
	header, msg, _ := strings.Cut(string(out), "\n\n")
	subject, body, _ := strings.Cut(strings.TrimSpace(msg), "\n")
	if secretInText(msg) != "" {
		return nil
	}
	s := &assets.Snippet{Kind: "commit", Label: id, Title: subject, Sha: full, Body: truncateRunes(strings.TrimSpace(body), refBodyMax)}
	for h := range strings.SplitSeq(header, "\n") {
		if rest, ok := strings.CutPrefix(h, "author "); ok {
			s.Author, s.Date = parseGitIdent(rest)
		}
	}
	if l.repo != "" {
		s.URL = fmt.Sprintf("https://github.com/%s/commit/%s", l.repo, full)
	}
	return s
}

func parseGitIdent(ident string) (string, string) {
	name, rest, ok := strings.Cut(ident, " <")
	if !ok {
		return ident, ""
	}
	_, stamp, _ := strings.Cut(rest, "> ")
	secs, _, _ := strings.Cut(stamp, " ")
	unix, err := strconv.ParseInt(secs, 10, 64)
	if err != nil {
		return name, ""
	}
	return name, time.Unix(unix, 0).UTC().Format(time.DateOnly)
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + "…"
}

package packs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/yasyf/cc-present/internal/toolpath"
)

const (
	buildRecordName = ".cc-present-build.json"
	buildTimeout    = 5 * time.Minute
	buildLogLines   = 20
)

var (
	sourceDirs   = []string{"src", "schema"}
	sourceFiles  = []string{"package.json", "vite.config.ts", "tsconfig.json"}
	bunLockfiles = []string{"bun.lock", "bun.lockb"}
)

var errBuilding = errors.New("building")

type buildRecord struct {
	Digest string `json:"digest"`
}

type bundleState struct {
	stale    bool
	digest   string
	recorded string
}

type bundler interface {
	bundle(dir, entry string) (string, error)
}

type syncBuilds struct {
	ctx context.Context
}

func (s syncBuilds) bundle(dir, entry string) (string, error) {
	st, err := inspectBundle(dir, entry)
	if err != nil {
		return "", err
	}
	if !st.stale {
		return st.recorded, nil
	}
	bun, err := findBun()
	if err != nil {
		return "", err
	}
	return rebuild(s.ctx, dir, entry, bun)
}

type failedBuild struct {
	digest string
	err    error
}

type backgroundBuilds struct {
	ctx context.Context

	mu      sync.Mutex
	running map[string]bool
	failed  map[string]failedBuild
	wg      sync.WaitGroup
}

func newBackgroundBuilds(ctx context.Context) *backgroundBuilds {
	return &backgroundBuilds{ctx: ctx, running: map[string]bool{}, failed: map[string]failedBuild{}}
}

func (b *backgroundBuilds) bundle(dir, entry string) (string, error) {
	st, err := inspectBundle(dir, entry)
	if err != nil {
		return "", err
	}
	if !st.stale {
		return st.recorded, nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.running[dir] {
		return "", errBuilding
	}
	if f, ok := b.failed[dir]; ok && f.digest == st.digest {
		return "", f.err
	}
	bun, err := findBun()
	if err != nil {
		return "", err
	}
	b.running[dir] = true
	b.wg.Go(func() {
		_, err := rebuild(b.ctx, dir, entry, bun)
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.running, dir)
		if err != nil {
			b.failed[dir] = failedBuild{digest: st.digest, err: err}
			slog.Warn("pack build failed", "dir", dir, "err", err)
			return
		}
		delete(b.failed, dir)
		slog.Info("built pack", "dir", dir)
	})
	return "", errBuilding
}

func buildable(dir string) bool {
	return fileExists(filepath.Join(dir, "package.json"))
}

func inspectBundle(dir, entry string) (bundleState, error) {
	files, err := listSources(dir)
	if err != nil {
		return bundleState{}, err
	}
	recorded, err := readBuildRecord(dir)
	if err != nil {
		return bundleState{}, err
	}
	info, err := os.Stat(filepath.Join(dir, entry))
	if errors.Is(err, fs.ErrNotExist) {
		digest, err := digestSources(dir, files)
		return bundleState{stale: true, digest: digest, recorded: recorded}, err
	}
	if err != nil {
		return bundleState{}, fmt.Errorf("stat entry: %w", err)
	}
	newest, err := newestModTime(dir, files)
	if err != nil {
		return bundleState{}, err
	}
	if !newest.After(info.ModTime()) {
		return bundleState{recorded: recorded}, nil
	}
	digest, err := digestSources(dir, files)
	if err != nil {
		return bundleState{}, err
	}
	return bundleState{stale: digest != recorded, digest: digest, recorded: recorded}, nil
}

func listSources(dir string) ([]string, error) {
	var files []string
	fsys := os.DirFS(dir)
	for _, d := range sourceDirs {
		if _, err := fs.Stat(fsys, d); errors.Is(err, fs.ErrNotExist) {
			continue
		}
		err := fs.WalkDir(fsys, d, func(p string, e fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !e.IsDir() {
				files = append(files, p)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("list sources: %w", err)
		}
	}
	for _, f := range sourceFiles {
		if fileExists(filepath.Join(dir, f)) {
			files = append(files, f)
		}
	}
	sort.Strings(files)
	return files, nil
}

func newestModTime(dir string, files []string) (time.Time, error) {
	var newest time.Time
	for _, f := range files {
		info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(f)))
		if err != nil {
			return time.Time{}, fmt.Errorf("stat source: %w", err)
		}
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	return newest, nil
}

func digestSources(dir string, files []string) (string, error) {
	h := sha256.New()
	for _, f := range files {
		//nolint:gosec // G304: hashing the pack's own source files is the digest's purpose.
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(f)))
		if err != nil {
			return "", fmt.Errorf("digest sources: %w", err)
		}
		_, _ = h.Write([]byte(f))
		_, _ = h.Write([]byte{0})
		_ = binary.Write(h, binary.BigEndian, uint64(len(data)))
		_, _ = h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func buildRecordPath(dir string) string {
	return filepath.Join(dir, "dist", buildRecordName)
}

func readBuildRecord(dir string) (string, error) {
	data, err := os.ReadFile(buildRecordPath(dir))
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read build record: %w", err)
	}
	var rec buildRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return "", fmt.Errorf("parse build record: %w", err)
	}
	return rec.Digest, nil
}

func writeBuildRecord(dir, digest string) error {
	data, err := json.Marshal(buildRecord{Digest: digest})
	if err != nil {
		return err
	}
	//nolint:gosec // G306: the record sits beside the bundle the daemon serves, readable like it.
	if err := os.WriteFile(buildRecordPath(dir), data, 0o644); err != nil {
		return fmt.Errorf("write build record: %w", err)
	}
	return nil
}

func rebuild(ctx context.Context, dir, entry, bun string) (string, error) {
	unlock, err := lockBuild(dir)
	if err != nil {
		return "", err
	}
	defer unlock()
	st, err := inspectBundle(dir, entry)
	if err != nil {
		return "", err
	}
	if !st.stale {
		return st.recorded, nil
	}
	staged, err := stageSources(dir)
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(staged) }()
	started := time.Now()
	if err := runBuild(ctx, dir, staged, bun); err != nil {
		return "", fmt.Errorf("build failed: %w", err)
	}
	entryPath := filepath.Join(staged, entry)
	if !fileExists(entryPath) {
		return "", fmt.Errorf("build did not produce entry %q", entry)
	}
	// A source edited mid-build must read as newer than the bundle it missed.
	if err := os.Chtimes(entryPath, started, started); err != nil {
		return "", fmt.Errorf("backdate entry: %w", err)
	}
	if err := writeBuildRecord(staged, st.digest); err != nil {
		return "", err
	}
	if err := swapDist(filepath.Join(staged, "dist"), filepath.Join(dir, "dist")); err != nil {
		return "", fmt.Errorf("swap dist: %w", err)
	}
	return st.digest, nil
}

func runBuild(ctx context.Context, dir, staged, bun string) error {
	ctx, cancel := context.WithTimeout(ctx, buildTimeout)
	defer cancel()
	var log bytes.Buffer
	path := strings.Join([]string{filepath.Join(dir, "node_modules", ".bin"), filepath.Dir(bun), os.Getenv("PATH")}, string(os.PathListSeparator))
	for _, step := range []struct {
		dir  string
		args []string
	}{{dir, installArgs(dir)}, {staged, []string{"run", "build"}}} {
		//nolint:gosec // G204: bun is the resolved toolchain and args are fixed install/build verbs.
		cmd := exec.CommandContext(ctx, bun, step.args...)
		cmd.Dir = step.dir
		cmd.Env = append(os.Environ(), "PATH="+path)
		cmd.Stdout = &log
		cmd.Stderr = &log
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("bun %s: %w\n%s", strings.Join(step.args, " "), err, logTail(log.String(), buildLogLines))
		}
	}
	return nil
}

func installArgs(dir string) []string {
	for _, f := range bunLockfiles {
		if fileExists(filepath.Join(dir, f)) {
			return []string{"install", "--frozen-lockfile"}
		}
	}
	return []string{"install"}
}

func logTail(log string, n int) string {
	lines := strings.Split(strings.TrimRight(log, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func findBun() (string, error) {
	bun, err := toolpath.Find("bun")
	if err != nil {
		return "", fmt.Errorf("build needs bun: %w", err)
	}
	return bun, nil
}

// The lock lives outside the pack so the build emptying dist/ cannot unlink it.
func lockBuild(dir string) (func(), error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve pack dir: %w", err)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("locate cache dir: %w", err)
	}
	locks := filepath.Join(cache, "cc-present", "pack-builds")
	if err := os.MkdirAll(locks, 0o700); err != nil {
		return nil, fmt.Errorf("create build lock dir: %w", err)
	}
	sum := sha256.Sum256([]byte(abs))
	//nolint:gosec // G304: the lock path is derived from a hash under the user cache dir.
	f, err := os.OpenFile(filepath.Join(locks, hex.EncodeToString(sum[:8])+".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open build lock: %w", err)
	}
	//nolint:gosec // G115: a file descriptor always fits in an int.
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("lock build: %w", err)
	}
	return func() { _ = f.Close() }, nil
}

func stageSources(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("stage sources: %w", err)
	}
	staged, err := os.MkdirTemp(filepath.Dir(abs), "."+filepath.Base(abs)+".cc-present-build-")
	if err != nil {
		return "", fmt.Errorf("stage sources: %w", err)
	}
	err = filepath.WalkDir(abs, func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(abs, p)
		if err != nil || rel == "." {
			return err
		}
		switch rel {
		case "dist", "node_modules", ManifestName:
			if e.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		info, err := e.Info()
		if err != nil {
			return err
		}
		if e.IsDir() {
			return os.Mkdir(filepath.Join(staged, rel), info.Mode().Perm())
		}
		//nolint:gosec // G304: copying the pack's own files into its staging dir.
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		//nolint:gosec // G703: rel comes from walking the pack root, so it stays inside the staging dir.
		return os.WriteFile(filepath.Join(staged, rel), data, info.Mode().Perm())
	})
	if err == nil {
		err = os.Symlink(filepath.Join(abs, "node_modules"), filepath.Join(staged, "node_modules"))
	}
	if err != nil {
		return "", errors.Join(fmt.Errorf("stage sources: %w", err), os.RemoveAll(staged))
	}
	return staged, nil
}

func swapDist(built, dist string) error {
	if _, err := os.Lstat(dist); errors.Is(err, fs.ErrNotExist) {
		return os.Rename(built, dist)
	}
	return exchange(built, dist)
}

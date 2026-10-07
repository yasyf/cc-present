package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yasyf/cc-present/internal/doc"
)

const mainGo = "package main\n\nfunc main() {\n\tprintln(\"hi\")\n}\n"

func gitRepo(t *testing.T, files map[string]string) (string, string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	dir := t.TempDir()
	for name, body := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) string {
		//nolint:gosec // G204: the test's own git setup over a t.TempDir() repo.
		out, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	git("add", ".")
	git("commit", "-q", "-m", "init")
	return dir, git("rev-parse", "--short", "HEAD")
}

func codeBlock(t *testing.T, raw string) *doc.Code {
	t.Helper()
	b, err := doc.DecodeBlock(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("decode code block: %v", err)
	}
	return b.(*doc.Code)
}

func TestFillCode(t *testing.T) {
	root, head := gitRepo(t, map[string]string{"cmd/main.go": mainGo})
	c := codeBlock(t, `{"id":"c1","type":"code","src":"cmd/main.go","lines":"3-5","highlight":"4","pins":[{"line":4,"title":"prints"}]}`)
	if err := fillCode(t.Context(), []doc.Block{c}, root); err != nil {
		t.Fatalf("fillCode: %v", err)
	}
	want := "func main() {\n\tprintln(\"hi\")\n}"
	if c.Code != want || c.Start != 3 || c.Lang != "go" || c.Sha != head {
		t.Fatalf("filled = {code %q, start %d, lang %q, sha %q}, want {%q, 3, go, %q}", c.Code, c.Start, c.Lang, c.Sha, want, head)
	}
	if err := (&doc.Doc{Version: 1, Title: "T", Blocks: []doc.Block{c}}).Validate(doc.NoPacks); err != nil {
		t.Fatalf("validate filled block: %v", err)
	}
}

func TestFillCodeWholeFileInsideCard(t *testing.T) {
	root, head := gitRepo(t, map[string]string{"main.go": mainGo})
	dd := mustDoc(t, `{"version":1,"title":"T","blocks":[{"id":"k1","type":"card","children":[{"id":"c1","type":"code","lang":"golang","src":"main.go"}]}]}`)
	if err := fillCode(t.Context(), dd.Blocks, root); err != nil {
		t.Fatalf("fillCode: %v", err)
	}
	c := dd.Blocks[0].(*doc.Card).Children[0].(*doc.Code)
	if c.Code != strings.TrimSuffix(mainGo, "\n") || c.Start != 1 || c.Lang != "golang" || c.Sha != head {
		t.Fatalf("filled = {code %q, start %d, lang %q, sha %q}", c.Code, c.Start, c.Lang, c.Sha)
	}
}

func TestFillCodeStampsWorkingTree(t *testing.T) {
	root, head := gitRepo(t, map[string]string{"main.go": mainGo})
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(mainGo+"// edited\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := codeBlock(t, `{"id":"c1","type":"code","src":"main.go","lines":"1"}`)
	if err := fillCode(t.Context(), []doc.Block{c}, root); err != nil {
		t.Fatalf("fillCode: %v", err)
	}
	if c.Sha != head+"+wt" {
		t.Fatalf("sha = %q, want %q", c.Sha, head+"+wt")
	}
}

func TestFillCodeRefuses(t *testing.T) {
	awsKey := "AKIA" + "IOSFODNN7EXAMPLE"
	root, _ := gitRepo(t, map[string]string{
		"main.go":   mainGo,
		".env":      "PORT=1\n",
		"keys.go":   fmt.Sprintf("package keys\n\nconst id = %q\n", awsKey),
		"cfg/a.txt": "a\n",
	})
	outside := filepath.Join(t.TempDir(), "outside.go")
	if err := os.WriteFile(outside, []byte(mainGo), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, ".env"), filepath.Join(root, "cfg", "dotenv")); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"symlink escape", `"src":"escape.go"`, `src "escape.go" resolves outside`},
		{"dotenv by name", `"src":".env"`, `looks like a secret file (dotfile credential)`},
		{"dotenv through a symlink", `"src":"cfg/dotenv"`, `resolves to .env, which looks like a secret file (dotfile credential)`},
		{"aws key in the file", `"src":"keys.go"`, `holds a secret (AWS access key id)`},
		{"range out of bounds", `"src":"main.go","lines":"4-9"`, `lines "4-9" but main.go has 5 lines`},
		{"missing file", `"src":"nope.go"`, `src "nope.go" not found`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := codeBlock(t, fmt.Sprintf(`{"id":"c1","type":"code",%s}`, tt.src))
			err := fillCode(t.Context(), []doc.Block{c}, root)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("fillCode() err = %v, want containing %q", err, tt.want)
			}
			if !strings.HasPrefix(err.Error(), `code "c1": `) {
				t.Fatalf("fillCode() err = %v, want it to name the block", err)
			}
			if c.Code != "" {
				t.Fatalf("code = %q, want unfilled", c.Code)
			}
		})
	}
}

func TestFillCodeJoinsErrorsPerBlock(t *testing.T) {
	root, _ := gitRepo(t, map[string]string{"main.go": mainGo})
	blocks := []doc.Block{
		codeBlock(t, `{"id":"a","type":"code","src":"x.go"}`),
		codeBlock(t, `{"id":"b","type":"code","src":"main.go"}`),
		codeBlock(t, `{"id":"c","type":"code","src":"y.go"}`),
	}
	err := fillCode(t.Context(), blocks, root)
	if err == nil {
		t.Fatal("fillCode() err = nil, want two failures")
	}
	lines := strings.Split(err.Error(), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], `code "a"`) || !strings.HasPrefix(lines[1], `code "c"`) {
		t.Fatalf("fillCode() err = %q, want one line each for a and c", err)
	}
}

func TestLongCodeNudge(t *testing.T) {
	long := codeBlock(t, fmt.Sprintf(`{"id":"long","type":"code","lang":"go","src":"a.go","code":%q}`, strings.Repeat("x\n", 40)+"x"))
	short := codeBlock(t, fmt.Sprintf(`{"id":"short","type":"code","lang":"go","src":"b.go","code":%q}`, strings.Repeat("x\n", 39)+"x"))
	pasted := codeBlock(t, fmt.Sprintf(`{"id":"pasted","type":"code","lang":"go","code":%q}`, strings.Repeat("x\n", 60)))
	got := longCodeNudge([]doc.Block{long, short, pasted})
	want := "hint: code long shows more than 40 lines; slice src with lines to the part that carries the point"
	if got != want {
		t.Fatalf("longCodeNudge() = %q, want %q", got, want)
	}
}

package packs

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestLintExamplePack lints the committed reference pack in examples/packs/example
// with a stubbed bundle, so go test needs no JS toolchain. It is the Go half of
// the pack's CI gate: the real manifest, schemas, and examples must validate.
func TestLintExamplePack(t *testing.T) {
	src := filepath.Join("..", "..", "examples", "packs", "example")
	dir := t.TempDir()
	copyPackTree(t, src, dir)
	// dist/ is not committed; stub the bundle lint only checks for existence.
	writeTreeInto(t, dir, map[string]string{"dist/pack.js": "0"})

	p, err := Lint(t.Context(), dir)
	if err != nil {
		t.Fatalf("Lint(example pack): %v", err)
	}
	if p.Name != "example" {
		t.Errorf("pack name = %q, want %q", p.Name, "example")
	}
	byName := map[string]*BlockType{}
	for _, bt := range p.Blocks {
		byName[bt.Name] = bt
	}
	if byName["callout"] == nil || byName["rating"] == nil || byName["survey"] == nil {
		t.Fatalf("blocks = %v, want callout, rating, and survey", blockNames(p))
	}
	if byName["callout"].Interactive() {
		t.Errorf("callout should be content-only, got interactive")
	}
	if !byName["rating"].Interactive() {
		t.Errorf("rating should be interactive")
	}
	if !byName["survey"].Interactive() {
		t.Errorf("survey should be interactive")
	}
}

func blockNames(p *Pack) []string {
	out := make([]string, 0, len(p.Blocks))
	for _, bt := range p.Blocks {
		out = append(out, bt.Name)
	}
	return out
}

func copyPackTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "node_modules" || d.Name() == "dist" {
				return fs.SkipDir
			}
			return nil
		}
		//nolint:gosec // G304: reading the repo's own committed example pack tree in a test.
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(out), 0o750); err != nil {
			return err
		}
		//nolint:gosec // G703: out is rooted at t.TempDir(); rel comes from the trusted source tree.
		return os.WriteFile(out, data, 0o600)
	})
	if err != nil {
		t.Fatalf("copy pack tree: %v", err)
	}
}

// TestLintPluginPack lints the pr pack the plugin ships at plugin/.claude/components.
func TestLintPluginPack(t *testing.T) {
	src := filepath.Join("..", "..", "plugin", ".claude", "components")
	dir := t.TempDir()
	copyPackTree(t, src, dir)
	writeTreeInto(t, dir, map[string]string{"dist/pack.js": "0"})

	p, err := Lint(t.Context(), dir)
	if err != nil {
		t.Fatalf("Lint(pr pack): %v", err)
	}
	if p.Name != "pr" {
		t.Errorf("pack name = %q, want %q", p.Name, "pr")
	}
	got := blockNames(p)
	slices.Sort(got)
	want := []string{"card", "commits", "diff"}
	if !slices.Equal(got, want) {
		t.Fatalf("blocks = %v, want %v", got, want)
	}
	for _, bt := range p.Blocks {
		if bt.Interactive() {
			t.Errorf("%s should be content-only, got interactive", bt.Name)
		}
	}
}

func TestLintDisplayPack(t *testing.T) {
	src := filepath.Join("..", "..", "plugin", ".claude", "components", "display")
	dir := t.TempDir()
	copyPackTree(t, src, dir)
	writeTreeInto(t, dir, map[string]string{"dist/pack.js": "0", "dist/pack.css": ""})

	p, err := Lint(t.Context(), dir)
	if err != nil {
		t.Fatalf("Lint(display pack): %v", err)
	}
	if p.Name != "display" {
		t.Errorf("pack name = %q, want %q", p.Name, "display")
	}
	got := blockNames(p)
	slices.Sort(got)
	if want := []string{"artifact", "compare", "page", "sequence", "timeline"}; !slices.Equal(got, want) {
		t.Fatalf("blocks = %v, want %v", got, want)
	}
	for _, bt := range p.Blocks {
		if bt.Interactive() {
			t.Errorf("%s should be read-only, got interactive", bt.Name)
		}
	}
}

func TestDisplayPackSchemas(t *testing.T) {
	src := filepath.Join("..", "..", "plugin", ".claude", "components", "display")
	dir := t.TempDir()
	copyPackTree(t, src, dir)
	writeTreeInto(t, dir, map[string]string{"dist/pack.js": "0", "dist/pack.css": ""})
	reg := buildRegistry([]packRoot{{dir: dir, tier: tierDev}}, nil, nil, syncBuilds{ctx: t.Context()})
	if len(reg.Dropped) != 0 {
		t.Fatalf("dropped = %+v", reg.Dropped)
	}

	tests := []struct {
		name, block string
		ok          bool
	}{
		{"artifact html", `{"id":"a","type":"display.artifact","kind":"html","source":"<p>x</p>"}`, true},
		{"artifact script kind", `{"id":"a","type":"display.artifact","kind":"js","source":"x"}`, false},
		{"artifact empty source", `{"id":"a","type":"display.artifact","kind":"svg","source":""}`, false},
		{"artifact height floor", `{"id":"a","type":"display.artifact","kind":"svg","source":"x","height":10}`, false},
		{"page", `{"id":"p","type":"display.page","md":"# hi"}`, true},
		{"page multiline title", `{"id":"p","type":"display.page","md":"x","title":"a\nb"}`, false},
		{"sequence note step", `{"id":"s","type":"display.sequence","panels":[{"actors":[{"id":"a","label":"A"}],"steps":[{"from":"a","label":"think"}]}]}`, true},
		{"sequence four panels", `{"id":"s","type":"display.sequence","panels":[` + strings.Repeat(`{"actors":[{"id":"a","label":"A"}],"steps":[{"from":"a","label":"x"}]},`, 3) + `{"actors":[{"id":"a","label":"A"}],"steps":[{"from":"a","label":"x"}]}]}`, false},
		{"sequence fast interval", `{"id":"s","type":"display.sequence","intervalMs":100,"panels":[{"actors":[{"id":"a","label":"A"}],"steps":[{"from":"a","label":"x"}]}]}`, false},
		{"timeline http url", `{"id":"t","type":"display.timeline","events":[{"when":"Oct 3","title":"x","url":"http://example.com"}]}`, false},
		{"timeline", `{"id":"t","type":"display.timeline","events":[{"when":"Oct 3","title":"x","tone":"danger","url":"https://example.com/x"}]}`, true},
		{"compare mixed cells", `{"id":"c","type":"display.compare","columns":[{"label":"A"},{"label":"B"}],"rows":[{"label":"r","cells":["yes",{"md":"no","tone":"bad"}]}]}`, true},
		{"compare unknown tone", `{"id":"c","type":"display.compare","columns":[{"label":"A"}],"rows":[{"label":"r","cells":[{"md":"no","tone":"red"}]}]}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var typ struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal([]byte(tt.block), &typ); err != nil {
				t.Fatal(err)
			}
			err := reg.ValidateBlock(typ.Type, json.RawMessage(tt.block))
			if (err == nil) != tt.ok {
				t.Fatalf("ValidateBlock err = %v, want ok=%v", err, tt.ok)
			}
		})
	}
}

func TestLintPlanPack(t *testing.T) {
	src := filepath.Join("..", "..", "plugin", ".claude", "components", "plan")
	dir := t.TempDir()
	copyPackTree(t, src, dir)
	writeTreeInto(t, dir, map[string]string{"dist/pack.js": "0", "dist/pack.css": ""})

	p, err := Lint(t.Context(), dir)
	if err != nil {
		t.Fatalf("Lint(plan pack): %v", err)
	}
	if p.Name != "plan" {
		t.Errorf("pack name = %q, want %q", p.Name, "plan")
	}
	got := blockNames(p)
	slices.Sort(got)
	if want := []string{"calls", "machine", "mock"}; !slices.Equal(got, want) {
		t.Fatalf("blocks = %v, want %v", got, want)
	}
	for _, bt := range p.Blocks {
		wantInteractive := bt.Name != "machine"
		if bt.Interactive() != wantInteractive || bt.Optional != wantInteractive {
			t.Errorf("%s interactive=%v optional=%v, want both %v", bt.Name, bt.Interactive(), bt.Optional, wantInteractive)
		}
	}
}

func TestPlanPackSchemas(t *testing.T) {
	src := filepath.Join("..", "..", "plugin", ".claude", "components", "plan")
	dir := t.TempDir()
	copyPackTree(t, src, dir)
	writeTreeInto(t, dir, map[string]string{"dist/pack.js": "0", "dist/pack.css": ""})
	reg := buildRegistry([]packRoot{{dir: dir, tier: tierDev}}, nil, nil, syncBuilds{ctx: t.Context()})
	if len(reg.Dropped) != 0 {
		t.Fatalf("dropped = %+v", reg.Dropped)
	}

	blocks := []struct {
		name, block string
		ok          bool
	}{
		{"calls nested", `{"id":"c","type":"plan.calls","calls":[{"call":"a()","mark":"~","calls":[{"id":"b","call":"b()","mark":"+","new":true,"at":"web/b.ts:12"}]}]}`, true},
		{"calls missing mark", `{"id":"c","type":"plan.calls","calls":[{"call":"a()"}]}`, false},
		{"calls unknown mark", `{"id":"c","type":"plan.calls","calls":[{"call":"a()","mark":"*"}]}`, false},
		{"calls at without line", `{"id":"c","type":"plan.calls","calls":[{"call":"a()","mark":"+","at":"web/b.ts"}]}`, false},
		{"calls multiline call", `{"id":"c","type":"plan.calls","calls":[{"call":"a()\nb()","mark":"+"}]}`, false},
		{"machine grid", `{"id":"m","type":"plan.machine","states":[{"id":"a"},{"id":"b","final":true,"screen":{"md":"x"}}],"grid":[["a","b"],[null,null]],"transitions":[{"from":"a","event":"go","to":"b","mark":"?"}]}`, true},
		{"machine mock screen", `{"id":"m","type":"plan.machine","states":[{"id":"a","screen":{"mock":{"html":"<p>x</p>","frame":"phone"}}}],"transitions":[]}`, true},
		{"machine screen both", `{"id":"m","type":"plan.machine","states":[{"id":"a","screen":{"md":"x","mock":{"html":"x"}}}],"transitions":[]}`, false},
		{"machine screen pins", `{"id":"m","type":"plan.machine","states":[{"id":"a","screen":{"mock":{"html":"x","pins":[]}}}],"transitions":[]}`, false},
		{"machine state mark ask", `{"id":"m","type":"plan.machine","states":[{"id":"a","mark":"?"}],"transitions":[]}`, false},
		{"machine bad state id", `{"id":"m","type":"plan.machine","states":[{"id":"a b"}],"transitions":[]}`, false},
		{"mock pins", `{"id":"k","type":"plan.mock","html":"<b data-ref=\"x\">x</b>","w":440,"frame":"browser","url":"https://a.test","pins":[{"ref":"x","title":"New"}]}`, true},
		{"mock narrow", `{"id":"k","type":"plan.mock","html":"x","w":100}`, false},
		{"mock unknown frame", `{"id":"k","type":"plan.mock","html":"x","frame":"watch"}`, false},
		{"mock bad ref", `{"id":"k","type":"plan.mock","html":"x","pins":[{"ref":"1x","title":"t"}]}`, false},
	}
	for _, tt := range blocks {
		t.Run(tt.name, func(t *testing.T) {
			var typ struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal([]byte(tt.block), &typ); err != nil {
				t.Fatal(err)
			}
			err := reg.ValidateBlock(typ.Type, json.RawMessage(tt.block))
			if (err == nil) != tt.ok {
				t.Fatalf("ValidateBlock err = %v, want ok=%v", err, tt.ok)
			}
		})
	}

	interactions := []struct {
		name, typ, payload string
		ok                 bool
	}{
		{"calls struck", "plan.calls", `{"struck":["b","0.1"]}`, true},
		{"calls duplicate", "plan.calls", `{"struck":["b","b"]}`, false},
		{"calls extra key", "plan.calls", `{"struck":[],"note":"x"}`, false},
		{"mock comments", "plan.mock", `{"comments":{"later":"split it"}}`, true},
		{"mock empty comment", "plan.mock", `{"comments":{"later":""}}`, false},
		{"mock bad ref", "plan.mock", `{"comments":{"1x":"y"}}`, false},
		{"machine read-only", "plan.machine", `{}`, false},
	}
	for _, tt := range interactions {
		t.Run(tt.name, func(t *testing.T) {
			err := reg.ValidateInteraction(tt.typ, json.RawMessage(tt.payload))
			if (err == nil) != tt.ok {
				t.Fatalf("ValidateInteraction err = %v, want ok=%v", err, tt.ok)
			}
		})
	}
}

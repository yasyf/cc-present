package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yasyf/cc-present/internal/assets"
	"github.com/yasyf/cc-present/internal/doc"
)

func writeImage(t *testing.T, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pic.png")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func imageBlock(t *testing.T, src string) doc.Block {
	t.Helper()
	raw := fmt.Sprintf(`{"id":"img1","type":"image","src":%q,"alt":"a"}`, src)
	b, err := doc.DecodeBlock(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("decode image block: %v", err)
	}
	return b
}

func TestInlineImagesRewritesLocal(t *testing.T) {
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{3}, 8)...)
	path := writeImage(t, png)
	blk := imageBlock(t, path)
	if err := inlineImages([]doc.Block{blk}, localUploader); err != nil {
		t.Fatalf("inline: %v", err)
	}
	want := "asset:" + assets.SHA(png)
	if got := blk.(*doc.Image).Src; got != want {
		t.Fatalf("src = %q, want %q", got, want)
	}
}

func TestInlineImagesLeavesResolvedSrc(t *testing.T) {
	for _, src := range []string{
		"https://example.com/x.png",
		"asset:" + strings.Repeat("a", 64),
		"data:image/png;base64,AAAA",
	} {
		blk := imageBlock(t, src)
		if err := inlineImages([]doc.Block{blk}, func(string) (string, error) {
			t.Fatal("uploader called for a resolved src")
			return "", nil
		}); err != nil {
			t.Fatalf("inline %q: %v", src, err)
		}
		if got := blk.(*doc.Image).Src; got != src {
			t.Fatalf("src = %q, want unchanged %q", got, src)
		}
	}
}

func TestInlineImagesRecursesIntoCards(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\nxyz")
	path := writeImage(t, png)
	raw := fmt.Sprintf(`{"id":"c1","type":"card","children":[{"id":"img1","type":"image","src":%q,"alt":"a"}]}`, path)
	blk, err := doc.DecodeBlock(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("decode card: %v", err)
	}
	if err := inlineImages([]doc.Block{blk}, localUploader); err != nil {
		t.Fatalf("inline: %v", err)
	}
	child := blk.(*doc.Card).Children[0].(*doc.Image)
	if child.Src != "asset:"+assets.SHA(png) {
		t.Fatalf("child src = %q", child.Src)
	}
}

func TestInlineImagesErrors(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		blk := imageBlock(t, filepath.Join(t.TempDir(), "absent.png"))
		if err := inlineImages([]doc.Block{blk}, localUploader); err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("err = %v, want 'not found'", err)
		}
	})
	t.Run("oversize file", func(t *testing.T) {
		path := writeImage(t, bytes.Repeat([]byte{1}, assets.MaxBytes+1))
		blk := imageBlock(t, path)
		if err := inlineImages([]doc.Block{blk}, localUploader); err == nil || !strings.Contains(err.Error(), "exceeds") {
			t.Fatalf("err = %v, want 'exceeds'", err)
		}
	})
}

// TestDryRunFlow exercises the push --dry-run pipeline: inline images locally,
// then validate the resulting document.
func TestDryRunFlow(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\nQ")
	path := writeImage(t, png)
	valid := fmt.Sprintf(`{"version":1,"title":"T","blocks":[{"id":"img1","type":"image","src":%q,"alt":"a"}]}`, path)
	dd := &doc.Doc{}
	if err := json.Unmarshal([]byte(valid), dd); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if err := inlineImages(dd.Blocks, localUploader); err != nil {
		t.Fatalf("inline: %v", err)
	}
	if err := dd.Validate(doc.NoPacks); err != nil {
		t.Fatalf("validate after inline: %v", err)
	}

	invalid := &doc.Doc{}
	if err := json.Unmarshal([]byte(`{"version":1,"title":"T","blocks":[{"id":"p1","type":"progress","label":"x","value":9,"max":2}]}`), invalid); err != nil {
		t.Fatalf("decode invalid: %v", err)
	}
	if err := invalid.Validate(doc.NoPacks); err == nil {
		t.Fatal("invalid doc validated, want error")
	}
}

package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ccevent "github.com/yasyf/cc-interact/event"
	ccstore "github.com/yasyf/cc-interact/store"

	"github.com/yasyf/cc-present/internal/assets"
)

func TestGCAssets(t *testing.T) {
	shaCardKept := strings.Repeat("a", 64)
	shaTopKept := strings.Repeat("b", 64)
	shaClosed := strings.Repeat("c", 64)
	shaUnrefOld := strings.Repeat("d", 64)
	shaUnrefYoung := strings.Repeat("e", 64)

	cc, err := ccstore.Open(context.Background(), filepath.Join(t.TempDir(), "t.db"), ccstore.Schema{})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = cc.Close() })
	assetsDir := filepath.Join(t.TempDir(), "assets")
	ast := assets.New(assetsDir)
	if err := ast.Prepare(); err != nil {
		t.Fatalf("prepare assets: %v", err)
	}

	// An open subject references shaCardKept via a card child and shaTopKept via a
	// top-level block; a closed subject references shaClosed.
	seedSubject(t, cc, "open-card", "/s1", statusOpen, cardImageDoc(shaCardKept))
	seedSubject(t, cc, "open-top", "/s2", statusOpen, topImageDoc(shaTopKept))
	seedSubject(t, cc, "closed", "/s3", statusClosed, topImageDoc(shaClosed))

	old := time.Now().Add(-30 * time.Minute)
	young := time.Now()
	writeAsset(t, assetsDir, shaCardKept, old)
	writeAsset(t, assetsDir, shaTopKept, old)
	writeAsset(t, assetsDir, shaClosed, old)
	writeAsset(t, assetsDir, shaUnrefOld, old)
	writeAsset(t, assetsDir, shaUnrefYoung, young)

	if err := gcAssets(context.Background(), cc.DB(), ast); err != nil {
		t.Fatalf("gcAssets: %v", err)
	}

	cases := []struct {
		name       string
		sha        string
		wantExists bool
	}{
		{"referenced by an open card child is kept", shaCardKept, true},
		{"referenced by an open top-level block is kept", shaTopKept, true},
		{"referenced only by a closed subject is deleted", shaClosed, false},
		{"unreferenced and older than grace is deleted", shaUnrefOld, false},
		{"unreferenced but within grace is kept", shaUnrefYoung, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, statErr := os.Stat(filepath.Join(assetsDir, tc.sha))
			if exists := statErr == nil; exists != tc.wantExists {
				t.Fatalf("asset %s exists=%v, want %v", tc.sha, exists, tc.wantExists)
			}
		})
	}
}

func topImageDoc(sha string) string {
	return fmt.Sprintf(`{"version":1,"title":"T","blocks":[{"id":"img","type":"image","src":"asset:%s","alt":"a"}]}`, sha)
}

func cardImageDoc(sha string) string {
	return fmt.Sprintf(
		`{"version":1,"title":"T","blocks":[{"id":"c","type":"card","children":[{"id":"img","type":"image","src":"asset:%s","alt":"a"}]}]}`,
		sha,
	)
}

// seedSubject creates a subject with the given status and appends one
// doc.replaced carrying docJSON, the same shape handlePush writes.
func seedSubject(t *testing.T, cc *ccstore.Store, id, scope, status, docJSON string) {
	t.Helper()
	ss := ccstore.NewSubjectStore(cc.DB())
	if _, err := ss.Create(context.Background(), id, id, "", scope, 0, status); err != nil {
		t.Fatalf("create subject %s: %v", id, err)
	}
	payload := fmt.Sprintf(`{"doc":%s,"revision":1}`, docJSON)
	if _, err := cc.AppendEvent(context.Background(), &ccevent.Event{
		SubjectID: id, Origin: ccevent.OriginAgent, Type: EventDocReplaced,
		Payload: json.RawMessage(payload),
	}); err != nil {
		t.Fatalf("append doc.replaced for %s: %v", id, err)
	}
}

func writeAsset(t *testing.T, dir, sha string, mtime time.Time) {
	t.Helper()
	path := filepath.Join(dir, sha)
	if err := os.WriteFile(path, []byte("img"), 0o600); err != nil {
		t.Fatalf("write asset %s: %v", sha, err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("chtimes asset %s: %v", sha, err)
	}
}

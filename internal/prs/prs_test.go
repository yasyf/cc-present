package prs_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yasyf/cc-present/internal/prs"
)

const openPR = `{"data":{"repository":{"pullRequest":{
  "title":"Add the pr block","url":"https://github.com/o/r/pull/7","state":"OPEN","isDraft":false,"mergedAt":null,
  "author":{"login":"yasyf"},"baseRefName":"main","headRefName":"pr-block","headRefOid":"h1","mergeCommit":null,
  "additions":12,"deletions":3,"changedFiles":3,"reviewDecision":"CHANGES_REQUESTED",
  "labels":{"nodes":[{"name":"web"}]},
  "latestOpinionatedReviews":{"nodes":[{"author":{"login":"rev"},"state":"CHANGES_REQUESTED"},{"author":null,"state":"APPROVED"}]},
  "commits":{"nodes":[{"commit":{"statusCheckRollup":{"state":"FAILURE","contexts":{
    "checkRunCountsByState":[{"state":"SUCCESS","count":4},{"state":"FAILURE","count":1},{"state":"SKIPPED","count":2},{"state":"IN_PROGRESS","count":1}],
    "statusContextCountsByState":[{"state":"ERROR","count":1}],
    "nodes":[{"__typename":"CheckRun","name":"go","conclusion":"SUCCESS"},{"__typename":"CheckRun","name":"web","conclusion":"FAILURE"},{"__typename":"CheckRun","name":"ios","conclusion":null},{"__typename":"StatusContext","context":"ci/legacy","state":"ERROR"}]
  }}}}]}
}}}}`

const closedPR = `{"data":{"repository":{"pullRequest":{
  "title":"Queued","url":"https://github.com/o/r/pull/9","state":"CLOSED","isDraft":false,"mergedAt":null,
  "author":{"login":"yasyf"},"baseRefName":"dev","headRefName":"q","mergeCommit":null,
  "additions":1,"deletions":1,"changedFiles":1,"reviewDecision":null,
  "labels":{"nodes":[]},"latestOpinionatedReviews":{"nodes":[]},
  "commits":{"nodes":[{"commit":{"statusCheckRollup":null}}]}
}}}}`

const filePages = `[{"filename":"web/src/a.ts","status":"modified","additions":5,"deletions":1,"patch":"@@ -1 +1 @@\n-a\n+b"},
 {"filename":"go.mod","status":"modified","additions":1,"deletions":1,"patch":"@@ -1 +1 @@\n-x\n+y"}]
[{"filename":"web/src/new.ts","previous_filename":"lib/old.ts","status":"renamed","additions":6,"deletions":1},
 {"filename":"docs/logo.png","status":"added","additions":0,"deletions":0}]`

type fakeGH struct {
	pr     string
	search string
	mu     sync.Mutex
	calls  [][]string
}

func (f *fakeGH) run(_ context.Context, args ...string) ([]byte, error) {
	f.mu.Lock()
	f.calls = append(f.calls, args)
	f.mu.Unlock()
	switch {
	case slices.Contains(args, "graphql"):
		return []byte(f.pr), nil
	case slices.Contains(args, "search/commits"):
		return []byte(f.search), nil
	case slices.Contains(args, "--paginate"):
		return []byte(filePages), nil
	}
	return nil, errors.New("unexpected gh call " + strings.Join(args, " "))
}

var now = time.Date(2026, 10, 14, 21, 0, 0, 0, time.FixedZone("PDT", -7*3600))

func TestFetchOpenPR(t *testing.T) {
	f := &fakeGH{pr: openPR}
	pr, err := new(prs.Cache).Fetch(t.Context(), f.run, "o/r", 7, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if pr.State != "open" || pr.Title != "Add the pr block" || pr.Author != "yasyf" || pr.Base != "main" || pr.Head != "pr-block" {
		t.Fatalf("header = %+v", pr)
	}
	want := prs.Checks{State: "failure", Total: 9, Passed: 4, Failed: 2, Pending: 1, Skipped: 2, Failing: []string{"web", "ci/legacy"}}
	if !slices.Equal(pr.Checks.Failing, want.Failing) || pr.Checks.Total != want.Total || pr.Checks.Passed != want.Passed ||
		pr.Checks.Failed != want.Failed || pr.Checks.Pending != want.Pending || pr.Checks.Skipped != want.Skipped || pr.Checks.State != want.State {
		t.Fatalf("checks = %+v, want %+v", pr.Checks, want)
	}
	if pr.ReviewDecision != "changes_requested" {
		t.Fatalf("reviewDecision = %q", pr.ReviewDecision)
	}
	if len(pr.Reviews) != 2 || pr.Reviews[0] != (prs.Review{Author: "rev", State: "changes_requested"}) || pr.Reviews[1].Author != "ghost" {
		t.Fatalf("reviews = %+v", pr.Reviews)
	}
	if !slices.Equal(pr.Labels, []string{"web"}) {
		t.Fatalf("labels = %v", pr.Labels)
	}
	if len(pr.Files) != 4 || pr.Files[2].PreviousPath != "lib/old.ts" || pr.Files[3].Patch != "" {
		t.Fatalf("files = %+v", pr.Files)
	}
	if !pr.FetchedAt.Equal(now) || pr.FetchedAt.Location() != time.UTC {
		t.Fatalf("fetchedAt = %v", pr.FetchedAt)
	}
	for _, c := range f.calls {
		if slices.Contains(c, "search/commits") {
			t.Fatal("an open PR must not search for a landed commit")
		}
	}
}

func TestCacheRefetches(t *testing.T) {
	tests := []struct {
		name      string
		after     time.Duration
		head      string
		base      string
		wantCalls int
	}{
		{"inside the floor", 30 * time.Second, "h1", "main", 0},
		{"same head after the floor", 2 * time.Minute, "h1", "main", 1},
		{"moved head after the floor", 2 * time.Minute, "h2", "main", 2},
		{"retargeted base after the floor", 2 * time.Minute, "h1", "dev", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeGH{pr: openPR}
			cache := &prs.Cache{Floor: time.Minute}
			if _, err := cache.Fetch(t.Context(), f.run, "o/r", 7, nil, now); err != nil {
				t.Fatal(err)
			}
			first := len(f.calls)
			f.pr = strings.NewReplacer(`"headRefOid":"h1"`, `"headRefOid":"`+tt.head+`"`, `"baseRefName":"main"`, `"baseRefName":"`+tt.base+`"`).Replace(openPR)
			pr, err := cache.Fetch(t.Context(), f.run, "o/r", 7, nil, now.Add(tt.after))
			if err != nil {
				t.Fatal(err)
			}
			if got := len(f.calls) - first; got != tt.wantCalls {
				t.Fatalf("gh calls = %d, want %d", got, tt.wantCalls)
			}
			if len(pr.Files) != 4 {
				t.Fatalf("files = %d, want 4", len(pr.Files))
			}
		})
	}
}

func TestFetchFiltersFiles(t *testing.T) {
	f := &fakeGH{pr: openPR}
	pr, err := new(prs.Cache).Fetch(t.Context(), f.run, "o/r", 7, []string{"web/src", "*.mod"}, now)
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 0, len(pr.Files))
	for _, file := range pr.Files {
		paths = append(paths, file.Path)
	}
	if !slices.Equal(paths, []string{"web/src/a.ts", "go.mod", "web/src/new.ts"}) {
		t.Fatalf("paths = %v", paths)
	}
	if pr.ChangedFiles != 3 {
		t.Fatalf("changedFiles = %d, want the PR's own count", pr.ChangedFiles)
	}
}

func TestFetchLandedThroughQueue(t *testing.T) {
	tests := []struct {
		name       string
		search     string
		wantState  string
		wantMerged string
	}{
		{"squash on base", `{"items":[{"sha":"aaa","commit":{"message":"other (#90)"}},{"sha":"bbb","commit":{"message":"Queued (#9)\n\nbody"}}]}`, "landed", "bbb"},
		{"no squash", `{"items":[{"sha":"aaa","commit":{"message":"other (#90)"}}]}`, "closed", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeGH{pr: closedPR, search: tt.search}
			pr, err := new(prs.Cache).Fetch(t.Context(), f.run, "o/r", 9, nil, now)
			if err != nil {
				t.Fatal(err)
			}
			if pr.State != tt.wantState || pr.MergedSHA != tt.wantMerged {
				t.Fatalf("state = %q merged = %q, want %q %q", pr.State, pr.MergedSHA, tt.wantState, tt.wantMerged)
			}
			if pr.Checks.State != "none" || pr.ReviewDecision != "none" {
				t.Fatalf("checks = %+v review = %q", pr.Checks, pr.ReviewDecision)
			}
		})
	}
}

func TestFetchErrors(t *testing.T) {
	tests := []struct {
		name string
		pr   string
		want string
	}{
		{"missing PR", `{"data":{"repository":{"pullRequest":null}}}`, "o/r#7 not found"},
		{"unknown check state", strings.Replace(openPR, `"IN_PROGRESS"`, `"LATER"`, 1), `unknown check state "LATER"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeGH{pr: tt.pr}
			_, err := new(prs.Cache).Fetch(t.Context(), f.run, "o/r", 7, nil, now)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestSelected(t *testing.T) {
	tests := []struct {
		patterns []string
		file     string
		want     bool
	}{
		{nil, "any/file.go", true},
		{[]string{"go.mod"}, "go.mod", true},
		{[]string{"internal"}, "internal/doc/doc.go", true},
		{[]string{"internal/"}, "internal/doc/doc.go", true},
		{[]string{"internal"}, "internals.go", false},
		{[]string{"web/src/*.ts"}, "web/src/a.ts", true},
		{[]string{"web/src/*.ts"}, "web/src/deep/a.ts", false},
	}
	for _, tt := range tests {
		if got := prs.Selected(tt.patterns, tt.file); got != tt.want {
			t.Errorf("Selected(%v, %q) = %v, want %v", tt.patterns, tt.file, got, tt.want)
		}
	}
}

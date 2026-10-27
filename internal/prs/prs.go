// Package prs reads a GitHub pull request's live state through the user's gh
// CLI for the pr block, so the daemon never holds or forwards a token.
package prs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

const maxFailing = 10

const prQuery = `query($owner: String!, $name: String!, $number: Int!) {
  repository(owner: $owner, name: $name) {
    pullRequest(number: $number) {
      title url state isDraft mergedAt headRefOid
      author { login }
      baseRefName headRefName
      mergeCommit { oid }
      additions deletions changedFiles
      reviewDecision
      labels(first: 20) { nodes { name } }
      latestOpinionatedReviews(first: 20) { nodes { author { login } state } }
      commits(last: 1) {
        nodes { commit { statusCheckRollup { state contexts(first: 100) {
          checkRunCountsByState { state count }
          statusContextCountsByState { state count }
          nodes {
            __typename
            ... on CheckRun { name conclusion }
            ... on StatusContext { context state }
          }
        } } } }
      }
    }
  }
}`

// Runner runs gh with args and returns its stdout.
type Runner func(ctx context.Context, args ...string) ([]byte, error)

// GH is the Runner for the gh binary at bin, with prompts, pagers, and update
// checks disabled.
func GH(bin string) Runner {
	return func(ctx context.Context, args ...string) ([]byte, error) {
		//nolint:gosec // G204: bin is the resolved gh binary and args never reach a shell.
		cmd := exec.CommandContext(ctx, bin, args...)
		cmd.Env = append(os.Environ(), "GH_PROMPT_DISABLED=1", "GH_NO_UPDATE_NOTIFIER=1", "GH_PAGER=cat", "NO_COLOR=1")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("gh %s: %w: %s", strings.Join(args[:min(2, len(args))], " "), err, strings.TrimSpace(stderr.String()))
		}
		return out, nil
	}
}

// Checks is the head commit's check rollup.
type Checks struct {
	State   string   `json:"state"`
	Total   int      `json:"total"`
	Passed  int      `json:"passed"`
	Failed  int      `json:"failed"`
	Pending int      `json:"pending"`
	Skipped int      `json:"skipped"`
	Failing []string `json:"failing,omitempty"`
}

// Review is one reviewer's latest approving or blocking review.
type Review struct {
	Author string `json:"author"`
	State  string `json:"state"`
}

// File is one changed file with its unified patch; Patch is empty when GitHub
// omits it for a binary or oversized file.
type File struct {
	Path         string `json:"path"`
	PreviousPath string `json:"previousPath,omitempty"`
	Status       string `json:"status"`
	Additions    int    `json:"additions"`
	Deletions    int    `json:"deletions"`
	Patch        string `json:"patch,omitempty"`
}

// PR is the live state the pr block renders. State is open, draft, merged,
// landed (closed with a "(#N)" squash on the default branch), or closed.
type PR struct {
	Repo           string    `json:"repo"`
	Number         int       `json:"number"`
	Title          string    `json:"title"`
	URL            string    `json:"url"`
	Author         string    `json:"author"`
	State          string    `json:"state"`
	Base           string    `json:"base"`
	Head           string    `json:"head"`
	MergedSHA      string    `json:"mergedSha,omitempty"`
	Additions      int       `json:"additions"`
	Deletions      int       `json:"deletions"`
	ChangedFiles   int       `json:"changedFiles"`
	Checks         Checks    `json:"checks"`
	ReviewDecision string    `json:"reviewDecision"`
	Reviews        []Review  `json:"reviews"`
	Labels         []string  `json:"labels"`
	Files          []File    `json:"files"`
	FetchedAt      time.Time `json:"fetchedAt"`
}

type stateCount struct {
	State string `json:"state"`
	Count int    `json:"count"`
}

type checkContext struct {
	Typename   string  `json:"__typename"`
	Name       string  `json:"name"`
	Conclusion *string `json:"conclusion"`
	Context    string  `json:"context"`
	State      string  `json:"state"`
}

type login struct {
	Login string `json:"login"`
}

type ghPR struct {
	Title       string  `json:"title"`
	URL         string  `json:"url"`
	State       string  `json:"state"`
	IsDraft     bool    `json:"isDraft"`
	MergedAt    *string `json:"mergedAt"`
	Author      *login  `json:"author"`
	BaseRefName string  `json:"baseRefName"`
	HeadRefName string  `json:"headRefName"`
	HeadRefOid  string  `json:"headRefOid"`
	MergeCommit *struct {
		Oid string `json:"oid"`
	} `json:"mergeCommit"`
	Additions      int     `json:"additions"`
	Deletions      int     `json:"deletions"`
	ChangedFiles   int     `json:"changedFiles"`
	ReviewDecision *string `json:"reviewDecision"`
	Labels         struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
	} `json:"labels"`
	LatestOpinionatedReviews struct {
		Nodes []struct {
			Author *login `json:"author"`
			State  string `json:"state"`
		} `json:"nodes"`
	} `json:"latestOpinionatedReviews"`
	Commits struct {
		Nodes []struct {
			Commit struct {
				StatusCheckRollup *struct {
					State    string `json:"state"`
					Contexts struct {
						CheckRunCountsByState      []stateCount   `json:"checkRunCountsByState"`
						StatusContextCountsByState []stateCount   `json:"statusContextCountsByState"`
						Nodes                      []checkContext `json:"nodes"`
					} `json:"contexts"`
				} `json:"statusCheckRollup"`
			} `json:"commit"`
		} `json:"nodes"`
	} `json:"commits"`
}

type ghFile struct {
	Filename         string `json:"filename"`
	PreviousFilename string `json:"previous_filename"`
	Status           string `json:"status"`
	Additions        int    `json:"additions"`
	Deletions        int    `json:"deletions"`
	Patch            string `json:"patch"`
}

var checkRunOutcomes = map[string]string{
	"SUCCESS": "passed", "COMPLETED": "passed",
	"FAILURE": "failed", "TIMED_OUT": "failed", "CANCELLED": "failed", "ACTION_REQUIRED": "failed", "STARTUP_FAILURE": "failed", "STALE": "failed",
	"NEUTRAL": "skipped", "SKIPPED": "skipped",
	"QUEUED": "pending", "IN_PROGRESS": "pending", "PENDING": "pending", "WAITING": "pending", "REQUESTED": "pending",
}

var statusOutcomes = map[string]string{
	"SUCCESS": "passed",
	"FAILURE": "failed", "ERROR": "failed",
	"PENDING": "pending", "EXPECTED": "pending",
}

// Cache keeps each pull request's newest read: a read younger than Floor is
// served as is, concurrent reads of one pull request share one fetch, and the
// changed files are fetched again only when the head commit or the base moves.
type Cache struct {
	Floor   time.Duration
	mu      sync.Mutex
	reads   map[string]*read
	flights singleflight.Group
}

type read struct {
	pr  *PR
	oid string
}

// Fetch reads repo#number through run, keeping the changed files patterns
// select; a closed, unmerged PR adds a commit search for a merge queue's squash.
func (c *Cache) Fetch(ctx context.Context, run Runner, repo string, number int, patterns []string, now time.Time) (*PR, error) {
	key := fmt.Sprintf("%s#%d %s", repo, number, strings.Join(patterns, "\x00"))
	c.mu.Lock()
	prior := c.reads[key]
	c.mu.Unlock()
	if prior != nil && now.Sub(prior.pr.FetchedAt) < c.Floor {
		return prior.pr, nil
	}
	fresh, err, _ := c.flights.Do(key, func() (any, error) {
		fresh, err := fetch(ctx, run, repo, number, patterns, now, prior)
		if err != nil {
			return nil, err
		}
		c.mu.Lock()
		if c.reads == nil {
			c.reads = map[string]*read{}
		}
		c.reads[key] = fresh
		c.mu.Unlock()
		return fresh, nil
	})
	if err != nil {
		return nil, err
	}
	return fresh.(*read).pr, nil
}

func fetch(ctx context.Context, run Runner, repo string, number int, patterns []string, now time.Time, prior *read) (*read, error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok {
		return nil, fmt.Errorf("repo %q is not owner/name", repo)
	}
	out, err := run(ctx, "api", "graphql", "-f", "owner="+owner, "-f", "name="+name, "-F", "number="+strconv.Itoa(number), "-f", "query="+prQuery)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data struct {
			Repository *struct {
				PullRequest *ghPR `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("decode pull request: %w", err)
	}
	if resp.Data.Repository == nil || resp.Data.Repository.PullRequest == nil {
		return nil, fmt.Errorf("%s#%d not found", repo, number)
	}
	gp := *resp.Data.Repository.PullRequest
	var files []File
	if prior != nil && prior.oid == gp.HeadRefOid && prior.pr.Base == gp.BaseRefName {
		files = prior.pr.Files
	} else {
		out, err := run(ctx, "api", "--paginate", fmt.Sprintf("repos/%s/pulls/%d/files?per_page=100", repo, number))
		if err != nil {
			return nil, err
		}
		if files, err = decodeFiles(out, patterns); err != nil {
			return nil, err
		}
	}
	checks, err := summarizeChecks(&gp)
	if err != nil {
		return nil, err
	}
	landed := ""
	if gp.State == "CLOSED" && gp.MergedAt == nil {
		if landed, err = landedSHA(ctx, run, repo, number); err != nil {
			return nil, err
		}
	}
	pr := &PR{
		Repo:           repo,
		Number:         number,
		Title:          gp.Title,
		URL:            gp.URL,
		Author:         loginOf(gp.Author),
		State:          prState(&gp, landed),
		Base:           gp.BaseRefName,
		Head:           gp.HeadRefName,
		MergedSHA:      landed,
		Additions:      gp.Additions,
		Deletions:      gp.Deletions,
		ChangedFiles:   gp.ChangedFiles,
		Checks:         checks,
		ReviewDecision: "none",
		Reviews:        []Review{},
		Labels:         []string{},
		Files:          files,
		FetchedAt:      now.UTC(),
	}
	if gp.MergeCommit != nil {
		pr.MergedSHA = gp.MergeCommit.Oid
	}
	if gp.ReviewDecision != nil {
		pr.ReviewDecision = strings.ToLower(*gp.ReviewDecision)
	}
	for _, r := range gp.LatestOpinionatedReviews.Nodes {
		pr.Reviews = append(pr.Reviews, Review{Author: loginOf(r.Author), State: strings.ToLower(r.State)})
	}
	for _, l := range gp.Labels.Nodes {
		pr.Labels = append(pr.Labels, l.Name)
	}
	return &read{pr: pr, oid: gp.HeadRefOid}, nil
}

func decodeFiles(out []byte, patterns []string) ([]File, error) {
	files := []File{}
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var page []ghFile
		err := dec.Decode(&page)
		if errors.Is(err, io.EOF) {
			return files, nil
		}
		if err != nil {
			return nil, fmt.Errorf("decode changed files: %w", err)
		}
		for _, f := range page {
			if !Selected(patterns, f.Filename) && (f.PreviousFilename == "" || !Selected(patterns, f.PreviousFilename)) {
				continue
			}
			files = append(files, File{
				Path:         f.Filename,
				PreviousPath: f.PreviousFilename,
				Status:       f.Status,
				Additions:    f.Additions,
				Deletions:    f.Deletions,
				Patch:        f.Patch,
			})
		}
	}
}

// Selected reports whether file matches one of patterns: a path.Match glob, or a
// directory whose subtree holds file. Empty patterns select every file.
func Selected(patterns []string, file string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, p := range patterns {
		if ok, _ := path.Match(p, file); ok {
			return true
		}
		if strings.HasPrefix(file, strings.TrimSuffix(p, "/")+"/") {
			return true
		}
	}
	return false
}

func summarizeChecks(gp *ghPR) (Checks, error) {
	c := Checks{State: "none"}
	if len(gp.Commits.Nodes) == 0 || gp.Commits.Nodes[0].Commit.StatusCheckRollup == nil {
		return c, nil
	}
	rollup := gp.Commits.Nodes[0].Commit.StatusCheckRollup
	c.State = strings.ToLower(rollup.State)
	if err := tally(&c, rollup.Contexts.CheckRunCountsByState, checkRunOutcomes); err != nil {
		return c, err
	}
	if err := tally(&c, rollup.Contexts.StatusContextCountsByState, statusOutcomes); err != nil {
		return c, err
	}
	for _, ctx := range rollup.Contexts.Nodes {
		if len(c.Failing) == maxFailing {
			break
		}
		switch {
		case ctx.Typename == "StatusContext" && statusOutcomes[ctx.State] == "failed":
			c.Failing = append(c.Failing, ctx.Context)
		case ctx.Typename == "CheckRun" && ctx.Conclusion != nil && checkRunOutcomes[*ctx.Conclusion] == "failed":
			c.Failing = append(c.Failing, ctx.Name)
		}
	}
	return c, nil
}

func tally(c *Checks, counts []stateCount, outcomes map[string]string) error {
	for _, sc := range counts {
		switch outcomes[sc.State] {
		case "passed":
			c.Passed += sc.Count
		case "failed":
			c.Failed += sc.Count
		case "pending":
			c.Pending += sc.Count
		case "skipped":
			c.Skipped += sc.Count
		default:
			return fmt.Errorf("unknown check state %q", sc.State)
		}
		c.Total += sc.Count
	}
	return nil
}

func landedSHA(ctx context.Context, run Runner, repo string, number int) (string, error) {
	out, err := run(ctx, "api", "-X", "GET", "search/commits", "-f", fmt.Sprintf(`q=repo:%s "(#%d)"`, repo, number), "-f", "per_page=100")
	if err != nil {
		return "", err
	}
	var resp struct {
		Items []struct {
			SHA    string `json:"sha"`
			Commit struct {
				Message string `json:"message"`
			} `json:"commit"`
		} `json:"items"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return "", fmt.Errorf("decode commit search: %w", err)
	}
	suffix := fmt.Sprintf("(#%d)", number)
	for _, it := range resp.Items {
		subject, _, _ := strings.Cut(it.Commit.Message, "\n")
		if strings.HasSuffix(strings.TrimRight(subject, " \t"), suffix) {
			return it.SHA, nil
		}
	}
	return "", nil
}

func prState(gp *ghPR, landed string) string {
	switch {
	case gp.State == "MERGED":
		return "merged"
	case gp.State == "OPEN" && gp.IsDraft:
		return "draft"
	case gp.State == "OPEN":
		return "open"
	case landed != "":
		return "landed"
	default:
		return "closed"
	}
}

func loginOf(l *login) string {
	if l == nil {
		return "ghost"
	}
	return l.Login
}

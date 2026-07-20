// Package anchor creates and resolves content-based line anchors.
package anchor

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	letters  = "abcdefghjkmnpqrstvwxyz"
	alphabet = "0123456789" + letters
)

var (
	refPattern        = regexp.MustCompile(`^(?:(\d+)(?:-(\d+))?#)?([a-hjkmnp-tv-z][0-9a-hjkmnp-tv-z]{3})$`)
	errInvalidRef     = errors.New("invalid anchor reference")
	errContentChanged = errors.New("content changed")
	errMultipleLines  = errors.New("multiple matching lines")
)

// Ref identifies a line or line range by position and start-line hash.
type Ref struct {
	Line int
	End  int
	Hash string
}

// Resolution reports the resolved line range and any movement from its hint.
type Resolution struct {
	Start int
	End   int
	Moved bool
	From  int
}

// Of returns the four-character anchor hash for line.
func Of(line string) string {
	const (
		basis = uint32(2166136261)
		prime = uint32(16777619)
	)

	h := basis
	for _, b := range []byte(strings.TrimSpace(line)) {
		h ^= uint32(b)
		h *= prime
	}
	v := h % 720896
	return string([]byte{
		letters[v>>15],
		alphabet[(v>>10)&31],
		alphabet[(v>>5)&31],
		alphabet[v&31],
	})
}

// Parse parses a bare, single-line, or ranged anchor reference.
func Parse(ref string) (Ref, error) {
	match := refPattern.FindStringSubmatch(ref)
	if match == nil {
		return Ref{}, fmt.Errorf("parse anchor %q: %w", ref, errInvalidRef)
	}

	r := Ref{Hash: match[3]}
	if match[1] == "" {
		return r, nil
	}

	line, err := strconv.Atoi(match[1])
	if err != nil {
		return Ref{}, fmt.Errorf("parse anchor %q line: %w", ref, err)
	}
	if line == 0 {
		return Ref{}, fmt.Errorf("parse anchor %q line must be positive: %w", ref, errInvalidRef)
	}
	r.Line = line
	r.End = line

	if match[2] == "" {
		return r, nil
	}
	end, err := strconv.Atoi(match[2])
	if err != nil {
		return Ref{}, fmt.Errorf("parse anchor %q range end: %w", ref, err)
	}
	if end < line {
		return Ref{}, fmt.Errorf("parse anchor %q range is reversed: %w", ref, errInvalidRef)
	}
	r.End = end
	return r, nil
}

// Format returns a single-line anchor reference.
func Format(line int, hash string) string {
	return fmt.Sprintf("%d#%s", line, hash)
}

// FormatRange returns a ranged anchor reference.
func FormatRange(start, end int, hash string) string {
	return fmt.Sprintf("%d-%d#%s", start, end, hash)
}

// Resolve locates r in lines using its hash and optional line hint.
func Resolve(r Ref, lines []string) (Resolution, error) {
	if r.Line > 0 && r.Line <= len(lines) && Of(lines[r.Line-1]) == r.Hash {
		return resolved(r, r.Line, len(lines)), nil
	}

	var candidates []int
	for i, line := range lines {
		if Of(line) == r.Hash {
			candidates = append(candidates, i+1)
		}
	}
	if len(candidates) == 0 {
		return Resolution{}, fmt.Errorf("anchor %s not found: %w", r.Hash, errContentChanged)
	}
	if r.Line == 0 {
		if len(candidates) > 1 {
			return Resolution{}, fmt.Errorf("anchor %s is ambiguous; candidates %v: %w", r.Hash, candidates, errMultipleLines)
		}
		return resolved(r, candidates[0], len(lines)), nil
	}

	nearest := candidates[0]
	nearestDistance := distance(nearest, r.Line)
	for _, candidate := range candidates[1:] {
		candidateDistance := distance(candidate, r.Line)
		if candidateDistance < nearestDistance {
			nearest = candidate
			nearestDistance = candidateDistance
		}
	}
	return resolved(r, nearest, len(lines)), nil
}

func resolved(r Ref, start, lineCount int) Resolution {
	end := start
	if r.Line > 0 {
		end = min(r.End, lineCount) + start - r.Line
		end = max(end, start)
		end = min(end, lineCount)
	}
	moved := r.Line > 0 && start != r.Line
	from := 0
	if moved {
		from = r.Line
	}
	return Resolution{Start: start, End: end, Moved: moved, From: from}
}

func distance(a, b int) int {
	if a < b {
		return b - a
	}
	return a - b
}

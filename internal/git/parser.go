package git

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrInvalidOutput indicates malformed or incomplete porcelain status output.
var ErrInvalidOutput = errors.New("unexpected git status output")

// ParseStatus parses the output of `git status --porcelain=v2 --branch`.
func ParseStatus(out []byte) (RepoInfo, error) {
	var repo RepoInfo
	var hasHead bool
	for line := range strings.SplitSeq(string(out), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "#") {
			repo.IsDirty = true
			continue
		}
		header, value, _ := strings.Cut(strings.TrimPrefix(line, "# "), " ")
		switch header {
		case "branch.oid":
			repo.Unborn = value == "(initial)"
			repo.HeadSHA = value
			if repo.Unborn {
				repo.HeadSHA = ""
			}
		case "branch.head":
			hasHead = true
			repo.Detached = value == "(detached)"
			repo.Branch = value
			if repo.Detached {
				repo.Branch = ""
			}
		case "branch.upstream":
			repo.Upstream = value
		case "branch.ab":
			ahead, behind, err := parseBranchCounts(value)
			if err != nil {
				return RepoInfo{}, err
			}
			repo.Ahead, repo.Behind = ahead, behind
		}
	}
	if !hasHead {
		return RepoInfo{}, fmt.Errorf("%w: missing branch.head header", ErrInvalidOutput)
	}
	return repo, nil
}

func parseBranchCounts(value string) (int, int, error) {
	fields := strings.Fields(value)
	if len(fields) != 2 || !strings.HasPrefix(fields[0], "+") || !strings.HasPrefix(fields[1], "-") {
		return 0, 0, fmt.Errorf("%w: malformed branch.ab header", ErrInvalidOutput)
	}
	ahead, err := strconv.ParseUint(fields[0][1:], 10, strconv.IntSize-1)
	if err != nil {
		return 0, 0, fmt.Errorf("%w: invalid ahead count: %w", ErrInvalidOutput, err)
	}
	behind, err := strconv.ParseUint(fields[1][1:], 10, strconv.IntSize-1)
	if err != nil {
		return 0, 0, fmt.Errorf("%w: invalid behind count: %w", ErrInvalidOutput, err)
	}
	return int(ahead), int(behind), nil
}

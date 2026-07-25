package e2e

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Markuysa/flightdeck/internal/api"
	"github.com/Markuysa/flightdeck/internal/core"
	"github.com/Markuysa/flightdeck/internal/derive"
	"github.com/Markuysa/flightdeck/internal/source/git"
)

// stubbedGitHubSource is this suite's "stubbed GitHub" seam (ticket 014's
// instructions): it composes the REAL git source and REAL derive.Derive,
// exactly like internal/api's own unexported gitHubSource does, but swaps a
// canned PR map for a real core.PRReader/GitHub REST call — so a test can
// annotate an in-review ticket with a PR/CI state, or approve a specific
// PR number, without any GitHub HTTP ever happening.
//
// Assembly follows ticket 005's handoff precisely: git.TicketsWithStatus for
// tickets + main status, git.Branches to find claude/NNN-* branches, and
// git.FileOnBranch + git.ParseStatus to read each branch's ticket file
// status — then derive.Derive over all of it, exactly as internal/api's
// board.go does for the real system.
type stubbedGitHubSource struct {
	mu  sync.Mutex
	prs map[string]map[string]core.PRState // project ID -> branch -> PRState
}

var _ api.ProjectSource = (*stubbedGitHubSource)(nil)

func newStubbedGitHubSource() *stubbedGitHubSource {
	return &stubbedGitHubSource{prs: map[string]map[string]core.PRState{}}
}

// setPRs records prs as projectID's canned open PRs, keyed by branch —
// exactly the shape core.PRReader.OpenPRs returns, standing in for GitHub.
func (s *stubbedGitHubSource) setPRs(projectID string, prs map[string]core.PRState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prs[projectID] = prs
}

func (s *stubbedGitHubSource) prsFor(projectID string) map[string]core.PRState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.prs[projectID]
}

// ticketBranchPattern and ticketFileOnBranch mirror internal/api/board.go's
// unexported helpers of the same name: docs/tickets/README.md's convention
// that a claude/NNN-slug branch shares its ticket file's own NNN-slug, so
// the branch name alone names the file to read on it.
var ticketBranchPattern = regexp.MustCompile(`^claude/(\d+)-`)

func ticketFileOnBranch(branch string) string {
	return "docs/tickets/" + strings.TrimPrefix(branch, "claude/") + ".md"
}

// BoardTickets implements api.ProjectSource: real git + real derive.Derive,
// stubbed PRs.
func (s *stubbedGitHubSource) BoardTickets(ctx context.Context, p core.Project) ([]core.BoardTicket, error) {
	repo := git.NewRepo(p.RepoPath)

	metas, err := repo.TicketsWithStatus(ctx)
	if err != nil {
		return nil, err
	}
	tickets := make([]core.Ticket, len(metas))
	mainStatus := make(map[int]string, len(metas))
	for i, m := range metas {
		tickets[i] = m.Ticket
		mainStatus[m.ID] = m.RawStatus
	}

	branchNames, err := repo.Branches(ctx)
	if err != nil {
		return nil, err
	}
	branches := make(map[int]derive.BranchState, len(branchNames))
	for _, name := range branchNames {
		m := ticketBranchPattern.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		id, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		content, err := repo.FileOnBranch(ctx, name, ticketFileOnBranch(name))
		if err != nil {
			continue
		}
		rawStatus, err := git.ParseStatus(content)
		if err != nil {
			continue
		}
		branches[id] = derive.BranchState{Name: name, FileStatus: rawStatus}
	}

	board := derive.Derive(tickets, mainStatus, branches, s.prsFor(p.ID))
	for i := range board {
		if board[i].Depends == nil {
			board[i].Depends = []int{}
		}
	}
	return board, nil
}

// BranchCommitTime implements api.ProjectSource by shelling to the real git
// checkout, exactly like the real gitHubSource does — no derive rule needs
// this, but GET /api/agents does (best-effort).
func (s *stubbedGitHubSource) BranchCommitTime(ctx context.Context, p core.Project, branch string) (time.Time, error) {
	return git.NewRepo(p.RepoPath).LastCommitTime(ctx, branch)
}

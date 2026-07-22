package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Markuysa/flightdeck/internal/core"
	"github.com/Markuysa/flightdeck/internal/derive"
	"github.com/Markuysa/flightdeck/internal/registry"
	"github.com/Markuysa/flightdeck/internal/source/git"
	"github.com/Markuysa/flightdeck/internal/source/github"
)

// ProjectSource computes a project's per-request derived state — status is
// never stored (ADR-001), so every call recomputes it from scratch. The
// real implementation, gitHubSource below, composes internal/source/git,
// internal/source/github and internal/derive; handler tests fake this
// interface instead of touching a real checkout or the network.
type ProjectSource interface {
	// BoardTickets returns every ticket for p with its derived status,
	// branch and PR/CI annotation — derive.Derive's output, sorted by id.
	BoardTickets(ctx context.Context, p core.Project) ([]core.BoardTicket, error)

	// BranchCommitTime returns branch's tip commit time in p's local
	// checkout. GET /api/agents is the only caller — it fills
	// AgentSession's *_at fields best-effort (ticket 008's handoff); no
	// derive rule needs this.
	BranchCommitTime(ctx context.Context, p core.Project, branch string) (time.Time, error)
}

// SecretsReader is the subset of registry.Store the board composition and
// the dispatcher factory need: a project's tokens. registry.Store satisfies
// it; nothing here ever returns a Secrets value to an HTTP response.
type SecretsReader interface {
	Secrets(ctx context.Context, projectID string) (registry.Secrets, error)
}

// gitHubSource is the real ProjectSource: internal/source/git for tickets
// and branch state, internal/source/github for PR/CI state when the
// project has a github remote, composed through derive.Derive. Nothing it
// returns is ever cached — every call re-reads git (and GitHub) from
// scratch, matching ADR-001.
type gitHubSource struct {
	secrets SecretsReader
}

// NewGitHubSource returns the real ProjectSource, sourcing GitHub tokens
// per project from secrets.
func NewGitHubSource(secrets SecretsReader) ProjectSource {
	return &gitHubSource{secrets: secrets}
}

// ticketBranchPattern matches a ticket branch's leading numeric id, e.g.
// "claude/007-dispatch-fire-autopilot-approve" -> "007".
var ticketBranchPattern = regexp.MustCompile(`^claude/(\d+)-`)

// ticketFileOnBranch derives a claude/NNN-slug branch's ticket file path
// from the branch's own name: docs/tickets' filenames and branch names
// share the same NNN-slug (docs/tickets/README.md's "Claiming is pushing
// the branch" convention), so the branch name alone names the file to
// read — no separate lookup is needed.
func ticketFileOnBranch(branch string) string {
	return "docs/tickets/" + strings.TrimPrefix(branch, "claude/") + ".md"
}

// BoardTickets implements ProjectSource for the real system: it reads
// p's tickets and branch state from git, annotates in-review tickets with
// PR/CI state from GitHub when p has a remote, and derives every status
// through derive.Derive — never storing the result.
func (s *gitHubSource) BoardTickets(ctx context.Context, p core.Project) ([]core.BoardTicket, error) {
	repo := git.NewRepo(p.RepoPath)

	metas, err := repo.TicketsWithStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("reading tickets for project %q: %w", p.ID, err)
	}
	tickets := make([]core.Ticket, len(metas))
	mainStatus := make(map[int]string, len(metas))
	for i, m := range metas {
		tickets[i] = m.Ticket
		mainStatus[m.ID] = m.RawStatus
	}

	branchNames, err := repo.Branches(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing branches for project %q: %w", p.ID, err)
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
			// The branch's ticket file could not be read on it (renamed,
			// deleted, not yet committed) — skip annotating this ticket
			// rather than failing the whole board: a per-project read
			// hiccup degrades, it never crashes the board
			// (ARCHITECTURE.md's failure policy).
			continue
		}
		rawStatus, err := git.ParseStatus(content)
		if err != nil {
			continue
		}
		branches[id] = derive.BranchState{Name: name, FileStatus: rawStatus}
	}

	prs, err := s.openPRs(ctx, p)
	if err != nil {
		return nil, err
	}

	board := derive.Derive(tickets, mainStatus, branches, prs)
	for i := range board {
		board[i] = normalizeDepends(board[i])
	}
	return board, nil
}

// openPRs returns p's open PRs keyed by branch, or nil when p has no GitHub
// remote configured, or when GitHub is unavailable — downgrading per
// ARCHITECTURE.md's failure policy so the board still renders from git
// alone, with CI shown as unknown.
func (s *gitHubSource) openPRs(ctx context.Context, p core.Project) (map[string]core.PRState, error) {
	if p.Remote != "github" || p.Owner == "" || p.Repo == "" {
		return nil, nil
	}
	sec, err := s.secrets.Secrets(ctx, p.ID)
	if err != nil && !errors.Is(err, core.ErrProjectNotFound) {
		return nil, fmt.Errorf("reading secrets for project %q: %w", p.ID, err)
	}
	client := github.New(p.Owner, p.Repo, sec.GitHubToken)
	prs, err := client.OpenPRs(ctx)
	if err != nil {
		if errors.Is(err, github.ErrGitHubUnavailable) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading open PRs for project %q: %w", p.ID, err)
	}
	return prs, nil
}

// BranchCommitTime implements ProjectSource: it shells to git for branch's
// tip commit time in p's local checkout.
func (s *gitHubSource) BranchCommitTime(ctx context.Context, p core.Project, branch string) (time.Time, error) {
	return git.NewRepo(p.RepoPath).LastCommitTime(ctx, branch)
}

// normalizeDepends ensures Ticket.Depends serializes as `[]`, never `null` —
// matching ui/src/lib/types.ts's non-nullable `depends: number[]`. A ticket
// with no dependencies parses to a nil slice (internal/source/git's
// frontmatter parser returns nil for an empty "[]" list), which is
// semantically empty but JSON-encodes as null by default.
func normalizeDepends(bt core.BoardTicket) core.BoardTicket {
	if bt.Depends == nil {
		bt.Depends = []int{}
	}
	return bt
}

// handleGetBoard implements GET /api/projects/{id}/board (US-2): the
// derived board, computed fresh from git (+ GitHub) on every request —
// never read from a store (ADR-001).
func (s *Server) handleGetBoard(w http.ResponseWriter, r *http.Request) {
	p, ok := s.projectOr404(w, r)
	if !ok {
		return
	}
	tickets, err := s.source.BoardTickets(r.Context(), p)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to compute project board")
		return
	}
	writeJSON(w, http.StatusOK, derive.Board(tickets))
}

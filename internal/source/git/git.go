// Package git implements core.TicketReader and core.GitState by shelling out
// to the git binary against a local checkout. This is the "git source" named
// in docs/ARCHITECTURE.md's package layout.
//
// Implementation choice: ARCHITECTURE.md names go-git, but CLAUDE.md permits
// shelling out ("go-git or shelling to git"). This package shells out. The
// commands it runs are exactly the ones docs/tickets/README.md documents as
// the queue's read model (git show <ref>:<path>, git branch -a, git
// merge-base --is-ancestor), so following the doc precisely was simpler and
// more auditable than mapping that model onto go-git's object-level API, and
// it avoids a sizeable dependency for three plumbing commands.
package git

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/Markuysa/flightdeck/internal/core"
)

// Repo implements core.TicketReader and core.GitState over a local checkout
// at Path. Main is the repository's default branch name; it defaults to
// "main" when empty.
type Repo struct {
	Path string
	Main string
}

var (
	_ core.GitState     = (*Repo)(nil)
	_ core.TicketReader = (*Repo)(nil)
)

// NewRepo returns a Repo reading the local checkout at path, defaulting Main
// to "main".
func NewRepo(path string) *Repo {
	return &Repo{Path: path, Main: "main"}
}

func (r *Repo) mainBranch() string {
	if r.Main != "" {
		return r.Main
	}
	return "main"
}

// run executes git with args against the repo's working directory and
// returns stdout verbatim (no trimming — callers that want file contents
// byte-for-byte, such as FileOnBranch, depend on that).
func (r *Repo) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = r.Path
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// remotes lists the repository's configured remote names (e.g. "origin").
func (r *Repo) remotes(ctx context.Context) ([]string, error) {
	out, err := r.run(ctx, "remote")
	if err != nil {
		return nil, fmt.Errorf("listing remotes: %w", err)
	}
	var remotes []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			remotes = append(remotes, line)
		}
	}
	return remotes, nil
}

// resolveRef finds a fully qualified ref for a short branch name, preferring
// a local branch (refs/heads/<branch>) and falling back to a remote-tracking
// branch (refs/remotes/<remote>/<branch>) on any configured remote. This is
// what lets IsMergedToMain and FileOnBranch work on a branch an agent pushed
// to origin but that was never checked out locally.
func (r *Repo) resolveRef(ctx context.Context, branch string) (string, error) {
	candidates := []string{"refs/heads/" + branch}
	remotes, err := r.remotes(ctx)
	if err != nil {
		return "", err
	}
	for _, remote := range remotes {
		candidates = append(candidates, "refs/remotes/"+remote+"/"+branch)
	}
	for _, ref := range candidates {
		if _, err := r.run(ctx, "rev-parse", "--verify", "--quiet", ref); err == nil {
			return ref, nil
		}
	}
	return "", fmt.Errorf("branch %q not found locally or on any remote", branch)
}

// shortBranchName reduces a fully qualified ref to the short branch name
// callers use everywhere else, stripping the leading refs/heads/ or the
// refs/remotes/<remote>/ prefix. It returns "" for refs that are neither
// (e.g. refs/tags/*).
func shortBranchName(ref string) string {
	switch {
	case strings.HasPrefix(ref, "refs/heads/"):
		return strings.TrimPrefix(ref, "refs/heads/")
	case strings.HasPrefix(ref, "refs/remotes/"):
		rest := strings.TrimPrefix(ref, "refs/remotes/")
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) == 2 {
			return parts[1]
		}
		return ""
	default:
		return ""
	}
}

// Branches lists the repository's branches by short name (e.g.
// "claude/003-git-source"), local and remote-tracking, deduplicated when a
// branch exists both locally and on a remote. The default branch (Main) and
// symbolic refs (e.g. origin/HEAD) are excluded: callers use Branches to
// find ticket branches, not main itself.
//
// Remote-tracking branches are included deliberately — the derive engine
// must see a ticket branch an agent pushed from a cloud session even when
// this checkout never fetched it into a local branch, and a plain `git
// branch` (local-only) would miss it.
func (r *Repo) Branches(ctx context.Context) ([]string, error) {
	out, err := r.run(ctx, "for-each-ref", "--format=%(refname)", "refs/heads", "refs/remotes")
	if err != nil {
		return nil, fmt.Errorf("listing branches: %w", err)
	}
	main := r.mainBranch()
	seen := make(map[string]bool)
	var branches []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name := shortBranchName(line)
		if name == "" || name == main || name == "HEAD" || seen[name] {
			continue
		}
		seen[name] = true
		branches = append(branches, name)
	}
	sort.Strings(branches)
	return branches, nil
}

// IsMergedToMain reports whether branch's history is an ancestor of the
// default branch (Main) — i.e. whether merging branch happened already.
func (r *Repo) IsMergedToMain(ctx context.Context, branch string) (bool, error) {
	branchRef, err := r.resolveRef(ctx, branch)
	if err != nil {
		return false, err
	}
	mainRef, err := r.resolveRef(ctx, r.mainBranch())
	if err != nil {
		return false, fmt.Errorf("resolving main branch %q: %w", r.mainBranch(), err)
	}
	_, err = r.run(ctx, "merge-base", "--is-ancestor", branchRef, mainRef)
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		// exit 1 from --is-ancestor means "not an ancestor", not a failure.
		return false, nil
	}
	return false, fmt.Errorf("checking whether %q is merged to %q: %w", branch, r.mainBranch(), err)
}

// FileOnBranch returns path's contents as they exist on branch — not on
// whatever ref the local working tree happens to have checked out.
func (r *Repo) FileOnBranch(ctx context.Context, branch, path string) (string, error) {
	ref, err := r.resolveRef(ctx, branch)
	if err != nil {
		return "", err
	}
	out, err := r.run(ctx, "show", ref+":"+path)
	if err != nil {
		return "", fmt.Errorf("reading %q on branch %q: %w", path, branch, err)
	}
	return out, nil
}

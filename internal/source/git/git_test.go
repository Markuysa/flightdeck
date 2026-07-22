package git

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestBranchesListsCreatedBranches(t *testing.T) {
	t.Parallel()
	repo := NewFixtureRepo(t,
		[]FixtureTicket{{ID: 1, Title: "One", Role: "backend", Status: "todo"}},
		[]FixtureBranch{
			{Name: "claude/001-in-progress"},
			{Name: "claude/002-merged", Merged: true},
		},
	)

	branches, err := repo.Branches(context.Background())
	if err != nil {
		t.Fatalf("Branches: %v", err)
	}

	for _, want := range []string{"claude/001-in-progress", "claude/002-merged"} {
		if !slices.Contains(branches, want) {
			t.Errorf("Branches() = %v, want it to contain %q", branches, want)
		}
	}
	if slices.Contains(branches, "main") {
		t.Errorf("Branches() = %v, want it to exclude the default branch", branches)
	}
}

func TestBranchesIncludesRemoteOnlyBranch(t *testing.T) {
	t.Parallel()
	repo := NewFixtureRepo(t,
		[]FixtureTicket{{ID: 1, Title: "One", Role: "backend", Status: "todo"}},
		nil,
	)

	const remoteOnly = "claude/099-remote-only"
	remoteDir := t.TempDir()
	runFixtureGit(t, remoteDir, "init", "-q", "--bare")
	runFixtureGit(t, repo.Path, "remote", "add", "origin", remoteDir)
	runFixtureGit(t, repo.Path, "push", "-q", "origin", "main")
	runFixtureGit(t, repo.Path, "checkout", "-q", "-b", remoteOnly)
	// A commit that only exists on this branch, so it is genuinely unmerged
	// (a branch identical to main's tip is trivially "merged" already).
	writeFixtureTicket(t, repo.Path, FixtureTicket{ID: 1, Title: "One", Role: "backend", Status: "todo", Body: "remote-only work"})
	runFixtureGit(t, repo.Path, "commit", "-q", "-am", "fixture: remote-only work")
	runFixtureGit(t, repo.Path, "push", "-q", "origin", remoteOnly)
	runFixtureGit(t, repo.Path, "checkout", "-q", "main")
	runFixtureGit(t, repo.Path, "branch", "-q", "-D", remoteOnly)
	runFixtureGit(t, repo.Path, "fetch", "-q", "origin")

	branches, err := repo.Branches(context.Background())
	if err != nil {
		t.Fatalf("Branches: %v", err)
	}
	if !slices.Contains(branches, remoteOnly) {
		t.Fatalf("Branches() = %v, want it to contain remote-only branch %q", branches, remoteOnly)
	}

	merged, err := repo.IsMergedToMain(context.Background(), remoteOnly)
	if err != nil {
		t.Fatalf("IsMergedToMain(%q): %v", remoteOnly, err)
	}
	if merged {
		t.Errorf("IsMergedToMain(%q) = true, want false (never merged)", remoteOnly)
	}
}

func TestIsMergedToMain(t *testing.T) {
	t.Parallel()
	repo := NewFixtureRepo(t,
		[]FixtureTicket{{ID: 1, Title: "One", Role: "backend", Status: "todo"}},
		[]FixtureBranch{
			{
				Name:    "claude/001-unmerged",
				Tickets: []FixtureTicket{{ID: 1, Title: "One", Role: "backend", Status: "todo", Body: "still working"}},
			},
			{
				Name:    "claude/002-merged",
				Tickets: []FixtureTicket{{ID: 1, Title: "One", Role: "backend", Status: "done"}},
				Merged:  true,
			},
		},
	)
	ctx := context.Background()

	merged, err := repo.IsMergedToMain(ctx, "claude/002-merged")
	if err != nil {
		t.Fatalf("IsMergedToMain(merged): %v", err)
	}
	if !merged {
		t.Errorf("IsMergedToMain(merged) = false, want true")
	}

	unmerged, err := repo.IsMergedToMain(ctx, "claude/001-unmerged")
	if err != nil {
		t.Fatalf("IsMergedToMain(unmerged): %v", err)
	}
	if unmerged {
		t.Errorf("IsMergedToMain(unmerged) = true, want false")
	}
}

func TestIsMergedToMainUnknownBranch(t *testing.T) {
	t.Parallel()
	repo := NewFixtureRepo(t, []FixtureTicket{{ID: 1, Title: "One", Role: "backend", Status: "todo"}}, nil)

	if _, err := repo.IsMergedToMain(context.Background(), "claude/does-not-exist"); err == nil {
		t.Fatal("IsMergedToMain(unknown branch) = nil error, want an error")
	}
}

func TestFileOnBranchReturnsBranchVersionNotMains(t *testing.T) {
	t.Parallel()
	repo := NewFixtureRepo(t,
		[]FixtureTicket{{ID: 1, Title: "One", Role: "backend", Status: "todo", Body: "main body"}},
		[]FixtureBranch{
			{
				Name:    "claude/001-diverged",
				Tickets: []FixtureTicket{{ID: 1, Title: "One", Role: "backend", Status: "done", Body: "branch body"}},
			},
		},
	)
	ctx := context.Background()
	path := "docs/tickets/001-fixture.md"

	onMain, err := repo.FileOnBranch(ctx, "main", path)
	if err != nil {
		t.Fatalf("FileOnBranch(main): %v", err)
	}
	onBranch, err := repo.FileOnBranch(ctx, "claude/001-diverged", path)
	if err != nil {
		t.Fatalf("FileOnBranch(branch): %v", err)
	}

	if onMain == onBranch {
		t.Fatalf("expected main and branch contents to differ, both were:\n%s", onMain)
	}
	if !strings.Contains(onMain, "status: todo") || !strings.Contains(onMain, "main body") {
		t.Errorf("main content = %q, want it to contain the main ticket's status and body", onMain)
	}
	if !strings.Contains(onBranch, "status: done") || !strings.Contains(onBranch, "branch body") {
		t.Errorf("branch content = %q, want it to contain the branch ticket's status and body", onBranch)
	}
}

func TestLastCommitTimeReturnsTipCommitTime(t *testing.T) {
	t.Parallel()
	before := time.Now().Add(-time.Minute)
	repo := NewFixtureRepo(t,
		[]FixtureTicket{{ID: 1, Title: "One", Role: "backend", Status: "todo"}},
		[]FixtureBranch{
			{
				Name:    "claude/001-in-progress",
				Tickets: []FixtureTicket{{ID: 1, Title: "One", Role: "backend", Status: "todo", Body: "in progress"}},
			},
		},
	)

	got, err := repo.LastCommitTime(context.Background(), "claude/001-in-progress")
	if err != nil {
		t.Fatalf("LastCommitTime: %v", err)
	}
	if got.Before(before) {
		t.Errorf("LastCommitTime() = %v, want a time within the last minute", got)
	}
}

func TestLastCommitTimeUnknownBranchIsAnError(t *testing.T) {
	t.Parallel()
	repo := NewFixtureRepo(t, []FixtureTicket{{ID: 1, Title: "One", Role: "backend", Status: "todo"}}, nil)

	if _, err := repo.LastCommitTime(context.Background(), "claude/does-not-exist"); err == nil {
		t.Fatal("LastCommitTime(unknown branch) error = nil, want an error")
	}
}

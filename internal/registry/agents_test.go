package registry

import (
	"context"
	"errors"
	"testing"

	"github.com/Markuysa/flightdeck/internal/core"
)

func agent(id, role string) core.Agent {
	return core.Agent{
		ID: id, ProjectID: "acme", Name: id, Role: role,
		Prompt: "Follow the repo conventions.",
		Skills: []string{"go", "sqlite"},
	}
}

func TestAgentRoundTrips(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	ctx := context.Background()

	if err := store.SetAgent(ctx, agent("backend-specialist", "backend")); err != nil {
		t.Fatalf("SetAgent: %v", err)
	}
	got, err := store.AgentForRole(ctx, "acme", "backend")
	if err != nil {
		t.Fatalf("AgentForRole: %v", err)
	}
	if got.Prompt == "" {
		t.Error("prompt did not round-trip")
	}
	if len(got.Skills) != 2 || got.Skills[0] != "go" {
		t.Errorf("skills = %v, want [go sqlite]", got.Skills)
	}
}

// TestOneAgentPerRole: dispatch looks an agent up BY role, so two rows sharing
// one would make which prompt an agent receives depend on row order.
func TestOneAgentPerRole(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	ctx := context.Background()

	if err := store.SetAgent(ctx, agent("first", "backend")); err != nil {
		t.Fatalf("SetAgent: %v", err)
	}
	second := agent("second", "backend")
	second.Prompt = "Replaced."
	if err := store.SetAgent(ctx, second); err != nil {
		t.Fatalf("replacing the backend agent: %v", err)
	}

	agents, err := store.Agents(ctx, "acme")
	if err != nil {
		t.Fatalf("Agents: %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("project has %d agents for one role, want 1: %+v", len(agents), agents)
	}
	if agents[0].Prompt != "Replaced." {
		t.Errorf("prompt = %q, want the replacement to win", agents[0].Prompt)
	}
}

func TestAgentForUnstaffedRole(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	_, err := store.AgentForRole(context.Background(), "acme", "qa")
	if !errors.Is(err, ErrAgentNotFound) {
		t.Errorf("AgentForRole for an unstaffed role = %v, want ErrAgentNotFound", err)
	}
}

// TestSkillsCannotSmuggleASeparator: skills are stored newline-delimited, so a
// skill containing a newline would decode as two.
func TestSkillsCannotSmuggleASeparator(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	ctx := context.Background()

	a := agent("sneaky", "backend")
	a.Skills = []string{"go\nsecretly-two"}
	if err := store.SetAgent(ctx, a); err != nil {
		t.Fatalf("SetAgent: %v", err)
	}
	got, err := store.AgentForRole(ctx, "acme", "backend")
	if err != nil {
		t.Fatalf("AgentForRole: %v", err)
	}
	if len(got.Skills) != 1 {
		t.Errorf("skills = %v, want one entry — a newline must not split a skill", got.Skills)
	}
}

func TestRemoveProjectDropsItsAgents(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	ctx := context.Background()

	if err := store.Add(ctx, testProject("acme")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := store.SetAgent(ctx, agent("backend-specialist", "backend")); err != nil {
		t.Fatalf("SetAgent: %v", err)
	}
	if err := store.Remove(ctx, "acme"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	agents, err := store.Agents(ctx, "acme")
	if err != nil {
		t.Fatalf("Agents: %v", err)
	}
	if len(agents) != 0 {
		t.Errorf("agents survived their project's removal: %+v", agents)
	}
}

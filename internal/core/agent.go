package core

// Agent is a configured specialist: which role of ticket it takes, what
// instructions it works under, and which skills it may draw on.
//
// FlightDeck does not run the agent — a routine does. What this type changes is
// what the routine is *told*: dispatching ticket N no longer sends only
// {ticket_id: N}, it sends the prompt and skills configured for that ticket's
// role. That is the difference between "every ticket gets the same generic
// agent" and a team with specialists.
//
// An unmatched role is not an error. A project with no agent for `qa` still
// dispatches its qa tickets — they simply carry no extra instructions, exactly
// as before this type existed.
type Agent struct {
	ID   string `json:"id"`   // stable slug, generated from Name
	Name string `json:"name"` // human-readable, e.g. "Backend specialist"
	// Role binds this agent to tickets whose `role:` frontmatter matches.
	// One agent per role per project; registering a second replaces the first.
	Role string `json:"role"`
	// Prompt is the system prompt the routine runs this agent under: how it
	// should work, what it must not do, what the project's conventions are.
	Prompt string `json:"prompt"`
	// Skills names capabilities the routine should make available — the
	// operator's vocabulary, passed through verbatim rather than interpreted
	// here. FlightDeck deliberately does not validate these against a known
	// list: which skills exist is the routine's business, and a hardcoded list
	// here would go stale the moment the routine gained one.
	Skills []string `json:"skills"`
	// ProjectID scopes the agent. Agents are per-project because prompts are:
	// "follow the repo's Go conventions" means nothing across two repos.
	ProjectID string `json:"project_id"`
}

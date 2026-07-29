package plan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/Markuysa/flightdeck/internal/core"
)

// DefaultModel is the model used for decomposition. Planning is the one place
// in FlightDeck where quality compounds: every ticket it emits becomes work an
// agent actually performs, so a cheap plan is a false economy — a bad edge in
// the dependency graph costs more agent time than the planning call ever saves.
const DefaultModel = anthropic.ModelClaudeOpus5

// maxPlanTokens bounds one planning response. A large decomposition with bodies
// and acceptance criteria runs long, and truncation mid-JSON fails the whole
// call, so this is generous rather than tight.
const maxPlanTokens = 16000

// ErrPlanFailed wraps any failure producing a plan: no API key, a transport or
// API error, or a response that does not survive validation.
var ErrPlanFailed = errors.New("plan: decomposition failed")

// ErrNoAPIKey is returned when no Anthropic API key is configured. It is its
// own sentinel so the API layer can answer 501 ("planning is not configured on
// this server") rather than 502 — a setup gap, not an upstream failure.
var ErrNoAPIKey = errors.New("plan: no Anthropic API key configured")

// ClaudePlanner implements Planner against the Claude API.
type ClaudePlanner struct {
	client anthropic.Client
	model  anthropic.Model
	// maxTickets caps how many tickets one plan may contain. The model is told
	// the limit and validation enforces it: an unbounded plan is an unbounded
	// amount of agent work approved by one click.
	maxTickets int
	// hasKey records whether a usable credential was supplied. Tracked
	// separately from client because WithBaseURL builds a test client with a
	// placeholder key, and because an empty key must fail at Propose (with
	// ErrNoAPIKey) rather than at construction — a server with planning
	// unconfigured still serves every other route.
	hasKey bool
}

var _ Planner = (*ClaudePlanner)(nil)

// Option configures a ClaudePlanner.
type Option func(*ClaudePlanner)

// WithModel overrides the planning model.
func WithModel(m anthropic.Model) Option {
	return func(p *ClaudePlanner) { p.model = m }
}

// WithMaxTickets overrides the per-plan ticket cap.
func WithMaxTickets(n int) Option {
	return func(p *ClaudePlanner) {
		if n > 0 {
			p.maxTickets = n
		}
	}
}

// WithBaseURL points the client at a different API root — used by tests to
// drive a local httptest server instead of calling Anthropic.
func WithBaseURL(url string) Option {
	return func(p *ClaudePlanner) {
		p.client = anthropic.NewClient(option.WithBaseURL(url), option.WithAPIKey("test-key"))
		p.hasKey = true
	}
}

// NewClaudePlanner returns a Planner backed by the Claude API. apiKey is a
// constructor parameter only — never read from a global here — and an empty one
// is not an error at construction: it surfaces as ErrNoAPIKey on the first
// Propose, so a server with no planning configured still starts and serves
// every other route.
func NewClaudePlanner(apiKey string, opts ...Option) *ClaudePlanner {
	p := &ClaudePlanner{model: DefaultModel, maxTickets: DefaultMaxTickets}
	if apiKey != "" {
		p.client = anthropic.NewClient(option.WithAPIKey(apiKey))
	}
	p.hasKey = apiKey != ""
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// planSchema is the JSON schema the response is constrained to. Structured
// outputs guarantee the shape, which is why decodePlan can unmarshal without
// defensive per-field checks — but shape is not sanity, so validate.go still
// runs on the result (see the package doc comment).
func (p *ClaudePlanner) planSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"summary": map[string]any{
				"type":        "string",
				"description": "One paragraph on the approach taken and why the work splits this way.",
			},
			"tickets": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"number": map[string]any{
							"type":        "integer",
							"description": "1-based position in the plan. Dependencies reference this.",
						},
						"title": map[string]any{"type": "string"},
						"role": map[string]any{
							"type": "string",
							"enum": Roles,
						},
						"depends": map[string]any{
							"type":        "array",
							"items":       map[string]any{"type": "integer"},
							"description": "Numbers of tickets that must merge before this one starts. Must be lower than this ticket's own number.",
						},
						"body": map[string]any{
							"type":        "string",
							"description": "Markdown: what to build and why. Enough for an agent with no other context.",
						},
						"acceptance": map[string]any{
							"type":        "array",
							"items":       map[string]any{"type": "string"},
							"description": "Verifiable criteria that make this ticket done.",
						},
						"handoff": map[string]any{
							"type":        "string",
							"description": "What a dependent ticket needs to know from this one. Empty if nothing.",
						},
					},
					"required":             []string{"number", "title", "role", "depends", "body", "acceptance", "handoff"},
					"additionalProperties": false,
				},
			},
		},
		"required":             []string{"summary", "tickets"},
		"additionalProperties": false,
	}
}

// systemPrompt is the planner's instructions. It states the constraints that
// the derive engine and the scheduler impose downstream — a plan that ignores
// them produces a queue that stalls, so they belong in the prompt rather than
// only in validation.
func (p *ClaudePlanner) systemPrompt() string {
	return fmt.Sprintf(`You decompose a software goal into a queue of tickets that autonomous coding agents implement one at a time.

How the queue works, and why it constrains you:
- Each ticket is implemented by one agent on its own branch, then opens a pull request.
- A ticket becomes startable only when every ticket it depends on has merged.
- Tickets with no dependency between them may run AT THE SAME TIME, in parallel.

Rules:
1. Order tickets so a ticket only ever depends on lower-numbered tickets. Number from 1.
2. No cycles. A dependency you cannot justify is a dependency you should remove — a false edge serializes work that could have run in parallel.
3. Tickets that run in parallel must not need to edit the same files. This is the constraint that matters most: two agents editing one file produces a merge conflict no one is watching for. When two pieces of work touch the same code, make one depend on the other instead of letting them race.
4. Size a ticket as one agent's focused session — a coherent unit with a clear boundary, not an afterthought and not an epic.
5. Front-load the shared foundation (types, interfaces, schema) as early tickets that later work depends on. That is what makes the rest parallelizable.
6. Acceptance criteria must be verifiable by running something — a test, a command, an observable behaviour. "Works correctly" is not a criterion.
7. Write the body for an agent that has this ticket and the repository, and nothing else: no memory of this conversation.
8. At most %d tickets. If the goal is bigger, cover the most valuable coherent slice and say what you left out in the summary.
9. Roles: %s. Pick the one that matches the work.`,
		p.maxTickets, strings.Join(Roles, ", "))
}

// userPrompt states the goal plus the queue's current contents, so a plan
// extends the project rather than duplicating work already queued.
func userPrompt(goal string, existing []core.Ticket) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Goal:\n%s\n", goal)
	if len(existing) > 0 {
		b.WriteString("\nTickets already in this project's queue — do not duplicate these, and depend on them only by describing the dependency in prose (you cannot reference them by number):\n")
		for _, t := range existing {
			fmt.Fprintf(&b, "- #%d [%s] %s\n", t.ID, t.Role, t.Title)
		}
	}
	return b.String()
}

// Propose implements Planner.
func (p *ClaudePlanner) Propose(ctx context.Context, goal string, existing []core.Ticket) (Plan, error) {
	if strings.TrimSpace(goal) == "" {
		return Plan{}, fmt.Errorf("%w: goal is empty", ErrPlanFailed)
	}
	if !p.hasKey {
		return Plan{}, ErrNoAPIKey
	}

	resp, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     p.model,
		MaxTokens: maxPlanTokens,
		System:    []anthropic.TextBlockParam{{Text: p.systemPrompt()}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt(goal, existing))),
		},
		OutputConfig: anthropic.OutputConfigParam{
			Effort: anthropic.OutputConfigEffortHigh,
			Format: anthropic.JSONOutputFormatParam{Schema: p.planSchema()},
		},
	})
	if err != nil {
		// The SDK's error carries status and request id but never the API key.
		return Plan{}, fmt.Errorf("%w: %w", ErrPlanFailed, err)
	}

	// A safety refusal is a successful HTTP response with no usable content.
	// Reporting it as a transport failure would send the operator to the wrong
	// place entirely, so it gets its own message.
	if resp.StopReason == anthropic.StopReasonRefusal {
		return Plan{}, fmt.Errorf("%w: the model declined to plan this goal", ErrPlanFailed)
	}

	raw, err := firstText(resp)
	if err != nil {
		return Plan{}, err
	}

	plan, err := decodePlan(raw)
	if err != nil {
		return Plan{}, err
	}
	plan.Goal = goal
	plan.Model = string(p.model)
	plan.CreatedAt = time.Now().UTC()

	if err := Validate(plan, p.maxTickets); err != nil {
		return Plan{}, err
	}
	return plan, nil
}

// firstText returns the response's text content.
func firstText(resp *anthropic.Message) (string, error) {
	for _, block := range resp.Content {
		if block.Type == "text" {
			return block.Text, nil
		}
	}
	return "", fmt.Errorf("%w: response carried no text content (stop reason: %s)", ErrPlanFailed, resp.StopReason)
}

// decodePlan unmarshals the model's JSON into a Plan.
func decodePlan(raw string) (Plan, error) {
	var body struct {
		Summary string   `json:"summary"`
		Tickets []Ticket `json:"tickets"`
	}
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		return Plan{}, fmt.Errorf("%w: decoding response: %w", ErrPlanFailed, err)
	}
	return Plan{Summary: body.Summary, Tickets: body.Tickets}, nil
}

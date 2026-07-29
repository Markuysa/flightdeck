package app

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Markuysa/flightdeck/internal/api"
	"github.com/Markuysa/flightdeck/internal/core"
	"github.com/Markuysa/flightdeck/internal/registry"
)

// defaultRefreshInterval is FLIGHTDECK_REFRESH_INTERVAL's default: frequent
// enough that a merge or CI flip shows up on the Board/Agents screens within
// a few seconds, without hammering git (and, when configured, GitHub) on
// every tick.
const defaultRefreshInterval = 5 * time.Second

// parseRefreshInterval turns FLIGHTDECK_REFRESH_INTERVAL's raw value into a
// poll interval:
//   - unset/empty -> defaultRefreshInterval. Every other FLIGHTDECK_* var in
//     this package treats "" as "use the default" (Addr, DBPath above); the
//     refresher follows the same convention deliberately, because os.Getenv
//     cannot tell "unset" from "set to empty" apart, and a refresher that is
//     silently off by default would quietly reopen the exact gap ticket 018
//     closes for anyone who doesn't know to configure it.
//   - "off" (case-insensitive) or a duration that parses to <= 0 (e.g. "0",
//     "0s") -> 0, meaning "disabled" — the caller must not start Run.
//   - anything else is parsed with time.ParseDuration; a value that fails to
//     parse is a configuration error, returned to the caller (ConfigFromEnv
//     fails fast on it, same as an invalid token).
func parseRefreshInterval(raw string) (time.Duration, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultRefreshInterval, nil
	}
	if strings.EqualFold(trimmed, "off") {
		return 0, nil
	}
	d, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("invalid FLIGHTDECK_REFRESH_INTERVAL %q: %w", raw, err)
	}
	if d <= 0 {
		return 0, nil
	}
	return d, nil
}

// boardFingerprint is a project's board reduced to two comparable strings so
// consecutive polls can be diffed without keeping the full board around.
//
//   - structural covers everything except CI: each ticket's id, derived
//     status, branch and (when in review) PR number/URL. Any difference here
//     means the board itself changed shape — a status flip, a branch
//     appearing, a PR opening or closing.
//   - ci covers only each ticket's PR CI state (pending/green/red/unknown).
//
// Precedence (documented once, applied in (*Refresher).pollOnce): a
// structural difference always publishes board.changed, even if the CI
// hash also moved in the same tick — board.changed is the broader event and
// a subscriber refetching the board picks up the CI state too, so there is
// no need to also fire ci.changed. ci.changed fires only when structural is
// unchanged and ci alone moved: a pure CI flip with nothing else different.
// This keeps exactly one event per tick per project — never both, never a
// storm.
type boardFingerprint struct {
	structural string
	ci         string
}

// fingerprintBoard builds tickets' boardFingerprint. tickets is expected
// sorted by id already (api.ProjectSource.BoardTickets's documented
// contract), so no re-sort is needed here.
func fingerprintBoard(tickets []core.BoardTicket) boardFingerprint {
	var structural, ci strings.Builder
	for _, t := range tickets {
		var prNumber int
		var prURL, prCI string
		if t.PR != nil {
			prNumber, prURL, prCI = t.PR.Number, t.PR.URL, t.PR.CI
		}
		fmt.Fprintf(&structural, "%d:%s:%s:%d:%s|", t.ID, t.Status, t.Branch, prNumber, prURL)
		fmt.Fprintf(&ci, "%d:%s|", t.ID, prCI)
	}
	return boardFingerprint{structural: structural.String(), ci: ci.String()}
}

// Refresher polls every registered project's derived board on an interval
// and publishes board.changed/ci.changed to the same *api.Broker the SSE
// server streams from, when a poll's fingerprint differs from the previous
// one. It never dispatches, merges, or mutates anything: it only observes
// derive's output (exactly what GET /api/projects/{id}/board would return)
// and notifies subscribers that they should refetch.
//
// Starting work autonomously is internal/schedule's job, not this one's, and
// it is off by default (ADR-007). Keeping the two loops separate is what lets
// an operator run the live board with no autonomous dispatch at all — the
// refresher costs nothing but a git read, while the scheduler spends agent
// budget.
type Refresher struct {
	broker   *api.Broker
	source   api.ProjectSource
	projects *registry.Store
	interval time.Duration

	// baselines holds the last poll's fingerprint per project id. It is
	// only ever touched from pollOnce, which Run calls from a single
	// goroutine (and tests call directly, also single-goroutine), so no
	// lock is needed.
	baselines map[string]boardFingerprint
}

// NewRefresher returns a Refresher publishing to broker, deriving each
// project's board through source, listing registered projects from
// projects, and polling every interval when run. interval <= 0 disables it
// (Run returns immediately without polling).
func NewRefresher(broker *api.Broker, source api.ProjectSource, projects *registry.Store, interval time.Duration) *Refresher {
	return &Refresher{
		broker:    broker,
		source:    source,
		projects:  projects,
		interval:  interval,
		baselines: make(map[string]boardFingerprint),
	}
}

// pollOnce lists every registered project, re-derives its board, and
// publishes board.changed/ci.changed for any project whose fingerprint moved
// since the previous call. The very first observation of a project only
// records a baseline — publishing on a poll that has nothing to compare
// against would fire an event for every project on startup, not an actual
// change. A project's BoardTickets error is logged (the wrapped error names
// only the project id and a git-level cause, never a secret — see
// api.gitHubSource.BoardTickets/openPRs) and skipped for this tick; it never
// stops the loop or drops the project's baseline, so a transient failure
// just retries next tick. A project removed from the registry has its
// baseline forgotten, so re-adding it later starts fresh rather than
// diffing against stale state.
func (r *Refresher) pollOnce(ctx context.Context) {
	projects, err := r.projects.List(ctx)
	if err != nil {
		log.Printf("flightdeck: refresh: listing projects: %v", err)
		return
	}

	seen := make(map[string]struct{}, len(projects))
	for _, p := range projects {
		seen[p.ID] = struct{}{}

		tickets, err := r.source.BoardTickets(ctx, p)
		if err != nil {
			log.Printf("flightdeck: refresh: project %q: %v", p.ID, err)
			continue
		}

		fp := fingerprintBoard(tickets)
		prev, known := r.baselines[p.ID]
		r.baselines[p.ID] = fp
		if !known {
			continue
		}

		switch {
		case fp.structural != prev.structural:
			r.broker.Publish(api.EventBoardChanged, map[string]any{"project_id": p.ID})
		case fp.ci != prev.ci:
			r.broker.Publish(api.EventCIChanged, map[string]any{"project_id": p.ID})
		}
	}

	for id := range r.baselines {
		if _, ok := seen[id]; !ok {
			delete(r.baselines, id)
		}
	}
}

// Run polls every r.interval until ctx is canceled, then returns. It does
// nothing (returns immediately) when r.interval <= 0 — the disabled
// configuration parseRefreshInterval produces for "off"/"0"/empty overrides.
// Callers run this in its own goroutine alongside the server and rely on ctx
// cancellation (the same context the server's graceful shutdown uses) to
// stop it — no separate lifecycle to manage, no goroutine leak.
func (r *Refresher) Run(ctx context.Context) {
	if r.interval <= 0 {
		return
	}
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.pollOnce(ctx)
		}
	}
}

package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Markuysa/flightdeck/internal/core"
)

// TicketMeta is one parsed ticket file: the core.Ticket plus the raw
// `status:` frontmatter value. core.Ticket deliberately has no Status field
// (ADR-001) — callers that need the two literals the derive engine is
// allowed to read (done, needs-attention) read RawStatus here instead of
// gaining a field on the shared domain type.
type TicketMeta struct {
	core.Ticket
	RawStatus string
}

// Tickets implements core.TicketReader: every docs/tickets/[0-9]*.md file
// under Path, parsed and ordered by ticket id. It reads the filesystem
// directly (the "disk ticket source"), not a specific git ref — callers
// that need a ticket's state on a particular branch use FileOnBranch.
func (r *Repo) Tickets(ctx context.Context) ([]core.Ticket, error) {
	metas, err := r.TicketsWithStatus(ctx)
	if err != nil {
		return nil, err
	}
	tickets := make([]core.Ticket, len(metas))
	for i, m := range metas {
		tickets[i] = m.Ticket
	}
	return tickets, nil
}

// TicketsWithStatus parses every docs/tickets/[0-9]*.md file under Path and
// returns each ticket alongside its raw status literal, ordered by ticket
// id ascending.
//
// Malformed frontmatter fails the whole call with an error naming the
// offending file, rather than skipping it: a ticket that silently
// disappeared from the board because its file didn't parse would be worse
// than a loud, actionable error.
func (r *Repo) TicketsWithStatus(ctx context.Context) ([]TicketMeta, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	pattern := filepath.Join(r.Path, "docs", "tickets", "[0-9]*.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("globbing ticket files: %w", err)
	}
	metas := make([]TicketMeta, 0, len(matches))
	for _, path := range matches {
		meta, err := parseTicketFile(path)
		if err != nil {
			return nil, err
		}
		metas = append(metas, meta)
	}
	sort.Slice(metas, func(i, j int) bool { return metas[i].ID < metas[j].ID })
	return metas, nil
}

func parseTicketFile(path string) (TicketMeta, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // path comes from filepath.Glob over a fixed pattern, not user input
	if err != nil {
		return TicketMeta{}, fmt.Errorf("reading ticket file %s: %w", path, err)
	}
	frontmatter, body, err := splitFrontmatter(string(raw))
	if err != nil {
		return TicketMeta{}, fmt.Errorf("parsing frontmatter in %s: %w", path, err)
	}
	ticket, rawStatus, err := parseFrontmatter(frontmatter)
	if err != nil {
		return TicketMeta{}, fmt.Errorf("parsing frontmatter in %s: %w", path, err)
	}
	ticket.Body = strings.TrimSpace(body)
	ticket.Handoff = extractHandoff(body)
	return TicketMeta{Ticket: ticket, RawStatus: rawStatus}, nil
}

// splitFrontmatter separates a ticket file into its "---" delimited
// frontmatter block and the markdown body that follows it.
func splitFrontmatter(content string) (frontmatter, body string, err error) {
	lines := strings.Split(content, "\n")
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i >= len(lines) || strings.TrimSpace(lines[i]) != "---" {
		return "", "", errors.New("missing opening --- frontmatter fence")
	}
	start := i + 1
	end := -1
	for j := start; j < len(lines); j++ {
		if strings.TrimSpace(lines[j]) == "---" {
			end = j
			break
		}
	}
	if end == -1 {
		return "", "", errors.New("missing closing --- frontmatter fence")
	}
	frontmatter = strings.Join(lines[start:end], "\n")
	body = strings.Join(lines[end+1:], "\n")
	return frontmatter, body, nil
}

// parseFrontmatter parses the key: value lines between the frontmatter
// fences into a core.Ticket plus the raw status literal. id, title, role,
// and status are required; depends defaults to nil (no dependencies) when
// absent.
func parseFrontmatter(frontmatter string) (core.Ticket, string, error) {
	var ticket core.Ticket
	var rawStatus string
	seen := make(map[string]bool)
	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return core.Ticket{}, "", fmt.Errorf("invalid frontmatter line %q: expected \"key: value\"", line)
		}
		key = strings.TrimSpace(key)
		value = stripInlineComment(strings.TrimSpace(value))

		switch key {
		case "id":
			id, err := strconv.Atoi(value)
			if err != nil {
				return core.Ticket{}, "", fmt.Errorf("invalid id %q: %w", value, err)
			}
			ticket.ID = id
		case "title":
			ticket.Title = value
		case "role":
			ticket.Role = value
		case "depends":
			depends, err := parseDepends(value)
			if err != nil {
				return core.Ticket{}, "", fmt.Errorf("invalid depends %q: %w", value, err)
			}
			ticket.Depends = depends
		case "status":
			rawStatus = value
		}
		seen[key] = true
	}
	for _, required := range []string{"id", "title", "role", "status"} {
		if !seen[required] {
			return core.Ticket{}, "", fmt.Errorf("missing required frontmatter field %q", required)
		}
	}
	return ticket, rawStatus, nil
}

// stripInlineComment removes a trailing " # comment" from a frontmatter
// value, e.g. "role: dev   # which agent implements it".
func stripInlineComment(value string) string {
	if idx := strings.Index(value, " #"); idx != -1 {
		return strings.TrimSpace(value[:idx])
	}
	return value
}

// parseDepends parses a frontmatter list literal such as "[]", "[1]", or
// "[5, 6, 7]" into ticket ids.
func parseDepends(value string) ([]int, error) {
	if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
		return nil, fmt.Errorf("expected a [...] list, got %q", value)
	}
	inner := strings.TrimSpace(value[1 : len(value)-1])
	if inner == "" {
		return nil, nil
	}
	parts := strings.Split(inner, ",")
	depends := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		id, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("invalid dependency id %q: %w", p, err)
		}
		depends = append(depends, id)
	}
	return depends, nil
}

// extractHandoff returns the content of the "## Handoff" section of body —
// the text between that heading and the next level-2 heading or end of
// file — or "" when no such section exists.
func extractHandoff(body string) string {
	lines := strings.Split(body, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "## Handoff" {
			start = i + 1
			break
		}
	}
	if start == -1 {
		return ""
	}
	end := len(lines)
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			end = i
			break
		}
	}
	return strings.TrimSpace(strings.Join(lines[start:end], "\n"))
}

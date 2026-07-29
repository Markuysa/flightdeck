package plan

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// ErrApplyFailed wraps any failure writing a plan's tickets to a repository.
var ErrApplyFailed = errors.New("plan: apply failed")

// ticketFilePattern matches an existing ticket filename, capturing its id.
var ticketFilePattern = regexp.MustCompile(`^(\d+)-`)

// slugPattern reduces a title to a filename-safe slug.
var slugPattern = regexp.MustCompile(`[^a-z0-9]+`)

// Applied is the outcome of writing a plan: the files created and the ticket
// ids they were assigned.
type Applied struct {
	// Files are the paths written, relative to the repository root.
	Files []string `json:"files"`
	// IDs are the ticket ids assigned, parallel to Files.
	IDs []int `json:"ids"`
}

// Apply writes p's tickets into repoPath's docs/tickets/ directory and returns
// what it created.
//
// Two things this deliberately does NOT do:
//
//   - It does not commit. The derive engine reads the working tree for a
//     ticket's status on main, so writing the files is what makes them visible;
//     committing and pushing is the operator's call, and doing it here would
//     mean a planning API that force-pushes to a branch nobody asked about.
//   - It does not renumber or touch existing tickets. New ids continue from the
//     highest id already present, so applying a plan can never rewrite work
//     that is already queued or in flight.
//
// Ids are assigned in plan order, and each ticket's `depends` is translated
// from plan-local Numbers to the real assigned ids — which is why the plan's
// Number field exists at all.
func Apply(repoPath string, p Plan) (Applied, error) {
	if err := Validate(p, DefaultMaxTickets); err != nil {
		return Applied{}, err
	}

	dir := filepath.Join(repoPath, "docs", "tickets")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Applied{}, fmt.Errorf("%w: creating %s: %w", ErrApplyFailed, dir, err)
	}

	nextID, err := nextTicketID(dir)
	if err != nil {
		return Applied{}, err
	}

	// Assign ids in plan order. The plan is already topologically ordered
	// (validation rejects cycles, and the prompt asks for lower-numbered
	// dependencies), so a ticket's dependencies always have their real id
	// assigned before it needs them.
	ordered := append([]Ticket(nil), p.Tickets...)
	slices.SortStableFunc(ordered, func(a, b Ticket) int { return a.Number - b.Number })

	realID := make(map[int]int, len(ordered))
	for i, t := range ordered {
		realID[t.Number] = nextID + i
	}

	var out Applied
	for _, t := range ordered {
		id := realID[t.Number]
		name := fmt.Sprintf("%03d-%s.md", id, slugify(t.Title))
		path := filepath.Join(dir, name)

		if _, err := os.Stat(path); err == nil {
			return out, fmt.Errorf("%w: %s already exists", ErrApplyFailed, name)
		}

		deps := make([]int, 0, len(t.Depends))
		for _, dep := range t.Depends {
			deps = append(deps, realID[dep])
		}
		slices.Sort(deps)

		if err := os.WriteFile(path, []byte(renderTicket(id, t, deps)), 0o644); err != nil {
			return out, fmt.Errorf("%w: writing %s: %w", ErrApplyFailed, name, err)
		}
		out.Files = append(out.Files, filepath.Join("docs", "tickets", name))
		out.IDs = append(out.IDs, id)
	}
	return out, nil
}

// nextTicketID returns one past the highest ticket id already in dir, or 1 for
// an empty queue. Continuing the existing numbering (rather than starting from
// 1) is what keeps a second plan from colliding with the first.
func nextTicketID(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, fmt.Errorf("%w: reading %s: %w", ErrApplyFailed, dir, err)
	}
	highest := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		m := ticketFilePattern.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		if id, err := strconv.Atoi(m[1]); err == nil && id > highest {
			highest = id
		}
	}
	return highest + 1, nil
}

// renderTicket produces the ticket file's contents in exactly the format
// internal/source/git parses: a `---` fenced frontmatter block carrying id,
// title, role, status and depends, then the markdown body. `status: todo` is
// the only status a new ticket may have — everything else is derived.
func renderTicket(id int, t Ticket, deps []int) string {
	var b strings.Builder

	depList := make([]string, len(deps))
	for i, d := range deps {
		depList[i] = strconv.Itoa(d)
	}

	b.WriteString("---\n")
	fmt.Fprintf(&b, "id: %d\n", id)
	fmt.Fprintf(&b, "title: %s\n", singleLine(t.Title))
	fmt.Fprintf(&b, "role: %s\n", t.Role)
	b.WriteString("status: todo\n")
	fmt.Fprintf(&b, "depends: [%s]\n", strings.Join(depList, ", "))
	b.WriteString("---\n\n")

	fmt.Fprintf(&b, "# %s\n\n", singleLine(t.Title))

	if body := strings.TrimSpace(t.Body); body != "" {
		b.WriteString(body)
		b.WriteString("\n\n")
	}

	if len(t.Acceptance) > 0 {
		b.WriteString("## Acceptance criteria\n\n")
		for _, c := range t.Acceptance {
			if c = strings.TrimSpace(c); c != "" {
				fmt.Fprintf(&b, "- [ ] %s\n", singleLine(c))
			}
		}
		b.WriteString("\n")
	}

	if handoff := strings.TrimSpace(t.Handoff); handoff != "" {
		b.WriteString("## Handoff\n\n")
		b.WriteString(handoff)
		b.WriteString("\n")
	}

	return b.String()
}

// singleLine collapses newlines so a value cannot break out of a frontmatter
// line or a list item. The title comes from model output, and a newline in it
// would produce a file the frontmatter parser rejects — taking the whole board
// down, since a malformed ticket fails the entire read.
func singleLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// slugify reduces a title to a filename-safe slug, bounded so a long title
// cannot produce a path the filesystem rejects.
func slugify(title string) string {
	slug := slugPattern.ReplaceAllString(strings.ToLower(title), "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "ticket"
	}
	if len(slug) > 60 {
		slug = strings.Trim(slug[:60], "-")
	}
	return slug
}

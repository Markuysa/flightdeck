package git

// ParseStatus returns the raw `status:` frontmatter literal carried by a
// ticket file's full content — frontmatter fences included, exactly what
// FileOnBranch returns for a path on a given branch. It reuses the same
// frontmatter parsing TicketsWithStatus applies to files on disk, so a
// status read off a branch and a status read off main agree on what counts
// as valid frontmatter and what "status" means.
//
// Exported for ticket 008 (internal/api): parseFrontmatter is otherwise
// private to this package, but deriving a ticket's BranchState requires a
// raw status for arbitrary branch file content, not just the working
// checkout TicketsWithStatus reads (ticket 005's handoff names this
// addition explicitly).
func ParseStatus(fileContent string) (string, error) {
	frontmatter, _, err := splitFrontmatter(fileContent)
	if err != nil {
		return "", err
	}
	_, rawStatus, err := parseFrontmatter(frontmatter)
	if err != nil {
		return "", err
	}
	return rawStatus, nil
}

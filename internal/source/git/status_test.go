package git

import "testing"

func TestParseStatusReadsRawStatusLiteral(t *testing.T) {
	t.Parallel()
	content := "---\nid: 3\ntitle: Example\nrole: backend\ndepends: [1]\nstatus: needs-attention\n---\nBody.\n"

	got, err := ParseStatus(content)
	if err != nil {
		t.Fatalf("ParseStatus: %v", err)
	}
	if got != "needs-attention" {
		t.Errorf("ParseStatus() = %q, want %q", got, "needs-attention")
	}
}

func TestParseStatusMalformedFrontmatterIsAnError(t *testing.T) {
	t.Parallel()
	if _, err := ParseStatus("no frontmatter fences here"); err == nil {
		t.Fatal("ParseStatus(malformed) error = nil, want an error")
	}
}

func TestParseStatusMissingStatusFieldIsAnError(t *testing.T) {
	t.Parallel()
	content := "---\nid: 3\ntitle: Example\nrole: backend\n---\nBody.\n"
	if _, err := ParseStatus(content); err == nil {
		t.Fatal("ParseStatus(no status field) error = nil, want an error")
	}
}

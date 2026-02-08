package createfixture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureSourceFileHasNoPlan_FailsWhenPlanExists(t *testing.T) {
	repo := t.TempDir()
	mkdirAll(t, filepath.Join(repo, ".rafa", "plans", "abc123-demo"))
	source := filepath.Join(repo, "docs", "designs", "feature.md")
	mkdirAll(t, filepath.Dir(source))
	writeFile(t, source, "# Feature")

	planJSON := map[string]any{
		"id":         "abc123",
		"name":       "demo",
		"sourceFile": "docs/designs/feature.md",
	}
	writeJSON(t, filepath.Join(repo, ".rafa", "plans", "abc123-demo", "plan.json"), planJSON)

	err := EnsureSourceFileHasNoPlan(repo, source)
	if err == nil {
		t.Fatalf("expected error when plan already exists for source")
	}
}

func TestEnsureSourceFileHasNoPlan_SucceedsWhenNoPlanExists(t *testing.T) {
	repo := t.TempDir()
	source := filepath.Join(repo, "docs", "designs", "new-feature.md")
	mkdirAll(t, filepath.Dir(source))
	writeFile(t, source, "# New Feature")

	mkdirAll(t, filepath.Join(repo, ".rafa", "plans", "abc123-existing"))
	planJSON := map[string]any{
		"id":         "abc123",
		"name":       "existing",
		"sourceFile": "docs/designs/other-feature.md",
	}
	writeJSON(t, filepath.Join(repo, ".rafa", "plans", "abc123-existing", "plan.json"), planJSON)

	if err := EnsureSourceFileHasNoPlan(repo, source); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
}

func TestNormalizeSourceFile(t *testing.T) {
	repo := t.TempDir()
	source := filepath.Join(repo, "docs", "designs", "test.md")
	mkdirAll(t, filepath.Dir(source))
	writeFile(t, source, "# Test")

	got, err := NormalizeSourceFile(repo, source)
	if err != nil {
		t.Fatalf("NormalizeSourceFile() error = %v", err)
	}
	if got != "docs/designs/test.md" {
		t.Fatalf("NormalizeSourceFile() = %q, want %q", got, "docs/designs/test.md")
	}
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func writeJSON(t *testing.T, path string, data any) {
	t.Helper()
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	writeFile(t, path, string(b))
}

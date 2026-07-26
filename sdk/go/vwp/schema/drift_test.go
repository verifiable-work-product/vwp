package schema

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sourceSchemasDir is the repository's canonical schema tree, relative to
// this package directory.
const sourceSchemasDir = "../../../../schemas/v0.1"

// TestEmbeddedSchemasMatchRepository asserts that every *.json schema under
// the repository's schemas/v0.1 tree is embedded byte-for-byte, and that the
// embedded copy carries nothing the repository does not. go:embed cannot
// cross the module boundary, so the SDK ships a copy; this test turns schema
// drift into a test failure instead of a silently stale SDK.
//
// When it fails, refresh the embedded copy with:
//
//	cd sdk/go && make sync-schemas
func TestEmbeddedSchemasMatchRepository(t *testing.T) {
	if _, err := os.Stat(sourceSchemasDir); err != nil {
		t.Fatalf("repository schema tree not found at %s (this test must run from a repo checkout): %v", sourceSchemasDir, err)
	}

	// Repository -> embedded: every source schema is embedded and identical.
	sourceFiles := map[string]bool{}
	err := filepath.WalkDir(sourceSchemasDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
			return err
		}
		rel, err := filepath.Rel(sourceSchemasDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		sourceFiles[rel] = true

		want, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		got, err := schemaFS.ReadFile("vwpschemas/v0.1/" + rel)
		if err != nil {
			t.Errorf("schema %s exists in schemas/v0.1 but is not embedded — run `cd sdk/go && make sync-schemas`", rel)
			return nil
		}
		if !bytes.Equal(want, got) {
			t.Errorf("embedded schema %s differs from schemas/v0.1 — run `cd sdk/go && make sync-schemas`", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sourceFiles) == 0 {
		t.Fatal("no schemas found in the repository tree; refusing to pass vacuously")
	}

	// Embedded -> repository: no stale embedded schemas.
	err = fs.WalkDir(schemaFS, "vwpschemas/v0.1", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
			return err
		}
		rel := strings.TrimPrefix(path, "vwpschemas/v0.1/")
		if !sourceFiles[rel] {
			t.Errorf("embedded schema %s no longer exists in schemas/v0.1 — run `cd sdk/go && make sync-schemas`", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

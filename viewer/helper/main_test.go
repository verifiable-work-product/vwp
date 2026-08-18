package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/builder"
)

func TestInspectEmbeddedPrimaryArtifact(t *testing.T) {
	packageDir := t.TempDir()
	b, err := builder.Scaffold(packageDir, builder.ScaffoldConfig{
		PackageId: "viewer-test",
		CreatedAt: "2026-08-06T12:00:00Z",
		Profile:   "vwp-core",
		Title:     "NYC Revenue Allocation",
		Workproduct: vwp.Workproduct{
			Id: "wp-viewer-test", Title: "NYC Revenue Allocation", Domain: "tax", Status: "draft",
		},
		Provenance: vwp.Provenance{
			CreatedAt: "2026-08-06T12:00:00Z",
			CreatedBy: vwp.ProvenanceAgent{Type: "system", Name: "viewer-test"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	artifact := []byte("region,revenue\nNew York City,125000\n")
	if err := b.AddPrimaryArtifact("artifacts/nyc-revenue-allocation.csv", "text/csv", artifact); err != nil {
		t.Fatal(err)
	}
	packagePath := filepath.Join(t.TempDir(), "allocation.vwp")
	if err := vwp.Pack(packageDir, packagePath); err != nil {
		t.Fatal(err)
	}

	doc, err := inspectPackage(packagePath, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if doc.Title != "NYC Revenue Allocation" || doc.PrimaryArtifact == nil {
		t.Fatalf("document = %#v", doc)
	}
	if doc.PrimaryArtifact.Role != rolePrimaryArtifact || doc.PrimaryArtifact.Content != string(artifact) {
		t.Fatalf("primary artifact = %#v", doc.PrimaryArtifact)
	}
	if doc.Readable == nil || doc.Readable.Path != doc.PrimaryArtifact.Path {
		t.Fatalf("v0.1 primary-only readable fallback = %#v", doc.Readable)
	}
	extracted, err := os.ReadFile(doc.PrimaryArtifact.ExtractedPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(extracted) != string(artifact) {
		t.Fatalf("extracted artifact = %q", extracted)
	}
	if !doc.Verification.Valid {
		t.Fatalf("package should verify: %#v", doc.Verification.Results)
	}
}

func TestInspectFallsBackToRendering(t *testing.T) {
	packagePath := filepath.Join("..", "..", "examples", "asc606-revenue-contract", "acme-asc606.vwp")
	doc, err := inspectPackage(packagePath, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if doc.PrimaryArtifact != nil {
		t.Fatalf("rendering must not be labeled as a primary artifact: %#v", doc.PrimaryArtifact)
	}
	if doc.Readable == nil || doc.Readable.Role != roleRendering {
		t.Fatalf("fallback readable = %#v", doc.Readable)
	}
	if doc.Readable.MediaType != "text/markdown" || doc.Readable.Content == "" {
		t.Fatalf("fallback rendering = %#v", doc.Readable)
	}
	if len(doc.Facts) == 0 || len(doc.Claims) == 0 || len(doc.Calculations) == 0 {
		t.Fatalf("structured sections missing: facts=%d claims=%d calculations=%d", len(doc.Facts), len(doc.Claims), len(doc.Calculations))
	}
}

func TestInspectKeepsCanonicalReadableSeparateFromPrimaryArtifact(t *testing.T) {
	packageDir := t.TempDir()
	b, err := builder.Scaffold(packageDir, builder.ScaffoldConfig{
		PackageId: "viewer-readable-test",
		CreatedAt: "2026-08-06T12:00:00Z",
		Profile:   "vwp-core",
		Workproduct: vwp.Workproduct{
			Id: "wp-viewer-readable-test", Title: "Readable Test", Domain: "tax", Status: "draft",
		},
		Provenance: vwp.Provenance{
			CreatedAt: "2026-08-06T12:00:00Z",
			CreatedBy: vwp.ProvenanceAgent{Type: "system", Name: "viewer-test"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := b.AddPrimaryArtifact("artifacts/allocation.csv", "text/csv", []byte("region,revenue\nNYC,125000\n")); err != nil {
		t.Fatal(err)
	}
	readable := []byte("# NYC revenue allocation\n\nThe allocation is 25%.\n")
	if err := b.AddFilePart("renderings/allocation.md", "text/markdown", builder.RoleReadable, readable); err != nil {
		t.Fatal(err)
	}
	packagePath := filepath.Join(t.TempDir(), "readable.vwp")
	if err := vwp.Pack(packageDir, packagePath); err != nil {
		t.Fatal(err)
	}

	doc, err := inspectPackage(packagePath, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if doc.Readable == nil || doc.Readable.Role != roleReadable || doc.Readable.Content != string(readable) {
		t.Fatalf("canonical readable = %#v", doc.Readable)
	}
	if doc.PrimaryArtifact == nil || doc.PrimaryArtifact.Role != rolePrimaryArtifact {
		t.Fatalf("primary artifact = %#v", doc.PrimaryArtifact)
	}
	if doc.Readable.ExtractedPath == doc.PrimaryArtifact.ExtractedPath {
		t.Fatalf("readable and artifact extraction paths collided: %q", doc.Readable.ExtractedPath)
	}
}

func TestSafePreviewName(t *testing.T) {
	if got := safePreviewName("revenue analysis.XLSX", "primary"); got != "primary.xlsx" {
		t.Fatalf("safePreviewName = %q", got)
	}
	if got := safePreviewName("report.<script>", "readable"); got != "readable" {
		t.Fatalf("unsafe extension survived: %q", got)
	}
}

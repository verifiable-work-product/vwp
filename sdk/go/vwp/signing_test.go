package vwp_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/builder"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/schema"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/validators"
)

func testKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func signFixture(t *testing.T, dir string, priv ed25519.PrivateKey) *vwp.Signature {
	t.Helper()
	b, err := builder.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, vwp.ManifestPath))
	if err != nil {
		t.Fatal(err)
	}
	sig, err := vwp.SignManifest(priv, vwp.SignatureSigner{Id: "bitwave", Name: "Bitwave"}, raw, time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if err := b.SetSignature(*sig); err != nil {
		t.Fatal(err)
	}
	return sig
}

// A signed package verifies under the published key, and the record it
// carries is valid against the spec's signature schema.
func TestSignedPackageVerifies(t *testing.T) {
	pub, priv := testKey(t)
	dir := copyFixture(t, "examples", "asc606-revenue-contract", "package")
	sig := signFixture(t, dir, priv)

	if sig.Signer.KeyId != vwp.Ed25519KeyId(pub) || sig.Algorithm != vwp.SignatureAlgorithmEdDSA || sig.Scope != vwp.SignatureScopeManifest {
		t.Fatalf("record = %+v", sig)
	}
	if errs, err := schema.Validate(schema.Signature, sig); err != nil || len(errs) > 0 {
		t.Fatalf("record does not satisfy the signature schema: %v %v", errs, err)
	}

	pkg, err := vwp.OpenPackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	// The record is declared in manifest.signatures with its hash, so the
	// part-hash validator covers it too.
	manifest, _ := vwp.ParseManifest(mustRead(t, pkg, vwp.ManifestPath))
	if len(manifest.Signatures) != 1 || manifest.Signatures[0].Path != vwp.SignaturePath(sig.Signer.KeyId) || manifest.Signatures[0].Role != builder.RoleSignature {
		t.Fatalf("manifest.signatures = %+v", manifest.Signatures)
	}

	trusted := map[string]ed25519.PublicKey{sig.Signer.KeyId: pub}
	report := validators.VerifyWith(pkg, vwp.WithTrustedSigningKeys(trusted))
	if !report.Valid {
		t.Fatalf("report invalid: %+v", report.Results)
	}
	if r := mustResult(t, report, vwp.ValidatorSignaturesPresent); r.Status != vwp.StatusPass {
		t.Fatalf("present = %+v", r)
	}
	if r := mustResult(t, report, vwp.ValidatorSignaturesValid); r.Status != vwp.StatusPass {
		t.Fatalf("valid = %+v", r)
	}

	// Without trusted keys the verifier only reports presence.
	plain := validators.Verify(pkg)
	if _, ok := plain.Result(vwp.ValidatorSignaturesValid); ok {
		t.Fatal("no trusted keys: core.signatures.valid must not be reported")
	}
}

// Changing any part after signing leaves the record stale: still a real
// signature, but over an earlier manifest. A forged or wrong-key value fails.
func TestSignatureGoesStaleWhenAPartChangesAndFailsUnderTheWrongKey(t *testing.T) {
	pub, priv := testKey(t)
	dir := copyFixture(t, "examples", "asc606-revenue-contract", "package")
	sig := signFixture(t, dir, priv)
	keyId := sig.Signer.KeyId

	b, err := builder.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.AddFact(vwp.Fact{Id: "f-late", Statement: "Added after signing.", EvidenceRefs: []vwp.EvidenceRef{{EvidenceId: "ev-msa"}}}); err != nil {
		t.Fatal(err)
	}
	pkg, err := vwp.OpenPackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	report := validators.VerifyWith(pkg, vwp.WithTrustedSigningKeys(map[string]ed25519.PublicKey{keyId: pub}))
	if r := mustResult(t, report, vwp.ValidatorSignaturesValid); r.Status != vwp.StatusWarning {
		t.Fatalf("a signature over an earlier manifest must be a stale warning, got %+v", r)
	}
	if err := sig.Verify(pub, mustRead(t, pkg, vwp.ManifestPath)); !errors.Is(err, vwp.ErrSignatureStale) {
		t.Fatalf("err = %v, want stale", err)
	}

	// Re-signing replaces the record rather than stacking a second one.
	signFixture(t, dir, priv)
	pkg, _ = vwp.OpenPackage(dir)
	manifest, _ := vwp.ParseManifest(mustRead(t, pkg, vwp.ManifestPath))
	if len(manifest.Signatures) != 1 {
		t.Fatalf("re-signing must replace the record, got %d", len(manifest.Signatures))
	}
	report = validators.VerifyWith(pkg, vwp.WithTrustedSigningKeys(map[string]ed25519.PublicKey{keyId: pub}))
	if r := mustResult(t, report, vwp.ValidatorSignaturesValid); r.Status != vwp.StatusPass {
		t.Fatalf("re-signed = %+v", r)
	}

	// The wrong key under the same id: invalid, and the package fails.
	other, _ := testKey(t)
	report = validators.VerifyWith(pkg, vwp.WithTrustedSigningKeys(map[string]ed25519.PublicKey{keyId: other}))
	if r := mustResult(t, report, vwp.ValidatorSignaturesValid); r.Status != vwp.StatusFail {
		t.Fatalf("wrong key = %+v", r)
	}
	if report.Valid {
		t.Fatal("a package whose signature does not verify must not be valid")
	}

	// A key the verifier does not know is reported, not failed.
	report = validators.VerifyWith(pkg, vwp.WithTrustedSigningKeys(map[string]ed25519.PublicKey{"ed25519-someone-else": other}))
	if r := mustResult(t, report, vwp.ValidatorSignaturesValid); r.Status != vwp.StatusWarning {
		t.Fatalf("unknown key = %+v", r)
	}
}

// The unsigned fixture keeps the v0.1 behaviour: a warning, never a failure.
func TestUnsignedPackageOnlyWarns(t *testing.T) {
	report := validators.VerifyWith(openFixture(t, "examples", "asc606-revenue-contract", "package"), vwp.WithTrustedSigningKeys(map[string]ed25519.PublicKey{"k": make([]byte, 32)}))
	if r := mustResult(t, report, vwp.ValidatorSignaturesPresent); r.Status != vwp.StatusWarning {
		t.Fatalf("present = %+v", r)
	}
	if _, ok := report.Result(vwp.ValidatorSignaturesValid); ok {
		t.Fatal("nothing to validate without a record")
	}
}

func mustRead(t *testing.T, pkg vwp.Source, path string) []byte {
	t.Helper()
	data, err := pkg.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

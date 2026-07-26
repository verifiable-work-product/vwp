package vwp_test

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/validators"
)

// buildZip creates an in-memory ZIP with the given name -> content entries,
// in order. archive/zip does not sanitize entry names, so traversal names
// can be crafted directly.
func buildZip(t *testing.T, entries [][2]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		w, err := zw.Create(e[0])
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(e[1])); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestZipPathTraversalRejected(t *testing.T) {
	data := buildZip(t, [][2]string{
		{"manifest.json", "{}"},
		{"../evil.txt", "nope"},
		{"/abs.txt", "nope"},
	})
	pkg, err := vwp.OpenPackageBytes(data, "evil.vwp")
	if err != nil {
		t.Fatal(err)
	}
	unsafe := pkg.UnsafeEntries()
	if len(unsafe) != 2 || unsafe[0] != "../evil.txt" || unsafe[1] != "/abs.txt" {
		t.Fatalf("UnsafeEntries = %v", unsafe)
	}
	if pkg.Has("../evil.txt") {
		t.Error("unsafe entry must not be readable")
	}
	report := validators.Verify(pkg)
	if report.Valid {
		t.Error("package with unsafe entries must be invalid")
	}
	res, _ := report.Result(vwp.ValidatorPackageSafePaths)
	if res.Status != vwp.StatusFail {
		t.Errorf("safePaths = %s", res.Status)
	}

	// Unpack must refuse the archive outright.
	zipPath := filepath.Join(t.TempDir(), "evil.vwp")
	if err := os.WriteFile(zipPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	outDir := t.TempDir()
	if err := vwp.Unpack(zipPath, outDir); err == nil || !strings.Contains(err.Error(), "unsafe entry paths") {
		t.Errorf("Unpack error = %v, want unsafe-entry refusal", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "manifest.json")); !os.IsNotExist(err) {
		t.Error("Unpack must not write anything when refusing an unsafe archive")
	}
}

func TestZipDecompressionLimits(t *testing.T) {
	t.Run("entry count", func(t *testing.T) {
		data := buildZip(t, [][2]string{
			{"a.txt", "a"}, {"b.txt", "b"}, {"c.txt", "c"},
		})
		_, err := vwp.OpenPackageBytes(data, "many.vwp", vwp.WithLimits(vwp.Limits{MaxEntries: 2}))
		if err == nil || !strings.Contains(err.Error(), "entry limit") {
			t.Errorf("err = %v, want entry limit", err)
		}
	})
	t.Run("total bytes", func(t *testing.T) {
		big := strings.Repeat("0", 4096)
		data := buildZip(t, [][2]string{{"a.txt", big}, {"b.txt", big}})
		_, err := vwp.OpenPackageBytes(data, "bomb.vwp", vwp.WithLimits(vwp.Limits{MaxTotalBytes: 6000}))
		if err == nil || !strings.Contains(err.Error(), "decompression limits") {
			t.Errorf("err = %v, want decompression limit", err)
		}
	})
	t.Run("entry bytes", func(t *testing.T) {
		data := buildZip(t, [][2]string{{"a.txt", strings.Repeat("0", 4096)}})
		_, err := vwp.OpenPackageBytes(data, "bomb.vwp", vwp.WithLimits(vwp.Limits{MaxEntryBytes: 1024}))
		if err == nil || !strings.Contains(err.Error(), "decompression limits") {
			t.Errorf("err = %v, want decompression limit", err)
		}
	})
	t.Run("within limits", func(t *testing.T) {
		data := buildZip(t, [][2]string{{"a.txt", "hello"}})
		pkg, err := vwp.OpenPackageBytes(data, "ok.vwp", vwp.WithLimits(vwp.Limits{MaxEntries: 5, MaxTotalBytes: 100, MaxEntryBytes: 100}))
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		got, err := pkg.Read("a.txt")
		if err != nil || string(got) != "hello" {
			t.Errorf("Read = %q, %v", got, err)
		}
	})
}

func TestPackUnpackRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	vwpPath := filepath.Join(tmp, "out", "acme.vwp")
	if err := vwp.Pack(fixturePath("examples", "asc606-revenue-contract", "package"), vwpPath); err != nil {
		t.Fatalf("Pack: %v", err)
	}

	zipped, err := vwp.OpenPackage(vwpPath)
	if err != nil {
		t.Fatal(err)
	}
	if report := validators.Verify(zipped); !report.Valid {
		t.Fatalf("packed archive should verify; statuses: %v", statuses(report))
	}

	unpackDir := filepath.Join(tmp, "unpacked")
	if err := vwp.Unpack(vwpPath, unpackDir); err != nil {
		t.Fatalf("Unpack: %v", err)
	}
	unpacked, err := vwp.OpenPackage(unpackDir)
	if err != nil {
		t.Fatal(err)
	}
	if report := validators.Verify(unpacked); !report.Valid {
		t.Fatalf("unpacked dir should verify; statuses: %v", statuses(report))
	}
	// Same file inventory both ways.
	if a, b := strings.Join(zipped.List(), ","), strings.Join(unpacked.List(), ","); a != b {
		t.Errorf("file lists differ:\nzip: %s\ndir: %s", a, b)
	}
}

func TestPackWithWriteHashes(t *testing.T) {
	dir := copyFixture(t, "testdata", "invalid", "bad-hash")
	vwpPath := filepath.Join(t.TempDir(), "fixed.vwp")
	if err := vwp.Pack(dir, vwpPath, vwp.WithWriteHashes()); err != nil {
		t.Fatalf("Pack: %v", err)
	}
	pkg, err := vwp.OpenPackage(vwpPath)
	if err != nil {
		t.Fatal(err)
	}
	report := validators.Verify(pkg)
	res, _ := report.Result(vwp.ValidatorPartHashesMatch)
	if res.Status != vwp.StatusPass {
		t.Errorf("partHashes after WithWriteHashes = %s (%s), want pass", res.Status, res.Message)
	}
}

func TestPackRequiresManifest(t *testing.T) {
	if err := vwp.Pack(t.TempDir(), filepath.Join(t.TempDir(), "x.vwp")); err == nil {
		t.Error("Pack without manifest.json should fail")
	}
}

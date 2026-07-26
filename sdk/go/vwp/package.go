package vwp

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Source is a readable VWP package, backed by either an unpacked directory
// or a .vwp ZIP archive. All paths are forward-slash paths relative to the
// package root.
type Source interface {
	// Origin describes where the package was loaded from (for display).
	Origin() string
	// List returns the relative paths of all safe files in the package,
	// sorted lexicographically.
	List() []string
	// Has reports whether the package contains the given relative path.
	Has(relPath string) bool
	// Read returns the bytes of the given relative path.
	Read(relPath string) ([]byte, error)
	// UnsafeEntries returns entry names that were rejected as unsafe
	// (path traversal, absolute paths, backslashes).
	UnsafeEntries() []string
}

// Limits bounds resource use when reading untrusted ZIP packages, mitigating
// decompression bombs. Zero values mean "use the default".
type Limits struct {
	// MaxEntries caps the number of file entries in the archive.
	MaxEntries int
	// MaxTotalBytes caps the total uncompressed size of all entries.
	MaxTotalBytes int64
	// MaxEntryBytes caps the uncompressed size of any single entry.
	MaxEntryBytes int64
}

// DefaultLimits are the limits applied when none are supplied.
func DefaultLimits() Limits {
	return Limits{
		MaxEntries:    10_000,
		MaxTotalBytes: 1 << 30,   // 1 GiB
		MaxEntryBytes: 512 << 20, // 512 MiB
	}
}

func (l Limits) withDefaults() Limits {
	d := DefaultLimits()
	if l.MaxEntries <= 0 {
		l.MaxEntries = d.MaxEntries
	}
	if l.MaxTotalBytes <= 0 {
		l.MaxTotalBytes = d.MaxTotalBytes
	}
	if l.MaxEntryBytes <= 0 {
		l.MaxEntryBytes = d.MaxEntryBytes
	}
	return l
}

// OpenOption configures OpenPackage / OpenPackageBytes / Unpack.
type OpenOption func(*openConfig)

type openConfig struct {
	limits Limits
}

// WithLimits overrides the decompression limits applied to ZIP packages.
func WithLimits(l Limits) OpenOption {
	return func(c *openConfig) { c.limits = l }
}

var driveLetterRe = regexp.MustCompile(`^[A-Za-z]:`)

// IsUnsafePath reports whether a package-relative path must be rejected: it
// is empty, absolute, contains a drive letter, a backslash, a NUL byte, or
// any ".." segment.
func IsUnsafePath(relPath string) bool {
	if relPath == "" {
		return true
	}
	if strings.ContainsAny(relPath, "\\\x00") {
		return true
	}
	if strings.HasPrefix(relPath, "/") || driveLetterRe.MatchString(relPath) {
		return true
	}
	for _, seg := range strings.Split(relPath, "/") {
		if seg == ".." {
			return true
		}
	}
	return false
}

// OpenPackage opens a VWP package from a .vwp file or an unpacked package
// directory.
func OpenPackage(path string, opts ...OpenOption) (Source, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("vwp: open package: %w", err)
	}
	if info.IsDir() {
		return openDir(path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("vwp: open package: %w", err)
	}
	return OpenPackageBytes(data, path, opts...)
}

// OpenPackageBytes opens a VWP package from an in-memory .vwp archive (e.g.
// an upload that should never touch disk). Unsafe entry paths are rejected
// exactly as for file-based packages.
func OpenPackageBytes(data []byte, origin string, opts ...OpenOption) (Source, error) {
	cfg := openConfig{limits: DefaultLimits()}
	for _, o := range opts {
		o(&cfg)
	}
	if origin == "" {
		origin = "(in-memory package)"
	}
	return openZip(data, origin, cfg.limits.withDefaults())
}

// ReadJSON reads and decodes a JSON part from the package into out.
// It returns fs.ErrNotExist (wrapped) when the part is absent.
func ReadJSON(pkg Source, relPath string, out any) error {
	if !pkg.Has(relPath) {
		return fmt.Errorf("vwp: %s: %w", relPath, fs.ErrNotExist)
	}
	data, err := pkg.Read(relPath)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("vwp: %s: %w", relPath, err)
	}
	return nil
}

type dirSource struct {
	origin  string
	rootDir string
	files   []string
	fileSet map[string]struct{}
}

func openDir(rootDir string) (*dirSource, error) {
	src := &dirSource{origin: rootDir, rootDir: rootDir, fileSet: map[string]struct{}{}}
	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() == ".DS_Store" || !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		src.files = append(src.files, rel)
		src.fileSet[rel] = struct{}{}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("vwp: open package dir: %w", err)
	}
	sort.Strings(src.files)
	return src, nil
}

func (s *dirSource) Origin() string { return s.origin }

func (s *dirSource) List() []string {
	out := make([]string, len(s.files))
	copy(out, s.files)
	return out
}

func (s *dirSource) Has(relPath string) bool {
	_, ok := s.fileSet[relPath]
	return ok
}

func (s *dirSource) Read(relPath string) ([]byte, error) {
	if IsUnsafePath(relPath) {
		return nil, fmt.Errorf("vwp: unsafe package path: %s", relPath)
	}
	if !s.Has(relPath) {
		return nil, fmt.Errorf("vwp: no such file in package: %s", relPath)
	}
	return os.ReadFile(filepath.Join(s.rootDir, filepath.FromSlash(relPath)))
}

func (s *dirSource) UnsafeEntries() []string { return nil }

type zipSource struct {
	origin  string
	entries map[string][]byte
	names   []string
	unsafe  []string
}

func openZip(data []byte, origin string, limits Limits) (*zipSource, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("vwp: open package archive: %w", err)
	}
	src := &zipSource{origin: origin, entries: map[string][]byte{}}
	fileCount := 0
	var totalBytes int64
	for _, entry := range zr.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		fileCount++
		if fileCount > limits.MaxEntries {
			return nil, fmt.Errorf("vwp: archive exceeds entry limit (%d)", limits.MaxEntries)
		}
		name := entry.Name
		if IsUnsafePath(name) {
			src.unsafe = append(src.unsafe, name)
			continue
		}
		content, err := readZipEntry(entry, limits, &totalBytes)
		if err != nil {
			return nil, err
		}
		src.entries[name] = content
		src.names = append(src.names, name)
	}
	sort.Strings(src.names)
	return src, nil
}

// readZipEntry reads one entry while enforcing per-entry and total
// uncompressed-size limits against the actual decompressed bytes (declared
// sizes in headers are untrusted).
func readZipEntry(entry *zip.File, limits Limits, totalBytes *int64) ([]byte, error) {
	remainingTotal := limits.MaxTotalBytes - *totalBytes
	maxAllowed := limits.MaxEntryBytes
	if remainingTotal < maxAllowed {
		maxAllowed = remainingTotal
	}
	rc, err := entry.Open()
	if err != nil {
		return nil, fmt.Errorf("vwp: read archive entry %s: %w", entry.Name, err)
	}
	defer rc.Close() //nolint:errcheck // read-only close
	content, err := io.ReadAll(io.LimitReader(rc, maxAllowed+1))
	if err != nil {
		return nil, fmt.Errorf("vwp: read archive entry %s: %w", entry.Name, err)
	}
	if int64(len(content)) > maxAllowed {
		return nil, fmt.Errorf("vwp: archive exceeds decompression limits at entry %s (max entry %d bytes, max total %d bytes)",
			entry.Name, limits.MaxEntryBytes, limits.MaxTotalBytes)
	}
	*totalBytes += int64(len(content))
	return content, nil
}

func (s *zipSource) Origin() string { return s.origin }

func (s *zipSource) List() []string {
	out := make([]string, len(s.names))
	copy(out, s.names)
	return out
}

func (s *zipSource) Has(relPath string) bool {
	_, ok := s.entries[relPath]
	return ok
}

func (s *zipSource) Read(relPath string) ([]byte, error) {
	data, ok := s.entries[relPath]
	if !ok {
		return nil, fmt.Errorf("vwp: no such file in package: %s", relPath)
	}
	return data, nil
}

func (s *zipSource) UnsafeEntries() []string {
	out := make([]string, len(s.unsafe))
	copy(out, s.unsafe)
	return out
}

// PackOption configures Pack.
type PackOption func(*packConfig)

type packConfig struct {
	writeHashes bool
}

// WithWriteHashes recomputes the SHA-256 digest of every part declared in
// the manifest and rewrites manifest.json in the package directory before
// packing. Packing fails if a declared part is missing.
func WithWriteHashes() PackOption {
	return func(c *packConfig) { c.writeHashes = true }
}

// Pack zips an unpacked package directory into a .vwp archive at output.
// The directory must contain manifest.json.
func Pack(dir, output string, opts ...PackOption) error {
	var cfg packConfig
	for _, o := range opts {
		o(&cfg)
	}
	if _, err := os.Stat(filepath.Join(dir, ManifestPath)); err != nil {
		return fmt.Errorf("vwp: pack: %s does not contain manifest.json", dir)
	}
	if cfg.writeHashes {
		if err := WriteManifestHashes(dir); err != nil {
			return err
		}
	}
	src, err := openDir(dir)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, relPath := range src.List() {
		w, err := zw.Create(relPath)
		if err != nil {
			return fmt.Errorf("vwp: pack %s: %w", relPath, err)
		}
		data, err := src.Read(relPath)
		if err != nil {
			return err
		}
		if _, err := w.Write(data); err != nil {
			return fmt.Errorf("vwp: pack %s: %w", relPath, err)
		}
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("vwp: pack: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return fmt.Errorf("vwp: pack: %w", err)
	}
	return os.WriteFile(output, buf.Bytes(), 0o644)
}

// WriteManifestHashes recomputes SHA-256 digests for every part declared in
// the manifest of the unpacked package at dir and rewrites manifest.json.
// It fails if the manifest declares a part that does not exist on disk.
func WriteManifestHashes(dir string) error {
	manifestPath := filepath.Join(dir, ManifestPath)
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("vwp: write hashes: %w", err)
	}
	m, err := ParseManifest(raw)
	if err != nil {
		return err
	}
	refs := []*PartRef{}
	if m.Workproduct != nil {
		refs = append(refs, m.Workproduct)
	}
	if m.Provenance != nil {
		refs = append(refs, m.Provenance)
	}
	for i := range m.Parts {
		refs = append(refs, &m.Parts[i])
	}
	for i := range m.Signatures {
		refs = append(refs, &m.Signatures[i])
	}
	for _, ref := range refs {
		if ref.Path == "" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(ref.Path)))
		if err != nil {
			return fmt.Errorf("vwp: manifest declares part that does not exist: %s", ref.Path)
		}
		ref.SHA256 = SHA256Hex(data)
	}
	encoded, err := m.Encode()
	if err != nil {
		return err
	}
	return os.WriteFile(manifestPath, encoded, 0o644)
}

// Unpack extracts a .vwp archive into outDir. Entries with unsafe paths
// (absolute, drive-letter, backslash, or ".." segments) abort the extraction
// before anything is written, and decompression limits apply.
func Unpack(input, outDir string, opts ...OpenOption) error {
	data, err := os.ReadFile(input)
	if err != nil {
		return fmt.Errorf("vwp: unpack: %w", err)
	}
	cfg := openConfig{limits: DefaultLimits()}
	for _, o := range opts {
		o(&cfg)
	}
	src, err := openZip(data, input, cfg.limits.withDefaults())
	if err != nil {
		return err
	}
	if unsafe := src.UnsafeEntries(); len(unsafe) > 0 {
		return fmt.Errorf("vwp: refusing to unpack; unsafe entry paths: %s", strings.Join(unsafe, ", "))
	}
	resolvedOut, err := filepath.Abs(outDir)
	if err != nil {
		return fmt.Errorf("vwp: unpack: %w", err)
	}
	for _, relPath := range src.List() {
		target := filepath.Join(resolvedOut, filepath.FromSlash(relPath))
		// Defense in depth: never write outside the output directory.
		if target != resolvedOut && !strings.HasPrefix(target, resolvedOut+string(filepath.Separator)) {
			return fmt.Errorf("vwp: refusing to write outside output directory: %s", relPath)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("vwp: unpack %s: %w", relPath, err)
		}
		content, err := src.Read(relPath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(target, content, 0o644); err != nil {
			return fmt.Errorf("vwp: unpack %s: %w", relPath, err)
		}
	}
	return nil
}

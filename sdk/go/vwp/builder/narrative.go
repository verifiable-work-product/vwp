package builder

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/schema"
)

// Part roles of the memo format snapshot and the narrative.
const (
	RoleNarrative  = "narrative"
	RoleMemoFormat = "memo-format"
)

// Graph builds the integrity graph of the package as it is on disk
// (vwp.BuildGraph).
func (b *Builder) Graph() (*vwp.Graph, error) {
	pkg, err := vwp.OpenPackage(b.dir)
	if err != nil {
		return nil, fmt.Errorf("vwp/builder: failed to open package: %w", err)
	}
	g, err := vwp.BuildGraph(pkg)
	if err != nil {
		return nil, fmt.Errorf("vwp/builder: failed to build the integrity graph: %w", err)
	}
	return g, nil
}

// Digests computes the package's current integrity roots (the manifest
// digests block). It fails while the package has unresolved references or
// dependency cycles.
func (b *Builder) Digests() (*vwp.Digests, error) {
	g, err := b.Graph()
	if err != nil {
		return nil, err
	}
	d, err := g.Digests()
	if err != nil {
		return nil, fmt.Errorf("vwp/builder: %w", err)
	}
	return d, nil
}

// RefreshDigests recomputes the integrity roots and writes them to the
// manifest. Every builder mutation already does this (through the manifest
// save), so callers only need it after editing parts by other means.
func (b *Builder) RefreshDigests() error {
	d, err := b.Digests()
	if err != nil {
		return err
	}
	b.manifest.Digests = d
	return b.saveManifest()
}

// refreshDigests recomputes the manifest digests block from the parts on
// disk, dropping it when the roots are undefined (unresolved references, a
// cycle, an unreadable part): a package never carries stale roots, and
// core.digests.match skips a package without them while
// core.graph.refsResolve reports why.
func (b *Builder) refreshDigests() {
	pkg, err := vwp.OpenPackage(b.dir)
	if err != nil {
		b.manifest.Digests = nil
		return
	}
	d, err := vwp.ComputeDigests(pkg)
	if err != nil {
		b.manifest.Digests = nil
		return
	}
	b.manifest.Digests = d
}

// SetMemoFormat snapshots the organization's memo format into the package
// at narrative/format.json. The format is configuration, so an existing
// snapshot is overwritten rather than conflicting.
func (b *Builder) SetMemoFormat(f vwp.MemoFormat) error {
	if err := f.Validate(); err != nil {
		return fmt.Errorf("vwp/builder: failed to set memo format: %w", err)
	}
	if err := validateObject(schema.MemoFormat, f); err != nil {
		return fmt.Errorf("vwp/builder: failed to set memo format: %w", err)
	}
	data, err := vwp.CanonicalJSON(f)
	if err != nil {
		return fmt.Errorf("vwp/builder: failed to encode memo format: %w", err)
	}
	return b.ReplaceFilePart(vwp.MemoFormatPath, "application/json", RoleMemoFormat, data)
}

// MemoFormat returns the format the package renders with: the snapshot
// when one is present (configured true), else vwp.DefaultMemoFormat().
func (b *Builder) MemoFormat() (vwp.MemoFormat, bool, error) {
	f, _, configured, err := b.memoFormatInUse()
	return f, configured, err
}

// memoFormatInUse returns the format in use together with its self hash
// (the narrative's FormatHash): hashed over the snapshot part's bytes when
// present, else over the default format.
func (b *Builder) memoFormatInUse() (vwp.MemoFormat, string, bool, error) {
	raw, err := os.ReadFile(b.path(vwp.MemoFormatPath))
	if os.IsNotExist(err) {
		f := vwp.DefaultMemoFormat()
		hash, err := vwp.SelfHash(vwp.KindMemoFormat, f)
		if err != nil {
			return vwp.MemoFormat{}, "", false, fmt.Errorf("vwp/builder: failed to hash default memo format: %w", err)
		}
		return f, hash, false, nil
	}
	if err != nil {
		return vwp.MemoFormat{}, "", false, fmt.Errorf("vwp/builder: failed to read memo format: %w", err)
	}
	var f vwp.MemoFormat
	if err := json.Unmarshal(raw, &f); err != nil {
		return vwp.MemoFormat{}, "", false, fmt.Errorf("vwp/builder: failed to parse memo format: %w", err)
	}
	if err := f.Validate(); err != nil {
		return vwp.MemoFormat{}, "", false, fmt.Errorf("vwp/builder: memo format snapshot is invalid: %w", err)
	}
	hash, err := vwp.SelfHash(vwp.KindMemoFormat, json.RawMessage(raw))
	if err != nil {
		return vwp.MemoFormat{}, "", false, fmt.Errorf("vwp/builder: failed to hash memo format: %w", err)
	}
	return f, hash, true, nil
}

// Narrative returns the narrative part when the package has one.
func (b *Builder) Narrative() (*vwp.Narrative, bool, error) {
	var n vwp.Narrative
	err := readJSONFile(b.path(vwp.NarrativePath), &n)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("vwp/builder: failed to read narrative: %w", err)
	}
	return &n, true, nil
}

// SetNarrativeSection upserts one top-level narrative section (the whole
// section, subsections included, is replaced). sec.Id must name a section
// of the format in use (snapshot or default) of kind narrative or
// conclusion. Cites are given by id: the builder resolves each against the
// package's integrity graph and stamps the cited object's current closure
// hash (an unknown id is ErrNotFound; a hash the caller supplied is
// replaced). The narrative is stamped with the format's self hash and the
// analysis root it was written against. A conclusion-kind section also
// fills Workproduct.ConclusionSummary with its conclusion-role paragraphs
// joined by a blank line.
func (b *Builder) SetNarrativeSection(sec vwp.NarrativeSection) error {
	if err := sec.ValidateStructure(); err != nil {
		return fmt.Errorf("vwp/builder: failed to set narrative section: %w", err)
	}
	format, formatHash, configured, err := b.memoFormatInUse()
	if err != nil {
		return err
	}
	if !configured {
		// Pin the layout the prose follows into the package, so a later
		// change to the SDK default cannot orphan this narrative.
		if err := b.SetMemoFormat(format); err != nil {
			return err
		}
		if format, formatHash, _, err = b.memoFormatInUse(); err != nil {
			return err
		}
	}
	fs, ok := format.Section(sec.Id)
	if !ok {
		return fmt.Errorf("vwp/builder: failed to set narrative section: %q is not a section of the memo format", sec.Id)
	}
	if !vwp.IsProseKind(fs.Kind) {
		return fmt.Errorf("vwp/builder: failed to set narrative section: format section %q is of kind %s, not prose", sec.Id, fs.Kind)
	}

	// Stamp the cites against the graph as it stands. The conclusion
	// summary rewrite below does not move any hash (it is stripped), so
	// the graph can be taken first.
	g, err := b.Graph()
	if err != nil {
		return err
	}
	digests, err := g.Digests()
	if err != nil {
		return fmt.Errorf("vwp/builder: failed to set narrative section: %w", err)
	}
	if err := stampCites(&sec, g); err != nil {
		return err
	}

	n, _, err := b.Narrative()
	if err != nil {
		return err
	}
	if n == nil {
		n = &vwp.Narrative{}
	}
	replaced := false
	for i := range n.Sections {
		if n.Sections[i].Id == sec.Id {
			n.Sections[i] = sec
			replaced = true
			break
		}
	}
	if !replaced {
		n.Sections = append(n.Sections, sec)
	}

	if fs.Kind == vwp.SectionKindConclusion {
		if err := b.setConclusionSummary(conclusionSummaryOf(sec)); err != nil {
			return err
		}
	}
	n.FormatHash = formatHash
	n.AnalysisRoot = digests.AnalysisRoot
	if n.Sections == nil {
		n.Sections = []vwp.NarrativeSection{}
	}
	if err := validateObject(schema.Narrative, n); err != nil {
		return err
	}
	data, err := vwp.CanonicalJSON(n)
	if err != nil {
		return fmt.Errorf("vwp/builder: failed to encode narrative: %w", err)
	}
	return b.ReplaceFilePart(vwp.NarrativePath, "application/json", RoleNarrative, data)
}

// stampCites resolves every cite of the section (subsections included) and
// records the cited object's closure hash.
func stampCites(sec *vwp.NarrativeSection, g *vwp.Graph) error {
	for i := range sec.Paragraphs {
		for j := range sec.Paragraphs[i].Cites {
			c := &sec.Paragraphs[i].Cites[j]
			obj, ok := g.Lookup(c.Id)
			if !ok {
				return fmt.Errorf("%w: section %s cites %s", ErrNotFound, sec.Id, c.Id)
			}
			if obj.Tainted || obj.ClosureHash == "" {
				return fmt.Errorf("vwp/builder: failed to stamp cite %s in section %s: the object has unresolved references", c.Id, sec.Id)
			}
			c.Hash = obj.ClosureHash
		}
	}
	for i := range sec.Subsections {
		if err := stampCites(&sec.Subsections[i], g); err != nil {
			return err
		}
	}
	return nil
}

// ClearNarrative removes the narrative part. A conclusion summary the
// narrative wrote is cleared with it; one written by other means is kept.
// Clearing a package without a narrative is a no-op.
func (b *Builder) ClearNarrative() error {
	n, ok, err := b.Narrative()
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	format, _, _, err := b.memoFormatInUse()
	if err != nil {
		return err
	}
	for _, sec := range n.Sections {
		fs, ok := format.Section(sec.Id)
		if !ok || fs.Kind != vwp.SectionKindConclusion {
			continue
		}
		summary := conclusionSummaryOf(sec)
		if summary == "" {
			continue
		}
		var wp vwp.Workproduct
		if err := readJSONFile(b.path(vwp.WorkproductPath), &wp); err != nil {
			return fmt.Errorf("vwp/builder: failed to read workproduct: %w", err)
		}
		if wp.ConclusionSummary == summary {
			if err := b.setConclusionSummary(""); err != nil {
				return err
			}
		}
	}
	if err := os.Remove(b.path(vwp.NarrativePath)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("vwp/builder: failed to remove narrative: %w", err)
	}
	b.removeManifestPart(vwp.NarrativePath)
	return b.saveManifest()
}

// conclusionSummaryOf joins a section's conclusion-role paragraphs
// (subsections included, in order) with a blank line.
func conclusionSummaryOf(sec vwp.NarrativeSection) string {
	var texts []string
	var walk func(s vwp.NarrativeSection)
	walk = func(s vwp.NarrativeSection) {
		for _, p := range s.Paragraphs {
			if p.Role == vwp.ParagraphRoleConclusion {
				texts = append(texts, strings.TrimSpace(p.Text))
			}
		}
		for _, sub := range s.Subsections {
			walk(sub)
		}
	}
	walk(sec)
	return strings.Join(texts, "\n\n")
}

// setConclusionSummary rewrites workproduct.json with the given summary,
// re-validating it and re-hashing the part. Unchanged summaries are a no-op.
func (b *Builder) setConclusionSummary(summary string) error {
	var wp vwp.Workproduct
	if err := readJSONFile(b.path(vwp.WorkproductPath), &wp); err != nil {
		return fmt.Errorf("vwp/builder: failed to read workproduct: %w", err)
	}
	if wp.ConclusionSummary == summary {
		return nil
	}
	wp.ConclusionSummary = summary
	if err := validateObject(schema.Workproduct, wp); err != nil {
		return err
	}
	data, err := vwp.EncodeJSON(wp)
	if err != nil {
		return err
	}
	if err := b.writePart(vwp.WorkproductPath, data); err != nil {
		return err
	}
	b.upsertManifestPart(vwp.WorkproductPath, "application/json", RoleWorkproduct, vwp.SHA256Hex(data))
	return b.saveManifest()
}

// removeManifestPart drops a part reference from manifest.parts.
func (b *Builder) removeManifestPart(relPath string) {
	kept := make([]vwp.PartRef, 0, len(b.manifest.Parts))
	for _, p := range b.manifest.Parts {
		if p.Path != relPath {
			kept = append(kept, p)
		}
	}
	b.manifest.Parts = kept
}

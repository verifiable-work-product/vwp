package builder

import (
	"fmt"
	"strings"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
)

// RenderPath is where Render writes the package's markdown rendering.
const RenderPath = "renderings/memo.md"

// Render writes a minimal, deterministic markdown rendering of the package's
// claims, judgments, and shipped validations to renderings/memo.md.
// Renderings are derived views and are regenerated (overwritten) on every
// call; the structured package remains the source of truth.
func (b *Builder) Render() error {
	var wp vwp.Workproduct
	if err := readJSONFile(b.path(vwp.WorkproductPath), &wp); err != nil {
		return fmt.Errorf("vwp/builder: render: %w", err)
	}

	var sb strings.Builder
	title := wp.Title
	if title == "" {
		title = b.manifest.PackageId
	}
	fmt.Fprintf(&sb, "# %s\n\n", title)
	fmt.Fprintf(&sb, "- Package: `%s`\n", b.manifest.PackageId)
	fmt.Fprintf(&sb, "- Profile: `%s`\n", b.manifest.Profile)
	if b.manifest.Extension != "" {
		fmt.Fprintf(&sb, "- Extension: `%s`\n", b.manifest.Extension)
	}
	if wp.Status != "" {
		fmt.Fprintf(&sb, "- Status: %s\n", wp.Status)
	}
	sb.WriteString("\n> Derived rendering. The structured package is the source of truth.\n")

	if wp.ConclusionSummary != "" {
		fmt.Fprintf(&sb, "\n## Conclusion\n\n%s\n", wp.ConclusionSummary)
	}

	renderClaims(&sb, b)
	renderJudgments(&sb, b)
	renderValidations(&sb, b)

	content := []byte(sb.String())
	// Renderings are derived: overwrite rather than conflict on change.
	if err := b.writePart(RenderPath, content); err != nil {
		return err
	}
	b.upsertManifestPart(RenderPath, "text/markdown", RoleRendering, vwp.SHA256Hex(content))
	return b.saveManifest()
}

func renderClaims(sb *strings.Builder, b *Builder) {
	doc, err := b.readCollectionDoc(vwp.ClaimsPath)
	if err != nil {
		return
	}
	items, _ := doc["claims"].([]any)
	if len(items) == 0 {
		return
	}
	sb.WriteString("\n## Claims\n")
	for _, item := range items {
		claim, ok := item.(map[string]any)
		if !ok {
			continue
		}
		fmt.Fprintf(sb, "\n### %v\n\n%v\n", claim["id"], claim["statement"])
		if conclusion, ok := claim["conclusion"].(string); ok && conclusion != "" {
			fmt.Fprintf(sb, "\n- Conclusion: %s\n", conclusion)
		}
		if status, ok := claim["status"].(string); ok && status != "" {
			fmt.Fprintf(sb, "- Status: %s\n", status)
		}
	}
}

func renderJudgments(sb *strings.Builder, b *Builder) {
	doc, err := b.readCollectionDoc(vwp.JudgmentsPath)
	if err != nil {
		return
	}
	items, _ := doc["judgments"].([]any)
	if len(items) == 0 {
		return
	}
	sb.WriteString("\n## Judgments\n")
	for _, item := range items {
		j, ok := item.(map[string]any)
		if !ok {
			continue
		}
		fmt.Fprintf(sb, "\n### %v — %v\n\n%v\n", j["id"], j["topic"], j["question"])
		if pc, ok := j["proposedConclusion"].(string); ok && pc != "" {
			fmt.Fprintf(sb, "\n- Proposed conclusion: %s\n", pc)
		}
		if status, ok := j["status"].(string); ok && status != "" {
			fmt.Fprintf(sb, "- Status: %s\n", status)
		}
	}
}

func renderValidations(sb *strings.Builder, b *Builder) {
	doc, err := b.readCollectionDoc(vwp.ValidationsPath)
	if err != nil {
		return
	}
	items, _ := doc["validations"].([]any)
	if len(items) == 0 {
		return
	}
	sb.WriteString("\n## Validations\n\n")
	for _, item := range items {
		v, ok := item.(map[string]any)
		if !ok {
			continue
		}
		fmt.Fprintf(sb, "- `%v` — %v", v["validator"], v["status"])
		if msg, ok := v["message"].(string); ok && msg != "" {
			fmt.Fprintf(sb, " — %s", msg)
		}
		sb.WriteString("\n")
	}
}

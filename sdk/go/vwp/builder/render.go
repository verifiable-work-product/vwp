package builder

import (
	"fmt"
	"strings"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
)

// RenderPath is where Render writes the package's markdown rendering.
const RenderPath = "renderings/memo.md"

// Render writes a minimal, deterministic markdown rendering of the package's
// claims, judgments, shipped validations, review actions (approvals) and
// analysis attestations to renderings/memo.md.
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
	renderApprovals(&sb, b)
	renderAttestations(&sb, b)

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

func renderApprovals(sb *strings.Builder, b *Builder) {
	doc, err := b.readCollectionDoc(vwp.ApprovalsPath)
	if err != nil {
		return
	}
	items, _ := doc["approvals"].([]any)
	if len(items) == 0 {
		return
	}
	sb.WriteString("\n## Review actions\n\n")
	for _, item := range items {
		a, ok := item.(map[string]any)
		if !ok {
			continue
		}
		approver, _ := a["approver"].(map[string]any)
		name, _ := approver["name"].(string)
		if name == "" {
			name, _ = approver["id"].(string)
		}
		fmt.Fprintf(sb, "- %v by %s", a["action"], name)
		if role, ok := a["role"].(string); ok && role != "" {
			fmt.Fprintf(sb, " (%s)", role)
		}
		if ts, ok := a["timestamp"].(string); ok && ts != "" {
			fmt.Fprintf(sb, " at %s", ts)
		}
		if scope, ok := a["scope"].(string); ok && scope != "" {
			fmt.Fprintf(sb, " — scope: %s", scope)
		}
		if comments, ok := a["comments"].(string); ok && comments != "" {
			fmt.Fprintf(sb, " — %s", comments)
		}
		sb.WriteString("\n")
	}
}

// renderAttestations lists every analysis attestation (adversarial review)
// in the package with its findings, so the memo shows what was challenged
// and what a human must still disposition. Attestations never affect
// package validity; they are rendered for the reader, not enforced.
func renderAttestations(sb *strings.Builder, b *Builder) {
	var atts []map[string]any
	for _, part := range b.manifest.Parts {
		if part.Role != RoleAttestation {
			continue
		}
		var att map[string]any
		if err := readJSONFile(b.path(part.Path), &att); err != nil {
			continue
		}
		atts = append(atts, att)
	}
	if len(atts) == 0 {
		return
	}
	sb.WriteString("\n## Analysis attestations\n")
	for _, att := range atts {
		fmt.Fprintf(sb, "\n### %v\n\n", att["id"])
		if by, ok := att["performedBy"].(map[string]any); ok {
			fmt.Fprintf(sb, "- Performed by: %v (%v)", by["name"], by["type"])
			if model, ok := att["model"].(map[string]any); ok {
				fmt.Fprintf(sb, " — model %v/%v", model["provider"], model["name"])
				if v, ok := model["version"].(string); ok && v != "" {
					fmt.Fprintf(sb, " %s", v)
				}
			}
			sb.WriteString("\n")
		}
		if scope, ok := att["scope"].(map[string]any); ok {
			if ev, ok := scope["evidenceExamined"].([]any); ok {
				fmt.Fprintf(sb, "- Evidence examined: %d\n", len(ev))
			}
			if claims, ok := scope["claimsChallenged"].([]any); ok && len(claims) > 0 {
				fmt.Fprintf(sb, "- Claims challenged: %d\n", len(claims))
			}
		}
		findings, _ := att["findings"].([]any)
		if len(findings) == 0 {
			sb.WriteString("- Findings: none recorded\n")
		} else {
			fmt.Fprintf(sb, "\n**Findings (%d)**\n\n", len(findings))
			for _, f := range findings {
				fm, ok := f.(map[string]any)
				if !ok {
					continue
				}
				fmt.Fprintf(sb, "- [%v] %v — %v", fm["severity"], fm["id"], fm["summary"])
				if rel, ok := fm["relatedClaims"].([]any); ok && len(rel) > 0 {
					fmt.Fprintf(sb, " (claims: %v)", rel)
				}
				sb.WriteString("\n")
			}
		}
		if lims, ok := att["limitations"].([]any); ok && len(lims) > 0 {
			sb.WriteString("\n**Limitations**\n\n")
			for _, l := range lims {
				fmt.Fprintf(sb, "- %v\n", l)
			}
		}
		if disps, ok := att["dispositions"].([]any); ok && len(disps) > 0 {
			sb.WriteString("\n**Dispositions**\n\n")
			for _, d := range disps {
				dm, ok := d.(map[string]any)
				if !ok {
					continue
				}
				by, _ := dm["by"].(map[string]any)
				fmt.Fprintf(sb, "- %v: %v by %v", dm["findingId"], dm["decision"], by["name"])
				if note, ok := dm["note"].(string); ok && note != "" {
					fmt.Fprintf(sb, " — %s", note)
				}
				sb.WriteString("\n")
			}
		}
	}
}

package builder

import (
	"fmt"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/schema"
)

// ErrNotFound is returned when an operation names an object id that is not
// in the package.
var ErrNotFound = fmt.Errorf("vwp/builder: object not found")

// Judgment status vocabulary (judgment.schema.json). Agents may only write
// the first two; the last three are conferred by a recorded human review
// action (SetJudgmentStatus from an approval flow).
const (
	JudgmentStatusOpen           = "open"
	JudgmentStatusReviewRequired = "reviewRequired"
	JudgmentStatusReviewed       = "reviewed"
	JudgmentStatusApproved       = "approved"
	JudgmentStatusRejected       = "rejected"
)

// SetJudgmentStatus records the outcome of a HUMAN review of one judgment:
// it rewrites the judgment's status in judgments/judgments.json (schema
// re-validated) and re-hashes the part. Idempotent for an unchanged status.
// This is the only in-place edit the builder performs on a collection
// object, and it is reserved for review dispositions — content fields are
// never rewritten.
func (b *Builder) SetJudgmentStatus(judgmentId, status string) error {
	switch status {
	case JudgmentStatusOpen, JudgmentStatusReviewRequired, JudgmentStatusReviewed, JudgmentStatusApproved, JudgmentStatusRejected:
	default:
		return fmt.Errorf("vwp/builder: invalid judgment status %q", status)
	}
	if judgmentId == "" {
		return fmt.Errorf("vwp/builder: judgment id is required")
	}
	doc, err := b.readCollectionDoc(vwp.JudgmentsPath)
	if err != nil {
		return err
	}
	items, _ := doc["judgments"].([]any)
	for i, item := range items {
		m, ok := item.(map[string]any)
		if !ok || m["id"] != judgmentId {
			continue
		}
		if current, _ := m["status"].(string); current == status {
			return nil
		}
		m["status"] = status
		if err := validateObject(schema.Judgment, m); err != nil {
			return err
		}
		items[i] = m
		doc["judgments"] = items
		return b.writeCollectionDoc(vwp.JudgmentsPath, RoleJudgments, doc)
	}
	return fmt.Errorf("%w: judgment %s", ErrNotFound, judgmentId)
}

// Package vwp contains stub types for VWP (Verifiable Work Product).
//
// Status: stub / preview. These structs mirror the v0.1 core objects. The
// normative definitions are the JSON Schemas in schemas/; if these types and
// the schemas disagree, the schemas win. Dates are ISO 8601 strings.
package vwp

// PackagePart is a part (file) declared in the package manifest.
type PackagePart struct {
	Path      string `json:"path"`
	MediaType string `json:"mediaType"`
	SHA256    string `json:"sha256,omitempty"`
	Role      string `json:"role,omitempty"`
}

// Provenance describes how a package or work product was produced.
type Provenance struct {
	CreatedAt      string                 `json:"createdAt"`
	CreatedBy      string                 `json:"createdBy"`
	Toolchain      map[string]interface{} `json:"toolchain,omitempty"`
	Models         map[string]interface{} `json:"models,omitempty"`
	PolicyVersions map[string]interface{} `json:"policyVersions,omitempty"`
	SourceSystems  []string               `json:"sourceSystems,omitempty"`
	Environment    map[string]interface{} `json:"environment,omitempty"`
	Notes          string                 `json:"notes,omitempty"`
}

// Manifest is the package manifest (manifest.json).
type Manifest struct {
	VWPVersion  string                 `json:"vwpVersion"`
	PackageID   string                 `json:"packageId"`
	CreatedAt   string                 `json:"createdAt"`
	Profile     string                 `json:"profile"`
	Extension   string                 `json:"extension,omitempty"`
	Subject     map[string]interface{} `json:"subject,omitempty"`
	WorkProduct string                 `json:"workproduct"`
	Parts       []PackagePart          `json:"parts,omitempty"`
	Provenance  Provenance             `json:"provenance"`
	Signatures  []interface{}          `json:"signatures,omitempty"`
}

// WorkProduct is the work product being packaged (workproduct.json).
type WorkProduct struct {
	ID                string                 `json:"id"`
	Title             string                 `json:"title"`
	Domain            string                 `json:"domain"`
	Profile           string                 `json:"profile"`
	Extension         string                 `json:"extension,omitempty"`
	StandardRefs      []string               `json:"standardRefs,omitempty"`
	Entity            string                 `json:"entity,omitempty"`
	Period            string                 `json:"period,omitempty"`
	Subject           map[string]interface{} `json:"subject,omitempty"`
	ConclusionSummary string                 `json:"conclusionSummary,omitempty"`
	Status            string                 `json:"status"`
	CreatedBy         string                 `json:"createdBy,omitempty"`
	ReviewState       string                 `json:"reviewState,omitempty"`
}

// Evidence is an evidence item under evidence/.
type Evidence struct {
	ID            string `json:"id"`
	Type          string `json:"type"`
	Title         string `json:"title"`
	SourceSystem  string `json:"sourceSystem,omitempty"`
	Path          string `json:"path,omitempty"`
	ExternalRef   string `json:"externalRef,omitempty"`
	SHA256        string `json:"sha256,omitempty"`
	RetrievedAt   string `json:"retrievedAt,omitempty"`
	PeriodCovered string `json:"periodCovered,omitempty"`
	AccessNotes   string `json:"accessNotes,omitempty"`
}

// Fact is a fact extracted from evidence, under facts/.
type Fact struct {
	ID               string      `json:"id"`
	Statement        string      `json:"statement"`
	Value            interface{} `json:"value,omitempty"`
	EvidenceRefs     []string    `json:"evidenceRefs,omitempty"`
	ExtractionMethod string      `json:"extractionMethod,omitempty"`
	Confidence       *float64    `json:"confidence,omitempty"`
	RequiresReview   *bool       `json:"requiresReview,omitempty"`
}

// Claim is an evidence-backed conclusion, under claims/.
type Claim struct {
	ID                     string   `json:"id"`
	Statement              string   `json:"statement"`
	Domain                 string   `json:"domain,omitempty"`
	StandardRefs           []string `json:"standardRefs,omitempty"`
	Assertions             []string `json:"assertions,omitempty"`
	SupportingFacts        []string `json:"supportingFacts,omitempty"`
	SupportingEvidence     []string `json:"supportingEvidence,omitempty"`
	SupportingCalculations []string `json:"supportingCalculations,omitempty"`
	Judgments              []string `json:"judgments,omitempty"`
	Conclusion             string   `json:"conclusion,omitempty"`
	Status                 string   `json:"status,omitempty"`
	RequiresReview         *bool    `json:"requiresReview,omitempty"`
}

// Judgment is a professional judgment, under judgments/.
type Judgment struct {
	ID                   string        `json:"id"`
	Topic                string        `json:"topic"`
	Question             string        `json:"question"`
	ProposedConclusion   string        `json:"proposedConclusion,omitempty"`
	Basis                string        `json:"basis,omitempty"`
	Alternatives         []interface{} `json:"alternatives,omitempty"`
	RiskLevel            string        `json:"riskLevel,omitempty"`
	RequiredApproverRole string        `json:"requiredApproverRole,omitempty"`
	Status               string        `json:"status,omitempty"`
}

// Calculation is a recomputable calculation, under calculations/.
type Calculation struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"`
	Inputs       map[string]interface{} `json:"inputs"`
	Outputs      map[string]interface{} `json:"outputs"`
	Method       string                 `json:"method,omitempty"`
	Validator    string                 `json:"validator,omitempty"`
	Recomputable *bool                  `json:"recomputable,omitempty"`
}

// Validation result statuses.
const (
	ValidationStatusPass           = "pass"
	ValidationStatusFail           = "fail"
	ValidationStatusWarning        = "warning"
	ValidationStatusReviewRequired = "reviewRequired"
	ValidationStatusNotRun         = "notRun"
)

// ValidationResult is the result of running a validator, under validations/.
type ValidationResult struct {
	ID        string                 `json:"id"`
	Validator string                 `json:"validator"`
	Scope     string                 `json:"scope,omitempty"`
	Status    string                 `json:"status"`
	Severity  string                 `json:"severity,omitempty"`
	Message   string                 `json:"message,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// Approval actions.
const (
	ApprovalActionApproved         = "approved"
	ApprovalActionRejected         = "rejected"
	ApprovalActionRequestedChanges = "requestedChanges"
	ApprovalActionAcknowledged     = "acknowledged"
)

// Approval is a review/approval event, under approvals/.
type Approval struct {
	ID        string `json:"id"`
	Approver  string `json:"approver"`
	Role      string `json:"role,omitempty"`
	Action    string `json:"action"`
	Timestamp string `json:"timestamp"`
	Scope     string `json:"scope,omitempty"`
	Comments  string `json:"comments,omitempty"`
}

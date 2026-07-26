package vwp

import "encoding/json"

// The structs in this file mirror the VWP v0.1 core objects. The normative
// definitions are the JSON Schemas vendored under schema/vwpschemas; when
// these types and the schemas disagree, the schemas win. Dates and timestamps
// are RFC 3339 strings.

// Status is a validation outcome status (validation.schema.json).
type Status string

// Validation statuses.
const (
	StatusPass           Status = "pass"
	StatusFail           Status = "fail"
	StatusWarning        Status = "warning"
	StatusReviewRequired Status = "reviewRequired"
	StatusNotRun         Status = "notRun"
)

// ValidationResult is a single validator outcome in a verification Report.
// ValidatorId identifies the validator that ran (e.g. "core.partHashes.match");
// Refs lists the offending object ids or part paths, when applicable.
type ValidationResult struct {
	ValidatorId string   `json:"validatorId"`
	Status      Status   `json:"status"`
	Message     string   `json:"message,omitempty"`
	Refs        []string `json:"refs,omitempty"`
}

// Report is the structured result of verifying a package.
type Report struct {
	// PackageOrigin describes where the package was loaded from (for display).
	PackageOrigin string `json:"packageOrigin"`
	// Manifest is the parsed manifest, when it could be read.
	Manifest *Manifest `json:"manifest,omitempty"`
	// Results holds every validator outcome, in execution order.
	Results []ValidationResult `json:"results"`
	// Valid is true when no result has StatusFail.
	Valid bool `json:"valid"`
}

// Result returns the outcome for the given validator id.
func (r *Report) Result(validatorId string) (ValidationResult, bool) {
	for _, res := range r.Results {
		if res.ValidatorId == validatorId {
			return res, true
		}
	}
	return ValidationResult{}, false
}

// Workproduct is the top-level work product (workproduct.json).
type Workproduct struct {
	Id                string         `json:"id"`
	Title             string         `json:"title"`
	Domain            string         `json:"domain"`
	Profile           string         `json:"profile"`
	Extension         string         `json:"extension,omitempty"`
	StandardRefs      []string       `json:"standardRefs,omitempty"`
	Entity            map[string]any `json:"entity,omitempty"`
	Period            map[string]any `json:"period,omitempty"`
	Subject           map[string]any `json:"subject,omitempty"`
	ConclusionSummary string         `json:"conclusionSummary,omitempty"`
	Status            string         `json:"status"`
	CreatedBy         map[string]any `json:"createdBy,omitempty"`
	ReviewState       string         `json:"reviewState,omitempty"`
}

// Evidence is an evidence record in evidence/evidence.json.
type Evidence struct {
	Id            string         `json:"id"`
	Type          string         `json:"type"`
	Title         string         `json:"title"`
	SourceSystem  string         `json:"sourceSystem,omitempty"`
	Path          string         `json:"path,omitempty"`
	ExternalRef   map[string]any `json:"externalRef,omitempty"`
	SHA256        string         `json:"sha256,omitempty"`
	DerivedFrom   string         `json:"derivedFrom,omitempty"`
	Transform     map[string]any `json:"transform,omitempty"`
	RetrievedAt   string         `json:"retrievedAt,omitempty"`
	PeriodCovered map[string]any `json:"periodCovered,omitempty"`
	AccessNotes   string         `json:"accessNotes,omitempty"`
}

// AnchorLines is a 1-based line range within text evidence.
type AnchorLines struct {
	From int `json:"from"`
	To   int `json:"to,omitempty"`
}

// AnchorCells is an A1-style cell reference within tabular evidence.
type AnchorCells struct {
	Sheet string `json:"sheet,omitempty"`
	Range string `json:"range"`
}

// AnchorRegion is a pixel region within image evidence.
type AnchorRegion struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// AnchorSpan is a 0-based, end-exclusive offset range: character offsets
// for charSpan, byte offsets for byteRange.
type AnchorSpan struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// Anchor locates one chunk of the referenced evidence. All fields are
// optional and compose to describe a single chunk (e.g. Page + Lines is a
// pin cite); multiple chunks are multiple Anchors. Quote is the
// machine-checked field: verifiers confirm it appears in text evidence after
// normalization. The remaining fields are advisory locators.
type Anchor struct {
	Quote     string        `json:"quote,omitempty"`
	Page      int           `json:"page,omitempty"`
	Lines     *AnchorLines  `json:"lines,omitempty"`
	Cells     *AnchorCells  `json:"cells,omitempty"`
	CharSpan  *AnchorSpan   `json:"charSpan,omitempty"`
	ByteRange *AnchorSpan   `json:"byteRange,omitempty"`
	Section   string        `json:"section,omitempty"`
	JsonPath  string        `json:"jsonPath,omitempty"`
	Region    *AnchorRegion `json:"region,omitempty"`
}

// EvidenceRef is a fact's reference to supporting evidence: either a bare
// evidence id, or an object carrying anchors locating the observation inside
// the evidence (one Anchor per chunk). It marshals as a bare string when
// Anchors is empty, matching the schema's oneOf.
type EvidenceRef struct {
	EvidenceId string
	Anchors    []Anchor
}

// MarshalJSON encodes a bare-string ref when no anchors are present.
func (r EvidenceRef) MarshalJSON() ([]byte, error) {
	if len(r.Anchors) == 0 {
		return json.Marshal(r.EvidenceId)
	}
	return json.Marshal(struct {
		EvidenceId string   `json:"evidenceId"`
		Anchors    []Anchor `json:"anchors"`
	}{r.EvidenceId, r.Anchors})
}

// UnmarshalJSON accepts both the bare-string and the object form.
func (r *EvidenceRef) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*r = EvidenceRef{EvidenceId: s}
		return nil
	}
	var obj struct {
		EvidenceId string   `json:"evidenceId"`
		Anchors    []Anchor `json:"anchors"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	*r = EvidenceRef{EvidenceId: obj.EvidenceId, Anchors: obj.Anchors}
	return nil
}

// ModelRef identifies an exact AI model (provenance models, fact extraction).
type ModelRef struct {
	Provider string `json:"provider"`
	Name     string `json:"name"`
	Version  string `json:"version"`
	Role     string `json:"role,omitempty"`
}

// Fact is an extracted observation in facts/facts.json.
type Fact struct {
	Id               string        `json:"id"`
	Statement        string        `json:"statement"`
	FactType         string        `json:"factType,omitempty"`
	Value            any           `json:"value,omitempty"`
	EvidenceRefs     []EvidenceRef `json:"evidenceRefs"`
	ExtractionMethod string        `json:"extractionMethod,omitempty"`
	ExtractionModel  *ModelRef     `json:"extractionModel,omitempty"`
	Confidence       *float64      `json:"confidence,omitempty"`
	RequiresReview   *bool         `json:"requiresReview,omitempty"`
}

// Claim is an evidence-backed conclusion in claims/claims.json.
type Claim struct {
	Id                     string   `json:"id"`
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

// JudgmentAlternative is an alternative conclusion considered for a judgment.
type JudgmentAlternative struct {
	Conclusion string `json:"conclusion,omitempty"`
	Notes      string `json:"notes,omitempty"`
}

// Judgment is a professional judgment point in judgments/judgments.json.
type Judgment struct {
	Id                   string                `json:"id"`
	Topic                string                `json:"topic"`
	Question             string                `json:"question"`
	ProposedConclusion   string                `json:"proposedConclusion,omitempty"`
	Basis                string                `json:"basis,omitempty"`
	Alternatives         []JudgmentAlternative `json:"alternatives,omitempty"`
	RiskLevel            string                `json:"riskLevel,omitempty"`
	RequiredApproverRole string                `json:"requiredApproverRole,omitempty"`
	Status               string                `json:"status,omitempty"`
}

// Calculation is a recomputable computation under calculations/.
type Calculation struct {
	Id           string         `json:"id"`
	Type         string         `json:"type"`
	Inputs       map[string]any `json:"inputs"`
	Outputs      map[string]any `json:"outputs"`
	Method       string         `json:"method,omitempty"`
	Validator    string         `json:"validator,omitempty"`
	Recomputable *bool          `json:"recomputable,omitempty"`
}

// Validation is a shipped validator result in validations/validation-report.json.
// (ValidationResult is the in-memory verification outcome; Validation is the
// package part object per validation.schema.json.)
type Validation struct {
	Id        string         `json:"id"`
	Validator string         `json:"validator"`
	Scope     string         `json:"scope,omitempty"`
	Status    Status         `json:"status"`
	Severity  string         `json:"severity,omitempty"`
	Message   string         `json:"message,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
}

// Approver identifies who performed an approval action.
type Approver struct {
	Id    string `json:"id,omitempty"`
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

// Approval is a review/approval event in approvals/approvals.json.
type Approval struct {
	Id        string   `json:"id"`
	Approver  Approver `json:"approver"`
	Role      string   `json:"role,omitempty"`
	Action    string   `json:"action"`
	Timestamp string   `json:"timestamp"`
	Scope     string   `json:"scope,omitempty"`
	Comments  string   `json:"comments,omitempty"`
}

// Approval actions.
const (
	ApprovalActionApproved         = "approved"
	ApprovalActionRejected         = "rejected"
	ApprovalActionRequestedChanges = "requestedChanges"
	ApprovalActionAcknowledged     = "acknowledged"
)

// SessionDigest ties a package to an append-only agent-session transcript
// held by the producing system.
type SessionDigest struct {
	System           string `json:"system"`
	SessionId        string `json:"sessionId"`
	TranscriptSHA256 string `json:"transcriptSha256"`
	EventCount       int    `json:"eventCount"`
}

// ToolchainEntry is a tool recorded in provenance.
type ToolchainEntry struct {
	Name          string         `json:"name"`
	Version       string         `json:"version,omitempty"`
	SessionDigest *SessionDigest `json:"sessionDigest,omitempty"`
}

// ProvenanceAgent identifies who or what created the package.
type ProvenanceAgent struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// PolicyVersion is a policy recorded in provenance.
type PolicyVersion struct {
	Id      string `json:"id"`
	Version string `json:"version,omitempty"`
}

// SourceSystem is a source system recorded in provenance.
type SourceSystem struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// Provenance records how the package was produced (provenance/provenance.json).
type Provenance struct {
	CreatedAt      string           `json:"createdAt"`
	CreatedBy      ProvenanceAgent  `json:"createdBy"`
	Toolchain      []ToolchainEntry `json:"toolchain,omitempty"`
	Models         []ModelRef       `json:"models,omitempty"`
	PolicyVersions []PolicyVersion  `json:"policyVersions,omitempty"`
	SourceSystems  []SourceSystem   `json:"sourceSystems,omitempty"`
	Environment    map[string]any   `json:"environment,omitempty"`
	Notes          string           `json:"notes,omitempty"`
}

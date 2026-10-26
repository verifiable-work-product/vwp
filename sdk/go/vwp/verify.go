package vwp

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/text/unicode/norm"

	"github.com/verifiable-work-product/vwp/sdk/go/vwp/accountingschedule"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/schema"
)

// Core validator ids, per spec/validations.md §5. Independent implementations
// must agree on these ids and their semantics.
const (
	ValidatorPackageSafePaths       = "core.package.safePaths"
	ValidatorManifestExists         = "core.manifest.exists"
	ValidatorManifestSchemaValid    = "core.manifest.schemaValid"
	ValidatorRequiredPartsExist     = "core.requiredParts.exist"
	ValidatorPartHashesMatch        = "core.partHashes.match"
	ValidatorWorkproductSchemaValid = "core.workproduct.schemaValid"
	ValidatorClaimsSchemaValid      = "core.claims.schemaValid"
	ValidatorProvenanceSchemaValid  = "core.provenance.schemaValid"
	ValidatorEvidenceSchemaValid    = "core.evidence.schemaValid"
	ValidatorFactsSchemaValid       = "core.facts.schemaValid"
	ValidatorJudgmentsSchemaValid   = "core.judgments.schemaValid"
	ValidatorApprovalsSchemaValid   = "core.approvals.schemaValid"
	ValidatorValidationsSchemaValid = "core.validations.schemaValid"
	ValidatorAttestationsSchemaOK   = "core.attestations.schemaValid"
	ValidatorEvidenceRefsResolve    = "core.evidenceRefs.resolve"
	ValidatorExternalRefsWellFormed = "core.externalRefs.wellFormed"
	ValidatorExternalRefsReachable  = "core.externalRefs.reachable"
	ValidatorFactAnchorsQuotesMatch = "core.factAnchors.quotesMatch"
	ValidatorClaimRefsResolve       = "core.claimRefs.resolve"
	ValidatorSignaturesPresent      = "core.signatures.present"
	ValidatorSignaturesValid        = "core.signatures.valid"

	// Integrity graph validators (spec/integrity.md section 7); see
	// verify_graph.go. core.graph.refsResolve subsumes core.claimRefs.resolve,
	// which is still emitted for compatibility.
	ValidatorGraphRefsResolve    = "core.graph.refsResolve"
	ValidatorGraphAcyclic        = "core.graph.acyclic"
	ValidatorGraphDeclaredInputs = "core.graph.declaredInputs"
	ValidatorDigestsMatch        = "core.digests.match"
	ValidatorApprovalsCurrent    = "approvals.current"

	// Narrative validators run only when the package carries a narrative
	// part (narrative/narrative.json); see verify_narrative.go.
	ValidatorNarrativeSchemaValid = "narrative.schemaValid"
	ValidatorNarrativeSections    = "narrative.sections"
	ValidatorNarrativeCitations   = "narrative.citations"
	ValidatorNarrativeRoles       = "narrative.roles"
	ValidatorNarrativeCitesFresh  = "narrative.cites.fresh"
	ValidatorNarrativeDrift       = "narrative.drift"

	ValidatorFinanceClaimsHaveAssertions  = "finance.claims.haveAssertions"
	ValidatorFinanceClaimsHaveSupport     = "finance.claims.haveSupport"
	ValidatorFinanceRequiredJudgmentsDone = "finance.requiredJudgments.complete"
)

// Profile and extension identifiers with built-in verification behavior.
const (
	ProfileFinance         = "vwp-finance"
	ExtensionASC606        = "vwp-accounting-asc606"
	ExtensionASC820        = "vwp-accounting-asc820"
	ExtensionWalletControl = "vwp-digital-assets-wallet-ownership-control"
	ExtensionASC35060      = "vwp-accounting-asc350-60"
	ExtensionASC35030      = "vwp-accounting-asc350-30"
	ExtensionStakingReward = "vwp-digital-assets-staking-rewards"
	ExtensionLending       = "vwp-digital-assets-lending"
	ExtensionCostBasis     = "vwp-digital-assets-cost-basis"
	ExtensionStablecoin    = "vwp-digital-assets-stablecoin"
	ExtensionCommissions   = "vwp-accounting-commissions"
)

// requiredParts are the parts every valid package must contain.
var requiredParts = []string{ManifestPath, WorkproductPath, ClaimsPath, ProvenancePath}

// ExtensionContext hands extension validators the collections the core
// verifier already parsed. Slices are empty (never nil-checked) when the
// corresponding part is absent, mirroring the reference CLI's `?? []`.
type ExtensionContext struct {
	Manifest  *Manifest
	Evidence  []map[string]any
	Facts     []map[string]any
	Claims    []map[string]any
	Judgments []map[string]any
}

// ExtensionValidator runs extension-specific checks against a package.
type ExtensionValidator func(pkg Source, ctx *ExtensionContext) []ValidationResult

// VerifyOption configures Verify.
type VerifyOption func(*verifyConfig)

type verifyConfig struct {
	extensions  map[string]ExtensionValidator
	trustedKeys map[string]ed25519.PublicKey
}

// WithExtensionValidators registers extension validators keyed by the
// manifest `extension` identifier they apply to. The validators package
// provides the built-in registry; validators.Verify wires it up.
func WithExtensionValidators(m map[string]ExtensionValidator) VerifyOption {
	return func(c *verifyConfig) {
		if c.extensions == nil {
			c.extensions = map[string]ExtensionValidator{}
		}
		for k, v := range m {
			c.extensions[k] = v
		}
	}
}

var attestationPathRe = regexp.MustCompile(`^attestations/[^/]+\.json$`)

// Verify runs core and profile validators against a package and returns a
// structured report. Extension validators run when registered via
// WithExtensionValidators and the manifest declares a matching extension;
// use validators.Verify for the batteries-included entry point.
//
// Validator ids and semantics mirror the reference TypeScript CLI
// (cli/src/lib/validations.ts) and spec/validations.md §5.
func Verify(pkg Source, opts ...VerifyOption) *Report {
	var cfg verifyConfig
	for _, o := range opts {
		o(&cfg)
	}

	report := &Report{PackageOrigin: pkg.Origin()}
	add := func(r ValidationResult) { report.Results = append(report.Results, r) }
	finish := func() *Report {
		report.Valid = true
		for _, r := range report.Results {
			if r.Status == StatusFail {
				report.Valid = false
				break
			}
		}
		return report
	}

	// core.package.safePaths
	if unsafe := pkg.UnsafeEntries(); len(unsafe) > 0 {
		add(ValidationResult{
			ValidatorId: ValidatorPackageSafePaths,
			Status:      StatusFail,
			Message:     "unsafe entries rejected: " + strings.Join(unsafe, ", "),
			Refs:        unsafe,
		})
	} else {
		add(ValidationResult{ValidatorId: ValidatorPackageSafePaths, Status: StatusPass})
	}

	// core.manifest.exists
	var manifestValue any
	var manifestRaw []byte
	if !pkg.Has(ManifestPath) {
		add(ValidationResult{ValidatorId: ValidatorManifestExists, Status: StatusFail, Message: "manifest.json not found"})
		return finish()
	}
	manifestRaw, err := pkg.Read(ManifestPath)
	if err == nil {
		err = json.Unmarshal(manifestRaw, &manifestValue)
	}
	if err != nil {
		add(ValidationResult{
			ValidatorId: ValidatorManifestExists,
			Status:      StatusFail,
			Message:     fmt.Sprintf("manifest.json is not valid JSON: %v", err),
		})
		return finish()
	}
	add(ValidationResult{ValidatorId: ValidatorManifestExists, Status: StatusPass})

	// core.manifest.schemaValid
	manifestErrs, err := schema.Validate(schema.Manifest, manifestValue)
	if err != nil {
		manifestErrs = []string{err.Error()}
	}
	add(schemaResult(ValidatorManifestSchemaValid, manifestErrs))

	// Typed manifest for downstream logic; a shape mismatch already failed
	// the schema check, so fall back to an empty manifest.
	manifest := &Manifest{}
	if err := json.Unmarshal(manifestRaw, manifest); err != nil {
		manifest = &Manifest{}
	}
	report.Manifest = manifest

	// Required extension outputs are enforced by core verification even when
	// the caller has not registered the financial extension validators.
	verifyRequiredOutputs(pkg, manifest, add)

	// An extension that requires the executable schedule unconditionally
	// (ASC 606) must carry a valid one; every other package validates the
	// artifact when it includes one, and the extension's tie-out decides
	// whether a conditionally required schedule had to be written.
	if pkg.Has(accountingschedule.ArtifactPath) || requiresAccountingScheduleUnconditionally(manifest.Extension) {
		raw, err := pkg.Read(accountingschedule.ArtifactPath)
		if err == nil {
			_, err = accountingschedule.Parse(raw)
		}
		result := ValidationResult{ValidatorId: ValidatorAccountingScheduleValid, Status: StatusPass, Refs: []string{accountingschedule.ArtifactPath}}
		if err != nil {
			result.Status, result.Message = StatusFail, err.Error()
		}
		add(result)
	}

	// core.requiredParts.exist
	var missing []string
	for _, p := range requiredParts {
		if !pkg.Has(p) {
			missing = append(missing, p)
		}
	}
	if len(missing) == 0 {
		add(ValidationResult{ValidatorId: ValidatorRequiredPartsExist, Status: StatusPass})
	} else {
		add(ValidationResult{
			ValidatorId: ValidatorRequiredPartsExist,
			Status:      StatusFail,
			Message:     "missing: " + strings.Join(missing, ", "),
			Refs:        missing,
		})
	}

	// core.partHashes.match
	verifyPartHashes(pkg, manifest, add)

	// core.workproduct.schemaValid
	addDocSchemaResult(pkg, add, ValidatorWorkproductSchemaValid, WorkproductPath, schema.Workproduct,
		"workproduct.json missing or unreadable")

	// core.claims.schemaValid
	claims, claimsOk := collectionItems(pkg, ClaimsPath, "claims")
	if !claimsOk {
		add(ValidationResult{
			ValidatorId: ValidatorClaimsSchemaValid,
			Status:      StatusFail,
			Message:     "claims/claims.json missing or unreadable",
		})
	} else {
		addCollectionSchemaResult(add, ValidatorClaimsSchemaValid, claims, schema.Claim, "claim")
	}

	// core.provenance.schemaValid
	addDocSchemaResult(pkg, add, ValidatorProvenanceSchemaValid, ProvenancePath, schema.Provenance,
		"provenance/provenance.json missing or unreadable")

	// Optional collections: schema-check when present.
	evidence, evidenceOk := collectionItems(pkg, EvidenceIndex, "evidence")
	if evidenceOk {
		addCollectionSchemaResult(add, ValidatorEvidenceSchemaValid, evidence, schema.EvidenceSchema, "evidence")
	}
	facts, factsOk := collectionItems(pkg, FactsPath, "facts")
	if factsOk {
		addCollectionSchemaResult(add, ValidatorFactsSchemaValid, facts, schema.Fact, "fact")
	}
	judgments, judgmentsOk := collectionItems(pkg, JudgmentsPath, "judgments")
	if judgmentsOk {
		addCollectionSchemaResult(add, ValidatorJudgmentsSchemaValid, judgments, schema.Judgment, "judgment")
	}
	if approvals, ok := collectionItems(pkg, ApprovalsPath, "approvals"); ok {
		addCollectionSchemaResult(add, ValidatorApprovalsSchemaValid, approvals, schema.Approval, "approval")
	}
	if validations, ok := collectionItems(pkg, ValidationsPath, "validations"); ok {
		addCollectionSchemaResult(add, ValidatorValidationsSchemaValid, validations, schema.Validation, "validation")
	}

	// core.attestations.schemaValid — attestations are optional parts under
	// attestations/. Each is schema-checked; beyond that, attestation content
	// never affects package validity.
	verifyAttestations(pkg, add)

	// core.evidenceRefs.resolve
	if evidenceOk {
		verifyEvidenceRefs(pkg, add, evidence, facts, claims)
	}

	// core.externalRefs.wellFormed — offline shape check of externalRef
	// retrieval/auth metadata. Deterministic and network-free.
	if evidenceOk {
		verifyExternalRefsWellFormed(add, evidence)
	}

	// core.factAnchors.quotesMatch
	if factsOk && evidenceOk {
		verifyFactAnchors(pkg, add, facts, evidence)
	}

	// core.claimRefs.resolve
	if claimsOk {
		verifyClaimRefs(pkg, add, claims, facts, judgments)
	}

	// core.graph.refsResolve, core.graph.acyclic, core.graph.declaredInputs,
	// core.digests.match
	graph, digests := verifyGraph(pkg, manifest, add)

	// core.signatures.present / core.signatures.valid
	for _, r := range VerifySignatures(pkg, cfg.trustedKeys) {
		add(r)
	}

	// narrative.* run only when the package carries a narrative part.
	verifyNarrative(pkg, graph, digests, add, evidence, facts, claims, judgments)

	// approvals.current
	verifyApprovalsCurrent(pkg, graph, digests, add)

	// Profile validators.
	if manifest.Profile == ProfileFinance {
		financeValidators(add, itemMaps(claims), itemMaps(judgments))
	}

	// Extension validators.
	if manifest.Extension != "" && cfg.extensions != nil {
		if ext, ok := cfg.extensions[manifest.Extension]; ok {
			ctx := &ExtensionContext{
				Manifest:  manifest,
				Evidence:  itemMaps(evidence),
				Facts:     itemMaps(facts),
				Claims:    itemMaps(claims),
				Judgments: itemMaps(judgments),
			}
			for _, r := range ext(pkg, ctx) {
				add(r)
			}
		}
	}

	return finish()
}

func schemaResult(validatorId string, errs []string) ValidationResult {
	if len(errs) == 0 {
		return ValidationResult{ValidatorId: validatorId, Status: StatusPass}
	}
	return ValidationResult{ValidatorId: validatorId, Status: StatusFail, Message: joinCapped(errs, 5)}
}

// addDocSchemaResult schema-checks a single JSON document part, failing with
// missingMsg when the part is absent or unreadable.
func addDocSchemaResult(pkg Source, add func(ValidationResult), validatorId, relPath, schemaName, missingMsg string) {
	var doc any
	if err := ReadJSON(pkg, relPath, &doc); err != nil {
		add(ValidationResult{ValidatorId: validatorId, Status: StatusFail, Message: missingMsg})
		return
	}
	errs, err := schema.Validate(schemaName, doc)
	if err != nil {
		errs = []string{err.Error()}
	}
	add(schemaResult(validatorId, errs))
}

// collectionItems reads a `{key: [...]}` collection part. ok is false when
// the part is missing or not valid JSON; a present document without the key
// (or with a non-array value) yields an empty, ok collection — both matching
// the reference CLI.
func collectionItems(pkg Source, relPath, key string) ([]any, bool) {
	if !pkg.Has(relPath) {
		return nil, false
	}
	data, err := pkg.Read(relPath)
	if err != nil {
		return nil, false
	}
	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, false
	}
	obj, _ := doc.(map[string]any)
	items, _ := obj[key].([]any)
	if items == nil {
		items = []any{}
	}
	return items, true
}

func addCollectionSchemaResult(add func(ValidationResult), validatorId string, items []any, schemaName, label string) {
	var errs []string
	for i, item := range items {
		itemErrs, err := schema.Validate(schemaName, item)
		if err != nil {
			itemErrs = []string{err.Error()}
		}
		id := getString(asMap(item), "id")
		if id == "" {
			id = strconv.Itoa(i)
		}
		for _, e := range itemErrs {
			errs = append(errs, fmt.Sprintf("%s[%s]: %s", label, id, e))
		}
	}
	add(schemaResult(validatorId, errs))
}

// VerifySignatures reports whether the package carries a signature record
// and, when the verifier was handed trusted keys, whether each record is a
// valid signature over the manifest it ships with. A record under an unknown
// key is reported, not failed: the package may be signed by a party this
// verifier does not know. A record that no longer covers the manifest is
// stale (a warning: something changed since it was signed); one that does
// not verify at all is a failure.
//
// Exported on its own because the outcome depends on the verifier's keys,
// not on the package: a stored validation report records what the writer
// knew, and a reader with the published key overlays the live outcome.
func VerifySignatures(pkg Source, trusted map[string]ed25519.PublicKey) []ValidationResult {
	var out []ValidationResult
	add := func(r ValidationResult) { out = append(out, r) }
	var records []string
	for _, p := range pkg.List() {
		if strings.HasPrefix(p, SignaturesDir) {
			records = append(records, p)
		}
	}
	if len(records) == 0 {
		add(ValidationResult{ValidatorId: ValidatorSignaturesPresent, Status: StatusWarning, Message: "no package signature found"})
		return out
	}
	add(ValidationResult{ValidatorId: ValidatorSignaturesPresent, Status: StatusPass, Refs: records})
	if len(trusted) == 0 {
		return out
	}
	manifestBytes, err := pkg.Read(ManifestPath)
	if err != nil {
		add(ValidationResult{ValidatorId: ValidatorSignaturesValid, Status: StatusFail, Message: "failed to read manifest: " + err.Error()})
		return out
	}
	for _, p := range records {
		data, err := pkg.Read(p)
		if err != nil {
			add(ValidationResult{ValidatorId: ValidatorSignaturesValid, Status: StatusFail, Message: "failed to read " + p + ": " + err.Error(), Refs: []string{p}})
			continue
		}
		sig, err := ParseSignature(data)
		if err != nil {
			add(ValidationResult{ValidatorId: ValidatorSignaturesValid, Status: StatusFail, Message: err.Error(), Refs: []string{p}})
			continue
		}
		pub, ok := trusted[sig.Signer.KeyId]
		if !ok {
			add(ValidationResult{ValidatorId: ValidatorSignaturesValid, Status: StatusWarning, Message: "signed with a key this verifier does not trust: " + sig.Signer.KeyId, Refs: []string{p}})
			continue
		}
		switch err := sig.Verify(pub, manifestBytes); {
		case err == nil:
			add(ValidationResult{ValidatorId: ValidatorSignaturesValid, Status: StatusPass, Message: "signed by " + sig.Signer.Name + " (" + sig.Signer.KeyId + ")", Refs: []string{p}})
		case errors.Is(err, ErrSignatureStale):
			add(ValidationResult{ValidatorId: ValidatorSignaturesValid, Status: StatusWarning, Message: "the package changed after " + sig.Signer.Name + " signed it", Refs: []string{p}})
		default:
			add(ValidationResult{ValidatorId: ValidatorSignaturesValid, Status: StatusFail, Message: err.Error(), Refs: []string{p}})
		}
	}
	return out
}

func verifyPartHashes(pkg Source, manifest *Manifest, add func(ValidationResult)) {
	var problems, refs []string
	declaredHashes := 0
	for _, part := range manifest.AllPartRefs() {
		if !pkg.Has(part.Path) {
			problems = append(problems, "declared part not found: "+part.Path)
			refs = append(refs, part.Path)
			continue
		}
		if part.SHA256 == "" {
			continue
		}
		declaredHashes++
		data, err := pkg.Read(part.Path)
		if err != nil || SHA256Hex(data) != part.SHA256 {
			problems = append(problems, "hash mismatch: "+part.Path)
			refs = append(refs, part.Path)
		}
	}
	switch {
	case len(problems) > 0:
		add(ValidationResult{
			ValidatorId: ValidatorPartHashesMatch,
			Status:      StatusFail,
			Message:     strings.Join(problems, "; "),
			Refs:        refs,
		})
	case declaredHashes == 0:
		add(ValidationResult{
			ValidatorId: ValidatorPartHashesMatch,
			Status:      StatusWarning,
			Message:     "no part hashes declared in manifest",
		})
	default:
		add(ValidationResult{ValidatorId: ValidatorPartHashesMatch, Status: StatusPass})
	}
}

func verifyAttestations(pkg Source, add func(ValidationResult)) {
	var attestationPaths []string
	for _, p := range pkg.List() {
		if attestationPathRe.MatchString(p) {
			attestationPaths = append(attestationPaths, p)
		}
	}
	if len(attestationPaths) == 0 {
		return
	}
	var errs []string
	for _, attPath := range attestationPaths {
		var doc any
		if err := ReadJSON(pkg, attPath, &doc); err != nil {
			errs = append(errs, attPath+": not valid JSON")
			continue
		}
		attErrs, err := schema.Validate(schema.AnalysisAttestation, doc)
		if err != nil {
			attErrs = []string{err.Error()}
		}
		for _, e := range attErrs {
			errs = append(errs, attPath+": "+e)
		}
	}
	if len(errs) == 0 {
		add(ValidationResult{
			ValidatorId: ValidatorAttestationsSchemaOK,
			Status:      StatusPass,
			Message:     fmt.Sprintf("%d %s schema-checked", len(attestationPaths), plural(len(attestationPaths), "attestation", "attestations")),
			Refs:        attestationPaths,
		})
	} else {
		add(ValidationResult{ValidatorId: ValidatorAttestationsSchemaOK, Status: StatusFail, Message: joinCapped(errs, 5)})
	}
}

// refEvidenceId extracts the evidence id from a fact evidenceRef, which is a
// bare string or an object with an evidenceId field. Empty means unresolvable.
func refEvidenceId(ref any) string {
	if s, ok := ref.(string); ok {
		return s
	}
	if m, ok := ref.(map[string]any); ok {
		return getString(m, "evidenceId")
	}
	return ""
}

func verifyEvidenceRefs(pkg Source, add func(ValidationResult), evidence, facts, claims []any) {
	evidenceIds := idSet(evidence)
	var problems []string
	for _, item := range evidence {
		ev := asMap(item)
		id := getString(ev, "id")
		if path := getString(ev, "path"); path != "" {
			if !pkg.Has(path) {
				problems = append(problems, fmt.Sprintf("evidence %s: file not found: %s", id, path))
			} else if declared := getString(ev, "sha256"); declared != "" {
				data, err := pkg.Read(path)
				if err != nil || SHA256Hex(data) != declared {
					problems = append(problems, fmt.Sprintf("evidence %s: hash mismatch: %s", id, path))
				}
			}
		}
	}
	for _, item := range evidence {
		ev := asMap(item)
		if derived := getString(ev, "derivedFrom"); derived != "" && !evidenceIds[derived] {
			problems = append(problems, fmt.Sprintf("evidence %s: unknown derivedFrom ref %s", getString(ev, "id"), derived))
		}
	}
	for _, item := range facts {
		fact := asMap(item)
		for _, ref := range getSlice(fact, "evidenceRefs") {
			evidenceId := refEvidenceId(ref)
			if evidenceId == "" || !evidenceIds[evidenceId] {
				shown := evidenceId
				if shown == "" {
					raw, _ := json.Marshal(ref)
					shown = string(raw)
				}
				problems = append(problems, fmt.Sprintf("fact %s: unknown evidence ref %s", getString(fact, "id"), shown))
			}
		}
	}
	for _, item := range claims {
		claim := asMap(item)
		for _, ref := range getSlice(claim, "supportingEvidence") {
			s, ok := ref.(string)
			if !ok || !evidenceIds[s] {
				problems = append(problems, fmt.Sprintf("claim %s: unknown evidence ref %v", getString(claim, "id"), ref))
			}
		}
	}
	if len(problems) == 0 {
		add(ValidationResult{ValidatorId: ValidatorEvidenceRefsResolve, Status: StatusPass})
	} else {
		add(ValidationResult{ValidatorId: ValidatorEvidenceRefsResolve, Status: StatusFail, Message: joinCapped(problems, 5)})
	}
}

// textEvidenceExtensions are the suffixes treated as machine-checkable text
// evidence for quote anchors.
var textEvidenceExtensions = []string{".txt", ".md", ".csv", ".json"}

func isTextEvidencePath(p string) bool {
	lower := strings.ToLower(p)
	for _, ext := range textEvidenceExtensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// quoteWhitespaceRe matches the whitespace class JavaScript's \s matches
// (ASCII whitespace, NBSP, Unicode space separators, line/paragraph
// separators, and the BOM), so normalization agrees with the reference CLI.
var quoteWhitespaceRe = regexp.MustCompile(`[\t\n\v\f\r \x{00A0}\x{1680}\x{2000}-\x{200A}\x{2028}\x{2029}\x{202F}\x{205F}\x{3000}\x{FEFF}]+`)

var quotePunctReplacer = strings.NewReplacer(
	"‘", "'", "’", "'", "‚", "'",
	"“", `"`, "”", `"`, "„", `"`,
	"–", "-", "—", "-",
)

// NormalizeForQuoteMatch applies the normalization from spec/facts.md to both
// quote and evidence text before matching: NFC, ASCII quotes/dashes,
// collapsed whitespace.
func NormalizeForQuoteMatch(text string) string {
	s := norm.NFC.String(text)
	s = quotePunctReplacer.Replace(s)
	s = quoteWhitespaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func verifyFactAnchors(pkg Source, add func(ValidationResult), facts, evidence []any) {
	evidenceById := map[string]map[string]any{}
	for _, item := range evidence {
		ev := asMap(item)
		if id := getString(ev, "id"); id != "" {
			evidenceById[id] = ev
		}
	}
	var mismatches, uncheckable []string
	checked := 0
	for _, item := range facts {
		fact := asMap(item)
		for _, ref := range getSlice(fact, "evidenceRefs") {
			refMap, _ := ref.(map[string]any)
			// One anchor per evidence chunk; every quote in the ref's
			// anchors array is verified independently.
			for _, anchorItem := range getSlice(refMap, "anchors") {
				quote := getString(asMap(anchorItem), "quote")
				if quote == "" {
					continue
				}
				ev := evidenceById[refEvidenceId(ref)]
				evPath := getString(ev, "path")
				if evPath == "" || !pkg.Has(evPath) || !isTextEvidencePath(evPath) {
					target := getString(ev, "id")
					if target == "" {
						target = refEvidenceId(ref)
					}
					uncheckable = append(uncheckable, fmt.Sprintf("fact %s -> evidence %s", getString(fact, "id"), target))
					continue
				}
				checked++
				data, err := pkg.Read(evPath)
				if err != nil || !strings.Contains(NormalizeForQuoteMatch(string(data)), NormalizeForQuoteMatch(quote)) {
					mismatches = append(mismatches, fmt.Sprintf("fact %s: quote not found in %s", getString(fact, "id"), evPath))
				}
			}
		}
	}
	switch {
	case len(mismatches) > 0:
		add(ValidationResult{ValidatorId: ValidatorFactAnchorsQuotesMatch, Status: StatusFail, Message: joinCapped(mismatches, 5)})
	case len(uncheckable) > 0:
		add(ValidationResult{
			ValidatorId: ValidatorFactAnchorsQuotesMatch,
			Status:      StatusWarning,
			Message: fmt.Sprintf(
				"%d quote anchors verified; not machine-checkable against binary or external evidence (anchor to a derived text layer): %s",
				checked, joinCapped(uncheckable, 3)),
		})
	case checked > 0:
		add(ValidationResult{
			ValidatorId: ValidatorFactAnchorsQuotesMatch,
			Status:      StatusPass,
			Message:     fmt.Sprintf("%d quote anchors verified", checked),
		})
	default:
		add(ValidationResult{ValidatorId: ValidatorFactAnchorsQuotesMatch, Status: StatusNotRun, Message: "no quote anchors declared"})
	}
}

func verifyClaimRefs(pkg Source, add func(ValidationResult), claims, facts, judgments []any) {
	factIds := idSet(facts)
	judgmentIds := idSet(judgments)
	calcIds := calculationIds(pkg)
	var problems []string
	check := func(claim map[string]any, key string, ids map[string]bool, kind string) {
		for _, ref := range getSlice(claim, key) {
			s, ok := ref.(string)
			if !ok || !ids[s] {
				problems = append(problems, fmt.Sprintf("claim %s: unknown %s ref %v", getString(claim, "id"), kind, ref))
			}
		}
	}
	for _, item := range claims {
		claim := asMap(item)
		check(claim, "supportingFacts", factIds, "fact")
		check(claim, "judgments", judgmentIds, "judgment")
		check(claim, "supportingCalculations", calcIds, "calculation")
	}
	if len(problems) == 0 {
		add(ValidationResult{ValidatorId: ValidatorClaimRefsResolve, Status: StatusPass})
	} else {
		add(ValidationResult{ValidatorId: ValidatorClaimRefsResolve, Status: StatusFail, Message: joinCapped(problems, 5)})
	}
}

// calculationIds collects the ids of the calculation objects under
// calculations/.
func calculationIds(pkg Source) map[string]bool {
	calcIds := map[string]bool{}
	for _, p := range pkg.List() {
		if !strings.HasPrefix(p, "calculations/") || !strings.HasSuffix(p, ".json") {
			continue
		}
		var doc map[string]any
		if err := ReadJSON(pkg, p, &doc); err != nil {
			continue
		}
		if id := getString(doc, "id"); id != "" {
			calcIds[id] = true
		}
	}
	return calcIds
}

// financeValidators implements the vwp-finance profile: assertion and support
// coverage, judgment review surfacing.
func financeValidators(add func(ValidationResult), claims, judgments []map[string]any) {
	var noAssertions []string
	for _, c := range claims {
		if len(getSlice(c, "assertions")) == 0 {
			noAssertions = append(noAssertions, getString(c, "id"))
		}
	}
	if len(noAssertions) == 0 {
		add(ValidationResult{ValidatorId: ValidatorFinanceClaimsHaveAssertions, Status: StatusPass})
	} else {
		add(ValidationResult{
			ValidatorId: ValidatorFinanceClaimsHaveAssertions,
			Status:      StatusWarning,
			Message:     "claims without assertions: " + strings.Join(noAssertions, ", "),
			Refs:        noAssertions,
		})
	}

	var unsupported []string
	for _, c := range claims {
		if len(getSlice(c, "supportingFacts")) == 0 &&
			len(getSlice(c, "supportingEvidence")) == 0 &&
			len(getSlice(c, "supportingCalculations")) == 0 &&
			len(getSlice(c, "judgments")) == 0 {
			unsupported = append(unsupported, getString(c, "id"))
		}
	}
	if len(unsupported) == 0 {
		add(ValidationResult{ValidatorId: ValidatorFinanceClaimsHaveSupport, Status: StatusPass})
	} else {
		add(ValidationResult{
			ValidatorId: ValidatorFinanceClaimsHaveSupport,
			Status:      StatusWarning,
			Message:     "claims without supporting facts, evidence, calculations, or judgments: " + strings.Join(unsupported, ", "),
			Refs:        unsupported,
		})
	}

	var open []string
	for _, j := range judgments {
		if s := getString(j, "status"); s == "reviewRequired" || s == "open" {
			open = append(open, getString(j, "id"))
		}
	}
	if len(open) == 0 {
		add(ValidationResult{ValidatorId: ValidatorFinanceRequiredJudgmentsDone, Status: StatusPass})
	} else {
		verb := "require"
		if len(open) == 1 {
			verb = "requires"
		}
		add(ValidationResult{
			ValidatorId: ValidatorFinanceRequiredJudgmentsDone,
			Status:      StatusReviewRequired,
			Message: fmt.Sprintf("%d %s %s review: %s",
				len(open), plural(len(open), "judgment", "judgments"), verb, strings.Join(open, ", ")),
			Refs: open,
		})
	}
}

// --- small helpers shared with the validators package ---

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func itemMaps(items []any) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func getString(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

func getSlice(m map[string]any, key string) []any {
	s, _ := m[key].([]any)
	return s
}

func idSet(items []any) map[string]bool {
	ids := map[string]bool{}
	for _, item := range items {
		if id := getString(asMap(item), "id"); id != "" {
			ids[id] = true
		}
	}
	return ids
}

func joinCapped(items []string, limit int) string {
	if len(items) > limit {
		items = items[:limit]
	}
	return strings.Join(items, "; ")
}

func plural(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

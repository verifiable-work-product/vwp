package validators

import (
	"fmt"
	"strconv"
	"strings"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/schema"
)

// Stablecoin extension validator ids.
const (
	ValidatorStablecoinRegisterPresent          = "stablecoin.holdings.registerPresent"
	ValidatorStablecoinSchemaValid              = "stablecoin.holdings.schemaValid"
	ValidatorStablecoinJudgmentLinked           = "stablecoin.classification.judgmentLinked"
	ValidatorStablecoinRedemptionEvidence       = "stablecoin.redemption.evidence"
	ValidatorStablecoinClassificationConsistent = "stablecoin.classification.consistent"
	ValidatorStablecoinCashEquivalentChallenged = "stablecoin.classification.cashEquivalentChallenged"
)

const (
	stablecoinHoldingsPath      = "facts/stablecoin-holdings.json"
	stablecoinHoldingsSchemaRef = "extensions/digital-assets-stablecoin/stablecoin-holdings.schema.json"
)

// cashLikeClassifications are the classifications that only hold if the
// reporting entity has an evidenced right to redeem at par.
var cashLikeClassifications = map[string]bool{"cash": true, "cashEquivalent": true, "financialAssetReceivable": true}

// Stablecoin implements the vwp-digital-assets-stablecoin extension
// validators over the holdings register: every holding links a resolvable
// classification judgment, cash-like classifications rest on an evidenced
// redemption right, and one token from one issuer is classified the same
// way everywhere it is held.
func Stablecoin(pkg vwp.Source, ctx *vwp.ExtensionContext) []vwp.ValidationResult {
	var results []vwp.ValidationResult
	add := func(r vwp.ValidationResult) { results = append(results, r) }

	registerDoc, present := readJSONValue(pkg, stablecoinHoldingsPath)
	holdings := getMaps(asMap(registerDoc), "holdings")
	if !present || len(holdings) == 0 {
		add(vwp.ValidationResult{ValidatorId: ValidatorStablecoinRegisterPresent, Status: vwp.StatusFail, Message: stablecoinHoldingsPath + " missing or contains no holdings"})
		return results
	}
	add(vwp.ValidationResult{ValidatorId: ValidatorStablecoinRegisterPresent, Status: vwp.StatusPass, Message: strconv.Itoa(len(holdings)) + " holdings in register"})
	errs, err := schema.ValidateExtension(stablecoinHoldingsSchemaRef, registerDoc)
	if err != nil {
		errs = []string{err.Error()}
	}
	if len(errs) > 0 {
		add(vwp.ValidationResult{ValidatorId: ValidatorStablecoinSchemaValid, Status: vwp.StatusFail, Message: joinCapped(errs, 5) + " (remaining stablecoin validators not run)"})
		return results
	}
	add(vwp.ValidationResult{ValidatorId: ValidatorStablecoinSchemaValid, Status: vwp.StatusPass})

	judgmentIds := idSet(ctx.Judgments)
	evidenceIds := idSet(ctx.Evidence)
	var unjudged, unevidenced, inconsistent, cashLike []string
	classBy := map[string]string{}
	for _, h := range holdings {
		id := getString(h, "id")
		if c := getString(h, "classification"); c == "cash" || c == "cashEquivalent" {
			cashLike = append(cashLike, id+" ("+c+")")
		}
		if ref := getString(h, "classificationJudgmentRef"); ref == "" || !judgmentIds[ref] {
			unjudged = append(unjudged, id)
		}
		class := getString(h, "classification")
		if cashLikeClassifications[class] {
			refs, isArray := stringRefs(h, "redemptionTermsEvidence")
			ok := isArray && len(refs) > 0 && getString(h, "redemptionRight") != "none"
			for _, r := range refs {
				if !evidenceIds[r] {
					ok = false
				}
			}
			if !ok {
				unevidenced = append(unevidenced, id)
			}
		}
		key := strings.ToLower(getString(h, "token")) + "@" + strings.ToLower(getString(h, "issuer"))
		if prev, seen := classBy[key]; seen && prev != class {
			inconsistent = append(inconsistent, fmt.Sprintf("%s (%s vs %s)", getString(h, "token"), prev, class))
		} else {
			classBy[key] = class
		}
	}
	failList(add, ValidatorStablecoinJudgmentLinked,
		"every holding links a resolvable classification judgment",
		"holdings without a resolvable classificationJudgmentRef: ", unjudged)
	failList(add, ValidatorStablecoinRedemptionEvidence,
		"cash-like classifications rest on an evidenced redemption right",
		"cash-like holdings without a redemption right backed by resolvable evidence: ", unevidenced)
	// A stablecoin has no maturity, so it cannot meet the ASC 230-10-20
	// cash equivalent definition on the usual reading, and it is not a
	// demand deposit; the AICPA Digital Assets Practice Aid view is that
	// stablecoins are generally not cash or cash equivalents. A package
	// that concludes otherwise is not wrong by construction, but the
	// conclusion needs a human to own it: surface it for review rather
	// than pass it silently.
	if len(cashLike) == 0 {
		add(vwp.ValidationResult{ValidatorId: ValidatorStablecoinCashEquivalentChallenged, Status: vwp.StatusPass,
			Message: "no holding is classified as cash or a cash equivalent"})
	} else {
		add(vwp.ValidationResult{ValidatorId: ValidatorStablecoinCashEquivalentChallenged, Status: vwp.StatusReviewRequired,
			Message: "holdings classified as cash or cash equivalent despite having no maturity (AICPA Digital Assets Practice Aid: generally not cash equivalents) — the classification judgment must address the maturity test and be dispositioned by a reviewer: " + joinCapped(cashLike, 8), Refs: cashLike})
	}
	if len(inconsistent) == 0 {
		add(vwp.ValidationResult{ValidatorId: ValidatorStablecoinClassificationConsistent, Status: vwp.StatusPass,
			Message: strconv.Itoa(len(classBy)) + " token/issuer pairs classified consistently"})
	} else {
		add(vwp.ValidationResult{ValidatorId: ValidatorStablecoinClassificationConsistent, Status: vwp.StatusFail,
			Message: "same token and issuer classified differently: " + joinCapped(inconsistent, 5)})
	}
	return results
}

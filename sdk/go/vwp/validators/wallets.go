package validators

import (
	"strconv"
	"strings"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/schema"
)

// Digital-asset wallet ownership extension validator ids.
const (
	ValidatorWalletsRegisterPresent    = "digitalAssets.wallets.registerPresent"
	ValidatorWalletsSchemaValid        = "digitalAssets.wallets.schemaValid"
	ValidatorWalletsEntityMapping      = "digitalAssets.wallets.entityMapping"
	ValidatorWalletsProofOfControl     = "digitalAssets.wallets.proofOfControl"
	ValidatorWalletsExclusionRationale = "digitalAssets.wallets.exclusionRationale"
	ValidatorWalletsBalanceEvidence    = "digitalAssets.wallets.balanceEvidence"
)

// Data contract paths consumed by the wallet ownership validators.
const (
	walletRegisterPath      = "facts/wallets.json"
	walletRegisterSchemaRef = "extensions/digital-assets-wallet-ownership-control/wallets.schema.json"
)

// WalletOwnership implements the vwp-digital-assets-wallet-ownership-control
// extension validators, run against the structured wallet register at
// facts/wallets.json.
func WalletOwnership(pkg vwp.Source, ctx *vwp.ExtensionContext) []vwp.ValidationResult {
	var results []vwp.ValidationResult
	add := func(r vwp.ValidationResult) { results = append(results, r) }

	registerDoc, _ := readJSONValue(pkg, walletRegisterPath)
	wallets := getMaps(asMap(registerDoc), "wallets")
	if len(wallets) == 0 {
		add(vwp.ValidationResult{
			ValidatorId: ValidatorWalletsRegisterPresent,
			Status:      vwp.StatusFail,
			Message:     "facts/wallets.json missing or contains no wallets",
		})
		return results
	}
	add(vwp.ValidationResult{ValidatorId: ValidatorWalletsRegisterPresent, Status: vwp.StatusPass, Message: strconv.Itoa(len(wallets)) + " wallets in register"})
	errs, err := schema.ValidateExtension(walletRegisterSchemaRef, registerDoc)
	if err != nil {
		errs = []string{err.Error()}
	}
	if len(errs) > 0 {
		add(vwp.ValidationResult{
			ValidatorId: ValidatorWalletsSchemaValid,
			Status:      vwp.StatusFail,
			Message:     joinCapped(errs, 5) + " (remaining wallet validators not run)",
		})
		return results
	}
	add(vwp.ValidationResult{ValidatorId: ValidatorWalletsSchemaValid, Status: vwp.StatusPass})

	evidenceIds := idSet(ctx.Evidence)
	resolves := func(w map[string]any, key string) bool {
		refs, isArray := stringRefs(w, key)
		if !isArray || len(refs) == 0 {
			return false
		}
		for _, r := range refs {
			if !evidenceIds[r] {
				return false
			}
		}
		return true
	}

	var included, excluded []map[string]any
	for _, w := range wallets {
		if inc, _ := w["included"].(bool); inc {
			included = append(included, w)
		} else {
			excluded = append(excluded, w)
		}
	}

	var noMapping []string
	for _, w := range included {
		if !resolves(w, "entityMappingEvidence") {
			noMapping = append(noMapping, getString(w, "id"))
		}
	}
	if len(noMapping) == 0 {
		add(vwp.ValidationResult{
			ValidatorId: ValidatorWalletsEntityMapping,
			Status:      vwp.StatusPass,
			Message:     strconv.Itoa(len(included)) + " included wallets mapped to entities",
		})
	} else {
		add(vwp.ValidationResult{
			ValidatorId: ValidatorWalletsEntityMapping,
			Status:      vwp.StatusFail,
			Message:     "included wallets without entity mapping evidence: " + strings.Join(noMapping, ", "),
			Refs:        noMapping,
		})
	}

	var selfCustody []map[string]any
	for _, w := range included {
		if getString(w, "custody") == "self" {
			selfCustody = append(selfCustody, w)
		}
	}
	var noProof []string
	for _, w := range selfCustody {
		if !resolves(w, "proofOfControlEvidence") {
			noProof = append(noProof, getString(w, "id"))
		}
	}
	if len(noProof) == 0 {
		add(vwp.ValidationResult{
			ValidatorId: ValidatorWalletsProofOfControl,
			Status:      vwp.StatusPass,
			Message:     strconv.Itoa(len(selfCustody)) + " self-custody wallets have proof-of-control evidence",
		})
	} else {
		add(vwp.ValidationResult{
			ValidatorId: ValidatorWalletsProofOfControl,
			Status:      vwp.StatusFail,
			Message:     "self-custody wallets without proof-of-control evidence: " + strings.Join(noProof, ", "),
			Refs:        noProof,
		})
	}

	var noRationale []string
	for _, w := range excluded {
		if getString(w, "exclusionRationale") == "" {
			noRationale = append(noRationale, getString(w, "id"))
		}
	}
	if len(noRationale) == 0 {
		add(vwp.ValidationResult{
			ValidatorId: ValidatorWalletsExclusionRationale,
			Status:      vwp.StatusPass,
			Message:     strconv.Itoa(len(excluded)) + " excluded wallets have rationale",
		})
	} else {
		add(vwp.ValidationResult{
			ValidatorId: ValidatorWalletsExclusionRationale,
			Status:      vwp.StatusFail,
			Message:     "excluded wallets without exclusion rationale: " + strings.Join(noRationale, ", "),
			Refs:        noRationale,
		})
	}

	var noBalance []string
	for _, w := range included {
		if _, hasBalance := w["periodEndBalance"]; hasBalance && !resolves(w, "balanceEvidence") {
			noBalance = append(noBalance, getString(w, "id"))
		}
	}
	if len(noBalance) == 0 {
		add(vwp.ValidationResult{
			ValidatorId: ValidatorWalletsBalanceEvidence,
			Status:      vwp.StatusPass,
			Message:     "period-end balances tie to source evidence",
		})
	} else {
		add(vwp.ValidationResult{
			ValidatorId: ValidatorWalletsBalanceEvidence,
			Status:      vwp.StatusFail,
			Message:     "period-end balances without source evidence: " + strings.Join(noBalance, ", "),
			Refs:        noBalance,
		})
	}
	return results
}

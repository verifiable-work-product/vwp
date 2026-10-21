package validators

import (
	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
)

// Register paths and schema references for the structured fact registers
// (facts/<name>.json) that extension validators consume. A register is a
// typed collection of facts the preparer writes in one piece — a wallet
// population, a fair value measurement table — rather than one fact at a
// time; the builder tools write it through AddFilePart with role "facts".
const (
	RegisterFairValueMeasurements = "fair-value-measurements"
	RegisterWallets               = "wallets"
	RegisterCryptoAssets          = "crypto-assets"
	RegisterStablecoinHoldings    = "stablecoin-holdings"
)

var registerSchemas = map[string]map[string]string{
	vwp.ExtensionASC820:        {RegisterFairValueMeasurements: fairValueRegisterSchemaRef},
	vwp.ExtensionWalletControl: {RegisterWallets: walletRegisterSchemaRef},
	vwp.ExtensionASC35060:      {RegisterCryptoAssets: cryptoAssetsSchemaRef},
	vwp.ExtensionStablecoin:    {RegisterStablecoinHoldings: stablecoinHoldingsSchemaRef},
}

// RegisterPath is the package part path of a named register.
func RegisterPath(name string) string {
	return "facts/" + name + ".json"
}

// RegisterSchema returns the extension schema (relative to schemas/v0.1) a
// named register must satisfy under the given extension, and whether the
// extension defines such a register at all.
func RegisterSchema(extension, name string) (string, bool) {
	ref, ok := registerSchemas[extension][name]
	return ref, ok
}

// RegisterNames lists the registers an extension defines, sorted.
func RegisterNames(extension string) []string {
	names := make([]string, 0, len(registerSchemas[extension]))
	for n := range registerSchemas[extension] {
		names = append(names, n)
	}
	sortStrings(names)
	return names
}

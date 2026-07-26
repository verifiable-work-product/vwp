// Package vwp is the official Go SDK for the VWP (Verifiable Work Product)
// format: package reading/writing with path-traversal and decompression-bomb
// protection, manifest SHA-256 integrity verification, JSON Schema
// validation, core and profile validators, and (in the builder subpackage)
// an incremental package builder.
//
// Validator ids and semantics match the reference TypeScript CLI and the
// conformance table in spec/validations.md, so reports from different
// verifiers are comparable. The batteries-included verification entry point,
// which also runs the built-in extension validators, is validators.Verify.
package vwp

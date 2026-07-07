package vwp

import "errors"

// ErrNotImplemented is returned by all stub SDK entry points.
var ErrNotImplemented = errors.New("not implemented: see cli/ for the reference verifier (v0.1)")

// VerifyReport is the result of verifying a .vwp package.
type VerifyReport struct {
	PackageID string             `json:"packageId"`
	Manifest  *Manifest          `json:"manifest,omitempty"`
	Results   []ValidationResult `json:"results,omitempty"`
	OK        bool               `json:"ok"`
}

// VerifyPackage verifies a .vwp package from its ZIP bytes (stub).
func VerifyPackage(data []byte) (*VerifyReport, error) {
	return nil, ErrNotImplemented
}

// VerifyPackageFile verifies a .vwp package from a file path (stub).
func VerifyPackageFile(path string) (*VerifyReport, error) {
	return nil, ErrNotImplemented
}

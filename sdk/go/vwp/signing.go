package vwp

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Package signing.
//
// A signature is a detached record (spec/signature.schema.json) over the
// package manifest, which in turn declares the hash of every part — so one
// signature binds the whole package. Because the signature record is itself
// declared in the manifest (manifest.signatures), it cannot sign the final
// manifest bytes: it signs the UNSIGNED manifest, the manifest with its
// `signatures` list removed, re-encoded the way the reference tooling does.
// A verifier rebuilds those bytes from the package it holds, so a signature
// stays valid only while every signed part is unchanged, and goes stale the
// moment a part is added, removed or edited.
//
// The algorithm is Ed25519 ("EdDSA" as a JWS alg), the one asymmetric scheme
// with no parameters to get wrong. The signer publishes its public key under
// a key id; a verifier is handed the keys it trusts (WithTrustedSigningKeys)
// and reports every signature it finds against them.

// Signature algorithm and scope identifiers.
const (
	// SignatureAlgorithmEdDSA is the JWS "alg" value for Ed25519 signatures.
	SignatureAlgorithmEdDSA = "EdDSA"
	// SignatureScopeManifest says the signature covers the unsigned manifest.
	SignatureScopeManifest = "manifest.json"
	// SignaturesDir is where signature records live inside a package.
	SignaturesDir = "signatures/"
	// SignatureMediaType is the media type of a signature record part.
	SignatureMediaType = "application/json"
)

// Signature is a detached signature record over the package manifest, in
// the shape spec/signature.schema.json defines. ManifestSha256 is this
// implementation's addition: the SHA-256 of the unsigned manifest bytes the
// value signs, so a reader can tell a stale signature from a forged one
// without the key.
type Signature struct {
	Id        string          `json:"id"`
	Signer    SignatureSigner `json:"signer"`
	Algorithm string          `json:"algorithm"`
	Created   string          `json:"created"`
	Scope     string          `json:"scope"`
	Value     string          `json:"value"`
	// ManifestSha256 is the lowercase hex SHA-256 of the unsigned manifest
	// bytes the signature covers.
	ManifestSha256 string `json:"manifestSha256"`
}

// SignatureSigner identifies who signed: a display name, an optional stable
// id, and the id of the key used (the "kid" of the published public key).
type SignatureSigner struct {
	Id    string `json:"id,omitempty"`
	Name  string `json:"name"`
	KeyId string `json:"keyId,omitempty"`
}

// Signature verification outcomes.
var (
	// ErrSignatureStale means the signature was valid for an earlier
	// manifest: a part changed since it was made.
	ErrSignatureStale = errors.New("vwp: signature does not cover this manifest")
	// ErrSignatureInvalid means the value does not verify under the key.
	ErrSignatureInvalid = errors.New("vwp: signature is invalid")
	// ErrSignatureUnsupported means the record names an algorithm or scope
	// this verifier does not implement.
	ErrSignatureUnsupported = errors.New("vwp: unsupported signature")
)

// Ed25519KeyId derives the published key id of an Ed25519 public key: the
// first 16 hex characters of its SHA-256, prefixed "ed25519-". Stable across
// deployments that hold the same key, so a verifier can match a record to a
// published key without configuration.
func Ed25519KeyId(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return "ed25519-" + hex.EncodeToString(sum[:8])
}

// SignaturePath is the package path of the signature record for a key id:
// one record per key, replaced on each signing.
func SignaturePath(keyId string) string {
	return SignaturesDir + keyId + ".json"
}

// UnsignedManifest returns the bytes a signature covers: the manifest with
// its `signatures` list removed, re-encoded canonically.
func UnsignedManifest(manifestBytes []byte) ([]byte, error) {
	m, err := ParseManifest(manifestBytes)
	if err != nil {
		return nil, err
	}
	m.Signatures = nil
	return m.Encode()
}

// SignManifest signs the unsigned form of manifestBytes with an Ed25519 key.
// signer names who is signing; its KeyId is filled from the key when empty.
func SignManifest(priv ed25519.PrivateKey, signer SignatureSigner, manifestBytes []byte, now time.Time) (*Signature, error) {
	if len(priv) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("vwp: sign: want an Ed25519 private key, got %d bytes", len(priv))
	}
	if strings.TrimSpace(signer.Name) == "" {
		return nil, errors.New("vwp: sign: a signer name is required")
	}
	unsigned, err := UnsignedManifest(manifestBytes)
	if err != nil {
		return nil, err
	}
	if signer.KeyId == "" {
		signer.KeyId = Ed25519KeyId(priv.Public().(ed25519.PublicKey))
	}
	return &Signature{
		Id:             signer.KeyId + "-" + now.UTC().Format("20060102T150405Z"),
		Signer:         signer,
		Algorithm:      SignatureAlgorithmEdDSA,
		Created:        now.UTC().Format(time.RFC3339),
		Scope:          SignatureScopeManifest,
		Value:          base64.StdEncoding.EncodeToString(ed25519.Sign(priv, unsigned)),
		ManifestSha256: SHA256Hex(unsigned),
	}, nil
}

// ParseSignature decodes a signature record.
func ParseSignature(data []byte) (*Signature, error) {
	var s Signature
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("vwp: parse signature: %w", err)
	}
	if s.Value == "" || s.Algorithm == "" || s.Scope == "" {
		return nil, errors.New("vwp: parse signature: value, algorithm and scope are required")
	}
	return &s, nil
}

// Verify checks the signature against the package's current manifest bytes
// under pub. It distinguishes a signature that no longer covers the manifest
// (ErrSignatureStale) from one that never verified (ErrSignatureInvalid).
func (s *Signature) Verify(pub ed25519.PublicKey, manifestBytes []byte) error {
	if s.Algorithm != SignatureAlgorithmEdDSA || s.Scope != SignatureScopeManifest {
		return fmt.Errorf("%w: algorithm %q over %q", ErrSignatureUnsupported, s.Algorithm, s.Scope)
	}
	if len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("vwp: verify: want an Ed25519 public key, got %d bytes", len(pub))
	}
	unsigned, err := UnsignedManifest(manifestBytes)
	if err != nil {
		return err
	}
	if s.ManifestSha256 != "" && s.ManifestSha256 != SHA256Hex(unsigned) {
		return ErrSignatureStale
	}
	sig, err := base64.StdEncoding.DecodeString(s.Value)
	if err != nil {
		return fmt.Errorf("%w: value is not base64", ErrSignatureInvalid)
	}
	if !ed25519.Verify(pub, unsigned, sig) {
		return ErrSignatureInvalid
	}
	return nil
}

// Encode renders the record as a package part.
func (s *Signature) Encode() ([]byte, error) {
	return EncodeJSON(s)
}

// WithTrustedSigningKeys tells Verify which published keys to check
// signature records against, by key id. With no trusted keys, signatures
// are only reported as present; with them, every record found is verified
// (core.signatures.valid).
func WithTrustedSigningKeys(keys map[string]ed25519.PublicKey) VerifyOption {
	return func(c *verifyConfig) {
		if c.trustedKeys == nil {
			c.trustedKeys = map[string]ed25519.PublicKey{}
		}
		for k, v := range keys {
			c.trustedKeys[k] = v
		}
	}
}

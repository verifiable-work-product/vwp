package vwp_test

import (
	"testing"

	vwp "github.com/verifiable-work-product/vwp/sdk/go/vwp"
	"github.com/verifiable-work-product/vwp/sdk/go/vwp/validators"
)

func TestTokenHintLooksLikeSecret(t *testing.T) {
	cases := map[string]bool{
		"env:X":          false,
		"vault:secret/x": false,
		"oauth:client-1": false,
		"none":           false,
		"sk-live-abc":    true,
		"Bearer abc.def": true,
	}
	for hint, want := range cases {
		if got := vwp.TokenHintLooksLikeSecret(hint); got != want {
			t.Errorf("TokenHintLooksLikeSecret(%q) = %v, want %v", hint, got, want)
		}
	}
}

func TestCollectRetrievalOrigins(t *testing.T) {
	evidence := []any{
		map[string]any{
			"id": "ev_gh",
			"externalRef": map[string]any{
				"locatorType": "url", "locator": "https://raw.githubusercontent.com/a",
				"retrieval": map[string]any{"url": "https://raw.githubusercontent.com/a", "auth": map[string]any{"scheme": "none"}},
			},
		},
		map[string]any{
			"id": "ev_bw1",
			"externalRef": map[string]any{
				"locatorType": "url", "locator": "https://api.bitwave.io/1",
				"retrieval": map[string]any{"url": "https://api.bitwave.io/1", "auth": map[string]any{
					"scheme": "bearer", "tokenHint": "env:BW",
					"discovery": "https://api.bitwave.io/.well-known/oauth-protected-resource",
					"scopes":    []any{"vwp:read"},
				}},
			},
		},
		map[string]any{
			"id": "ev_bw2",
			"externalRef": map[string]any{
				"locatorType": "url", "locator": "https://api.bitwave.io/2",
				"retrieval": map[string]any{"url": "https://api.bitwave.io/2", "auth": map[string]any{"scheme": "bearer", "tokenHint": "env:BW"}},
			},
		},
	}
	origins := vwp.CollectRetrievalOrigins(evidence)
	if len(origins) != 2 {
		t.Fatalf("expected 2 distinct origins, got %d", len(origins))
	}
	var bw *vwp.RetrievalOrigin
	for i := range origins {
		if origins[i].Origin == "https://api.bitwave.io" {
			bw = &origins[i]
		}
	}
	if bw == nil {
		t.Fatal("bitwave origin not collected")
	}
	if !bw.RequiresAuth || len(bw.Schemes) != 1 || bw.Schemes[0] != "bearer" {
		t.Errorf("bitwave origin auth wrong: %+v", bw)
	}
	if bw.Discovery != "https://api.bitwave.io/.well-known/oauth-protected-resource" {
		t.Errorf("discovery not collected: %q", bw.Discovery)
	}
	if len(bw.Scopes) != 1 || bw.Scopes[0] != "vwp:read" {
		t.Errorf("scopes not collected: %v", bw.Scopes)
	}
}

// The DTCC/Canton 606 example carries a public URL ref, a vwp-ref, and a
// bearer ref with a source tokenHint — so wellFormed must PASS offline.
func TestExternalRefsWellFormedOnExample(t *testing.T) {
	report := validators.Verify(openFixture(t, "examples", "dtcc-canton-milestone", "asc606-milestone-consideration", "package"))
	assertStatus(t, report, vwp.ValidatorExternalRefsWellFormed, vwp.StatusPass)
	// Offline verify must not run the online reachability check.
	if _, ok := report.Result(vwp.ValidatorExternalRefsReachable); ok {
		t.Error("reachable check must not run during offline Verify")
	}
}

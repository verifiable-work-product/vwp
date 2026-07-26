package vwp

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
)

// knownAuthSchemes are the auth schemes with defined semantics in v0.1. Other
// non-empty strings are permitted (forward-compatible) but surface a warning.
var knownAuthSchemes = map[string]bool{"bearer": true, "none": true}

// tokenHintSourcePattern matches a tokenHint that is a source *reference*
// (env:/vault:/oauth:/keychain:/prompt:/none) rather than a literal secret.
var tokenHintSourcePattern = regexp.MustCompile(`^(env:|vault:|oauth:|keychain:|prompt:|none$)`)

// TokenHintLooksLikeSecret reports whether a tokenHint appears to embed a
// literal secret rather than a source reference. A literal secret in a
// shareable package is a leak and must fail verification.
func TokenHintLooksLikeSecret(hint string) bool {
	return !tokenHintSourcePattern.MatchString(strings.TrimSpace(hint))
}

func isValidURI(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.Scheme != "" && u.Host != ""
}

// verifyExternalRefsWellFormed implements core.externalRefs.wellFormed: an
// offline, deterministic check of externalRef.retrieval/auth SHAPE only. It
// mirrors the reference CLI: an embedded literal secret is a hard FAIL; a
// malformed URL or unknown auth scheme is a WARNING. It never accesses the
// network.
func verifyExternalRefsWellFormed(add func(ValidationResult), evidence []any) {
	var fails, warns []string
	checked := 0
	for _, item := range evidence {
		ev := asMap(item)
		ref := asMap(ev["externalRef"])
		if ref == nil {
			continue
		}
		id := getString(ev, "id")
		if getString(ref, "locatorType") == "url" {
			if loc := getString(ref, "locator"); loc != "" && !isValidURI(loc) {
				warns = append(warns, fmt.Sprintf("evidence %s: locatorType 'url' but locator is not a valid URI", id))
			}
		}
		r := asMap(ref["retrieval"])
		if r == nil {
			continue
		}
		checked++
		if u := getString(r, "url"); u != "" && !isValidURI(u) {
			warns = append(warns, fmt.Sprintf("evidence %s: retrieval.url is not a valid URI", id))
		}
		if auth := asMap(r["auth"]); auth != nil {
			if scheme := getString(auth, "scheme"); scheme != "" && !knownAuthSchemes[scheme] {
				warns = append(warns, fmt.Sprintf("evidence %s: auth.scheme '%s' is not a v0.1-defined scheme (bearer|none)", id, scheme))
			}
			if hint := getString(auth, "tokenHint"); hint != "" && TokenHintLooksLikeSecret(hint) {
				fails = append(fails, fmt.Sprintf("evidence %s: auth.tokenHint appears to embed a literal secret; use a source reference (env:/vault:/oauth:/keychain:/prompt:)", id))
			}
			if disc := getString(auth, "discovery"); disc != "" && !isValidURI(disc) {
				warns = append(warns, fmt.Sprintf("evidence %s: auth.discovery is not a valid URI", id))
			}
		}
	}
	switch {
	case len(fails) > 0:
		add(ValidationResult{ValidatorId: ValidatorExternalRefsWellFormed, Status: StatusFail, Message: joinCapped(fails, 5)})
	case len(warns) > 0:
		add(ValidationResult{ValidatorId: ValidatorExternalRefsWellFormed, Status: StatusWarning, Message: joinCapped(warns, 5)})
	case checked > 0:
		add(ValidationResult{ValidatorId: ValidatorExternalRefsWellFormed, Status: StatusPass, Message: fmt.Sprintf("%d external retrieval reference(s) well-formed", checked)})
	default:
		add(ValidationResult{ValidatorId: ValidatorExternalRefsWellFormed, Status: StatusNotRun, Message: "no external retrieval metadata declared"})
	}
}

// RetrievalOrigin is one distinct network origin the package wants to reach,
// with the auth it requires. Secrets live NOWHERE in the package — this is
// gathered from the refs so a caller can request a credential per origin, once.
type RetrievalOrigin struct {
	Origin       string
	RequiresAuth bool
	Schemes      []string
	TokenHints   []string
	Discovery    string
	Scopes       []string
}

// CredentialResolver supplies a bearer token for an origin at run time.
// Credentials are gathered "over the top" — the verifier never reads a secret
// from the package.
type CredentialResolver func(RetrievalOrigin) string

func originOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// CollectRetrievalOrigins enumerates the distinct origins the package's
// externalRefs want to reach, and what auth each needs, WITHOUT touching the
// network. This is the "gather all the base URLs, then request secrets for
// them" step a caller runs before the online reachability check.
func CollectRetrievalOrigins(evidence []any) []RetrievalOrigin {
	order := []string{}
	byOrigin := map[string]*RetrievalOrigin{}
	appendUnique := func(list []string, v string) []string {
		for _, x := range list {
			if x == v {
				return list
			}
		}
		return append(list, v)
	}
	for _, item := range evidence {
		ev := asMap(item)
		r := asMap(asMap(ev["externalRef"])["retrieval"])
		if r == nil {
			continue
		}
		u := getString(r, "url")
		origin := originOf(u)
		if origin == "" {
			continue
		}
		auth := asMap(r["auth"])
		scheme := "none"
		if auth != nil {
			if s := getString(auth, "scheme"); s != "" {
				scheme = s
			}
		}
		entry, ok := byOrigin[origin]
		if !ok {
			entry = &RetrievalOrigin{Origin: origin}
			byOrigin[origin] = entry
			order = append(order, origin)
		}
		if scheme != "none" {
			entry.RequiresAuth = true
		}
		entry.Schemes = appendUnique(entry.Schemes, scheme)
		if auth != nil {
			if hint := getString(auth, "tokenHint"); hint != "" {
				entry.TokenHints = appendUnique(entry.TokenHints, hint)
			}
			if disc := getString(auth, "discovery"); disc != "" {
				entry.Discovery = disc
			}
			for _, s := range getSlice(auth, "scopes") {
				if str, ok := s.(string); ok {
					entry.Scopes = appendUnique(entry.Scopes, str)
				}
			}
		}
	}
	out := make([]RetrievalOrigin, 0, len(order))
	for _, o := range order {
		out = append(out, *byOrigin[o])
	}
	return out
}

// EnvCredentialResolver gathers a bearer token per origin from the environment:
// each ref's own 'env:NAME' tokenHint on that origin, then a conventional
// VWP_TOKEN_<HOST> variable. It never reads anything from the package. Non-env
// sources (vault:/oauth:/keychain:/prompt:) are left to a richer resolver
// supplied by the embedding app.
func EnvCredentialResolver(origin RetrievalOrigin) string {
	for _, hint := range origin.TokenHints {
		if strings.HasPrefix(hint, "env:") {
			if v := os.Getenv(strings.TrimPrefix(hint, "env:")); v != "" {
				return v
			}
		}
	}
	u, err := url.Parse(origin.Origin)
	if err != nil {
		return ""
	}
	host := nonAlphaNum.ReplaceAllString(u.Host, "_")
	return os.Getenv("VWP_TOKEN_" + strings.ToUpper(host))
}

var nonAlphaNum = regexp.MustCompile(`[^A-Za-z0-9]`)

// VerifyExternalRefsReachable implements core.externalRefs.reachable: an
// ONLINE, opt-in check. It fetches each externalRef.retrieval.url, supplying a
// Bearer token (RFC 6750) obtained PER ORIGIN from the resolver — credentials
// are gathered over the top, never embedded — and compares contentHash when
// declared. This check is environment-dependent and is never part of default
// offline verification. A nil resolver defaults to EnvCredentialResolver.
func VerifyExternalRefsReachable(pkg Source, resolve CredentialResolver) []ValidationResult {
	if resolve == nil {
		resolve = EnvCredentialResolver
	}
	evidence, ok := collectionItems(pkg, EvidenceIndex, "evidence")
	if !ok {
		return []ValidationResult{{ValidatorId: ValidatorExternalRefsReachable, Status: StatusNotRun, Message: "no evidence index"}}
	}

	var targets []map[string]any
	for _, item := range evidence {
		ev := asMap(item)
		if r := asMap(asMap(ev["externalRef"])["retrieval"]); r != nil && getString(r, "url") != "" {
			targets = append(targets, ev)
		}
	}
	if len(targets) == 0 {
		return []ValidationResult{{ValidatorId: ValidatorExternalRefsReachable, Status: StatusNotRun, Message: "no retrievable external references"}}
	}

	// Step 1: gather origins, resolve one credential per origin up front.
	origins := CollectRetrievalOrigins(evidence)
	tokenByOrigin := map[string]string{}
	for _, o := range origins {
		if o.RequiresAuth {
			tokenByOrigin[o.Origin] = resolve(o)
		}
	}

	// Step 2: fetch each ref, reusing the per-origin credential.
	var problems []string
	okCount := 0
	client := &http.Client{}
	for _, ev := range targets {
		id := getString(ev, "id")
		r := asMap(asMap(ev["externalRef"])["retrieval"])
		u := getString(r, "url")
		method := getString(r, "method")
		if method == "" {
			method = http.MethodGet
		}
		auth := asMap(r["auth"])
		scheme := "none"
		header := "Authorization"
		if auth != nil {
			if s := getString(auth, "scheme"); s != "" {
				scheme = s
			}
			if h := getString(auth, "header"); h != "" {
				header = h
			}
		}
		req, err := http.NewRequest(method, u, nil)
		if err != nil {
			problems = append(problems, fmt.Sprintf("evidence %s: bad request: %v", id, err))
			continue
		}
		if scheme == "bearer" {
			token := tokenByOrigin[originOf(u)]
			if token == "" {
				problems = append(problems, fmt.Sprintf("evidence %s: no bearer credential available for origin %s", id, originOf(u)))
				continue
			}
			req.Header.Set(header, "Bearer "+token)
		}
		res, err := client.Do(req)
		if err != nil {
			problems = append(problems, fmt.Sprintf("evidence %s: fetch failed: %v", id, err))
			continue
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			challenge := res.Header.Get("WWW-Authenticate")
			msg := fmt.Sprintf("evidence %s: %d %s", id, res.StatusCode, http.StatusText(res.StatusCode))
			if challenge != "" {
				msg += fmt.Sprintf(" (%s)", challenge)
			}
			problems = append(problems, msg)
			continue
		}
		if want := getString(r, "contentHash"); want != "" {
			got := SHA256Hex(body)
			if got != want {
				problems = append(problems, fmt.Sprintf("evidence %s: contentHash mismatch (expected %s…, got %s…)", id, truncate(want, 12), truncate(got, 12)))
				continue
			}
		}
		okCount++
	}

	if len(problems) > 0 {
		return []ValidationResult{{
			ValidatorId: ValidatorExternalRefsReachable,
			Status:      StatusFail,
			Message:     fmt.Sprintf("%d/%d reachable across %d origin(s); %s", okCount, len(targets), len(origins), joinCapped(problems, 5)),
		}}
	}
	return []ValidationResult{{
		ValidatorId: ValidatorExternalRefsReachable,
		Status:      StatusPass,
		Message:     fmt.Sprintf("%d/%d external reference(s) reachable across %d origin(s); content-hash verified where declared", okCount, len(targets), len(origins)),
	}}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

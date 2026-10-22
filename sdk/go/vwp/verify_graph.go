package vwp

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// verifyGraph runs the integrity graph validators (spec/integrity.md
// section 7): core.graph.refsResolve, core.graph.acyclic,
// core.graph.declaredInputs and core.digests.match. It returns the graph
// and the recomputed digests for the validators that run later (narrative
// freshness, approvals.current); both are nil when they could not be
// computed.
func verifyGraph(pkg Source, manifest *Manifest, add func(ValidationResult)) (*Graph, *Digests) {
	g, err := BuildGraph(pkg)
	if err != nil {
		message := "failed to build the integrity graph: " + err.Error()
		var dup *DuplicateIdError
		if errors.As(err, &dup) {
			message = dup.Error()
		}
		add(ValidationResult{ValidatorId: ValidatorGraphRefsResolve, Status: StatusFail, Message: message})
		if manifest.Digests != nil {
			add(ValidationResult{ValidatorId: ValidatorDigestsMatch, Status: StatusNotRun, Message: "roots cannot be computed: " + err.Error()})
		} else {
			add(ValidationResult{ValidatorId: ValidatorDigestsMatch, Status: StatusNotRun, Message: "no digests block"})
		}
		return nil, nil
	}

	// core.graph.refsResolve
	if len(g.Unresolved) == 0 {
		add(ValidationResult{ValidatorId: ValidatorGraphRefsResolve, Status: StatusPass})
	} else {
		var problems, refs []string
		seen := map[string]bool{}
		for _, p := range g.Unresolved {
			problems = append(problems, p.String())
			if !seen[p.Id] {
				seen[p.Id] = true
				refs = append(refs, p.Id)
			}
		}
		add(ValidationResult{ValidatorId: ValidatorGraphRefsResolve, Status: StatusFail, Message: joinCapped(problems, 5), Refs: refs})
	}

	// core.graph.acyclic
	if len(g.Cycles) == 0 {
		add(ValidationResult{ValidatorId: ValidatorGraphAcyclic, Status: StatusPass})
	} else {
		var problems, refs []string
		seen := map[string]bool{}
		for _, cycle := range g.Cycles {
			problems = append(problems, strings.Join(cycle, " -> "))
			for _, id := range cycle {
				if !seen[id] {
					seen[id] = true
					refs = append(refs, id)
				}
			}
		}
		sort.Strings(refs)
		add(ValidationResult{ValidatorId: ValidatorGraphAcyclic, Status: StatusFail, Message: "dependency cycles: " + joinCapped(problems, 5), Refs: refs})
	}

	// core.graph.declaredInputs
	verifyDeclaredInputs(g, add)

	// core.digests.match
	digests, derr := g.Digests()
	if manifest.Digests != nil {
		verifyDigestsMatch(manifest.Digests, digests, derr, add)
	} else {
		add(ValidationResult{ValidatorId: ValidatorDigestsMatch, Status: StatusNotRun, Message: "no digests block"})
	}
	if derr != nil {
		return g, nil
	}
	return g, digests
}

// VerifyDigests recomputes a package's integrity roots and compares them to
// the manifest's digests block. It returns nil when they match or when the
// manifest declares no digests.
func VerifyDigests(pkg Source) error {
	raw, err := pkg.Read(ManifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}
	manifest, err := ParseManifest(raw)
	if err != nil {
		return err
	}
	if manifest.Digests == nil {
		return nil
	}
	var result ValidationResult
	digests, derr := ComputeDigests(pkg)
	verifyDigestsMatch(manifest.Digests, digests, derr, func(r ValidationResult) { result = r })
	if result.Status == StatusFail {
		return fmt.Errorf("vwp: digests mismatch: %s", result.Message)
	}
	return nil
}

func verifyDigestsMatch(declared, computed *Digests, computeErr error, add func(ValidationResult)) {
	if declared.Algorithm != DigestAlgorithm {
		add(ValidationResult{
			ValidatorId: ValidatorDigestsMatch,
			Status:      StatusFail,
			Message:     fmt.Sprintf("unknown digests algorithm %q (this verifier implements %s)", declared.Algorithm, DigestAlgorithm),
		})
		return
	}
	if computeErr != nil {
		add(ValidationResult{ValidatorId: ValidatorDigestsMatch, Status: StatusFail, Message: computeErr.Error()})
		return
	}
	var mismatched []string
	if declared.AnalysisRoot != computed.AnalysisRoot {
		mismatched = append(mismatched, "analysisRoot")
	}
	if declared.ContentRoot != computed.ContentRoot {
		mismatched = append(mismatched, "contentRoot")
	}
	if declared.ReviewRoot != computed.ReviewRoot {
		mismatched = append(mismatched, "reviewRoot")
	}
	if declared.Collections != nil {
		var keys []string
		for k := range declared.Collections {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if declared.Collections[k] != computed.Collections[k] {
				mismatched = append(mismatched, "collections."+k)
			}
		}
	}
	if len(mismatched) == 0 {
		add(ValidationResult{ValidatorId: ValidatorDigestsMatch, Status: StatusPass})
		return
	}
	add(ValidationResult{
		ValidatorId: ValidatorDigestsMatch,
		Status:      StatusFail,
		Message:     "manifest digests do not match the recomputed roots: " + strings.Join(mismatched, ", "),
		Refs:        mismatched,
	})
}

// verifyDeclaredInputs warns about inputs an object evidently used but did
// not declare as dependencies: a known fact, calculation or evidence id
// appearing as a string anywhere under a calculation's inputs that is not
// in its dependsOn, and a known fact id mentioned in a judgment's basis
// text that is not in its basisFacts.
func verifyDeclaredInputs(g *Graph, add func(ValidationResult)) {
	inputKinds := map[string]bool{KindFact: true, KindCalculation: true, KindEvidence: true}
	var problems, refs []string
	for _, calc := range g.ObjectsOfKind(KindCalculation) {
		declared := toSet(calc.Deps)
		var missing []string
		seen := map[string]bool{}
		walkStrings(calc.Value["inputs"], func(s string) {
			if s == calc.Id || seen[s] || declared[s] {
				return
			}
			if obj, ok := g.Objects[s]; ok && inputKinds[obj.Kind] {
				seen[s] = true
				missing = append(missing, s)
			}
		})
		if len(missing) > 0 {
			sort.Strings(missing)
			problems = append(problems, fmt.Sprintf("calculation %s: inputs reference %s not declared in dependsOn", calc.Id, strings.Join(missing, ", ")))
			refs = append(refs, calc.Id)
		}
	}
	factIds := make([]string, 0)
	for _, f := range g.ObjectsOfKind(KindFact) {
		factIds = append(factIds, f.Id)
	}
	for _, j := range g.ObjectsOfKind(KindJudgment) {
		basis := getString(j.Value, "basis")
		if basis == "" {
			continue
		}
		declared := toSet(j.Deps)
		var missing []string
		for _, id := range factIds {
			if !declared[id] && mentionsToken(basis, id) {
				missing = append(missing, id)
			}
		}
		if len(missing) > 0 {
			problems = append(problems, fmt.Sprintf("judgment %s: basis mentions %s not declared in basisFacts", j.Id, strings.Join(missing, ", ")))
			refs = append(refs, j.Id)
		}
	}
	if len(problems) == 0 {
		add(ValidationResult{ValidatorId: ValidatorGraphDeclaredInputs, Status: StatusPass})
		return
	}
	add(ValidationResult{ValidatorId: ValidatorGraphDeclaredInputs, Status: StatusWarning, Message: joinCapped(problems, 5), Refs: refs})
}

// isIdRune reports whether r can be part of an id token: a mention of an
// id ends at the first rune outside [A-Za-z0-9_-], so "s2_f10" is not a
// mention of "s2_f1".
func isIdRune(r rune) bool {
	return r == '_' || r == '-' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// mentionsToken reports whether text contains id as a whole token.
func mentionsToken(text, id string) bool {
	if id == "" {
		return false
	}
	for start := 0; ; {
		i := strings.Index(text[start:], id)
		if i < 0 {
			return false
		}
		i += start
		end := i + len(id)
		before := i == 0 || !isIdRune(lastRune(text[:i]))
		after := end == len(text) || !isIdRune(firstRune(text[end:]))
		if before && after {
			return true
		}
		start = i + 1
	}
}

func firstRune(s string) rune {
	for _, r := range s {
		return r
	}
	return 0
}

func lastRune(s string) rune {
	var last rune
	for _, r := range s {
		last = r
	}
	return last
}

// walkStrings visits every string value nested anywhere under v.
func walkStrings(v any, visit func(string)) {
	switch t := v.(type) {
	case string:
		visit(t)
	case []any:
		for _, item := range t {
			walkStrings(item, visit)
		}
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			walkStrings(t[k], visit)
		}
	}
}

func toSet(items []string) map[string]bool {
	out := make(map[string]bool, len(items))
	for _, s := range items {
		out[s] = true
	}
	return out
}

// verifyApprovalsCurrent reports approved actions whose recorded
// contentRoot is not the current content root: the content they approved
// was superseded. It emits nothing when the package has no approvals part;
// notRun when no approval carries a contentRoot or the roots could not be
// computed.
func verifyApprovalsCurrent(pkg Source, g *Graph, digests *Digests, add func(ValidationResult)) {
	if !pkg.Has(ApprovalsPath) {
		return
	}
	notRun := func(why string) {
		add(ValidationResult{ValidatorId: ValidatorApprovalsCurrent, Status: StatusNotRun, Message: why})
	}
	if g == nil {
		notRun("integrity graph could not be built")
		return
	}
	stamped := 0
	current := 0
	var superseded []string
	for _, a := range g.ObjectsOfKind(KindApproval) {
		root := getString(a.Value, "contentRoot")
		if root == "" {
			continue
		}
		stamped++
		if digests == nil || getString(a.Value, "action") != ApprovalActionApproved {
			continue
		}
		if root == digests.ContentRoot {
			current++
		} else {
			superseded = append(superseded, a.Id)
		}
	}
	if stamped == 0 {
		notRun("no approval carries a contentRoot")
		return
	}
	if digests == nil {
		notRun("roots cannot be computed (see core.graph.refsResolve and core.graph.acyclic)")
		return
	}
	// Earlier approvals superseded by a later one over the current content
	// are history, not a problem: the package has a current approval.
	if current > 0 {
		res := ValidationResult{ValidatorId: ValidatorApprovalsCurrent, Status: StatusPass}
		if len(superseded) > 0 {
			res.Message = "earlier approvals superseded by a current one: " + strings.Join(superseded, ", ")
			res.Refs = superseded
		}
		add(res)
		return
	}
	if len(superseded) == 0 {
		add(ValidationResult{ValidatorId: ValidatorApprovalsCurrent, Status: StatusPass})
		return
	}
	add(ValidationResult{
		ValidatorId: ValidatorApprovalsCurrent,
		Status:      StatusReviewRequired,
		Message:     "the content changed after these approvals were recorded; they are superseded and a new approval is required: " + strings.Join(superseded, ", "),
		Refs:        superseded,
	})
}

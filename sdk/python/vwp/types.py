"""Dataclasses mirroring the VWP core objects.

Status: stub / preview. The normative definitions are the JSON Schemas in
schemas/; if these types and the schemas disagree, the schemas win. Dates are
represented as ISO 8601 strings.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Optional


@dataclass
class PackagePart:
    path: str
    media_type: str
    sha256: Optional[str] = None
    role: Optional[str] = None


@dataclass
class Provenance:
    created_at: str
    created_by: str
    toolchain: Optional[dict[str, Any]] = None
    models: Optional[dict[str, Any]] = None
    policy_versions: Optional[dict[str, Any]] = None
    source_systems: Optional[list[str]] = None
    environment: Optional[dict[str, Any]] = None
    notes: Optional[str] = None


@dataclass
class Manifest:
    vwp_version: str
    package_id: str
    created_at: str
    profile: str
    workproduct: str
    provenance: Provenance
    extension: Optional[str] = None
    subject: Optional[dict[str, Any]] = None
    parts: Optional[list[PackagePart]] = None
    signatures: Optional[list[Any]] = None


@dataclass
class WorkProduct:
    id: str
    title: str
    domain: str
    profile: str
    status: str
    extension: Optional[str] = None
    standard_refs: Optional[list[str]] = None
    entity: Optional[str] = None
    period: Optional[str] = None
    subject: Optional[dict[str, Any]] = None
    conclusion_summary: Optional[str] = None
    created_by: Optional[str] = None
    review_state: Optional[str] = None


@dataclass
class Evidence:
    id: str
    type: str
    title: str
    source_system: Optional[str] = None
    path: Optional[str] = None
    external_ref: Optional[str] = None
    sha256: Optional[str] = None
    retrieved_at: Optional[str] = None
    period_covered: Optional[str] = None
    access_notes: Optional[str] = None


@dataclass
class Fact:
    id: str
    statement: str
    value: Any = None
    evidence_refs: Optional[list[str]] = None
    extraction_method: Optional[str] = None
    confidence: Optional[float] = None
    requires_review: Optional[bool] = None


@dataclass
class Claim:
    id: str
    statement: str
    domain: Optional[str] = None
    standard_refs: Optional[list[str]] = None
    assertions: Optional[list[str]] = None
    supporting_facts: Optional[list[str]] = None
    supporting_evidence: Optional[list[str]] = None
    supporting_calculations: Optional[list[str]] = None
    judgments: Optional[list[str]] = None
    conclusion: Optional[str] = None
    status: Optional[str] = None
    requires_review: Optional[bool] = None


@dataclass
class Judgment:
    id: str
    topic: str
    question: str
    proposed_conclusion: Optional[str] = None
    basis: Optional[str] = None
    alternatives: Optional[list[Any]] = None
    risk_level: Optional[str] = None
    required_approver_role: Optional[str] = None
    status: Optional[str] = None


@dataclass
class Calculation:
    id: str
    type: str
    inputs: dict[str, Any] = field(default_factory=dict)
    outputs: dict[str, Any] = field(default_factory=dict)
    method: Optional[str] = None
    validator: Optional[str] = None
    recomputable: Optional[bool] = None


#: Allowed validation result statuses.
VALIDATION_STATUSES = ("pass", "fail", "warning", "reviewRequired", "notRun")


@dataclass
class ValidationResult:
    id: str
    validator: str
    status: str  # one of VALIDATION_STATUSES
    scope: Optional[str] = None
    severity: Optional[str] = None
    message: Optional[str] = None
    details: Optional[dict[str, Any]] = None


#: Allowed approval actions.
APPROVAL_ACTIONS = ("approved", "rejected", "requestedChanges", "acknowledged")


@dataclass
class Approval:
    id: str
    approver: str
    action: str  # one of APPROVAL_ACTIONS
    timestamp: str
    role: Optional[str] = None
    scope: Optional[str] = None
    comments: Optional[str] = None

"""VWP package verifier (stub).

Status: planned. See cli/ in the repository root for the working reference
verifier (v0.1).
"""

from __future__ import annotations

from typing import Any


def verify_package(data: bytes, **options: Any) -> Any:
    """Verify a .vwp package from its ZIP bytes (stub)."""
    raise NotImplementedError(
        "Not implemented: see cli/ for the reference verifier (v0.1)"
    )


def verify_package_file(path: str, **options: Any) -> Any:
    """Verify a .vwp package from a file path (stub)."""
    raise NotImplementedError(
        "Not implemented: see cli/ for the reference verifier (v0.1)"
    )

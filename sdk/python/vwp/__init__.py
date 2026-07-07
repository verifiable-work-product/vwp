"""VWP (Verifiable Work Product) Python SDK.

Status: planned stub. See cli/ in the repository root for the working
reference verifier (v0.1).
"""

__version__ = "0.0.0"

from . import types  # noqa: F401
from .verifier import verify_package, verify_package_file  # noqa: F401

from __future__ import annotations

import os
import sys
from dataclasses import dataclass
from pathlib import Path


@dataclass(slots=True)
class AppPaths:
    app_root: Path
    default_profile: Path
    bootstrap_profile: Path
    generated_root: Path
    runtime_root: Path
    runtime_generated_root: Path


def resolve_app_paths() -> AppPaths:
    app_root = _resolve_app_root()
    generated_root = _resolve_generated_root(app_root)
    runtime_root = app_root / "client" / "desktop" / "runtime"
    return AppPaths(
        app_root=app_root,
        default_profile=app_root / "client" / "common" / "profiles" / "reality" / "default.profile.json",
        bootstrap_profile=app_root / "client" / "android" / "app" / "src" / "main" / "secure" / "bootstrap.json",
        generated_root=generated_root,
        runtime_root=runtime_root,
        runtime_generated_root=generated_root / "runtime",
    )


def _resolve_app_root() -> Path:
    if getattr(sys, "frozen", False):
        return Path(sys.executable).resolve().parent

    current_file = Path(__file__).resolve()
    for parent in current_file.parents:
        if (parent / "go.mod").is_file() and (parent / "client").is_dir():
            return parent
    return current_file.parents[4]


def _resolve_generated_root(app_root: Path) -> Path:
    override = os.environ.get("NOVPN_GENERATED_ROOT", "").strip()
    if override:
        return Path(override).expanduser().resolve()

    if getattr(sys, "frozen", False):
        local_app_data = os.environ.get("LOCALAPPDATA", "").strip()
        if local_app_data:
            return Path(local_app_data) / "NoVPN Desktop" / "generated"
        return Path.home() / "AppData" / "Local" / "NoVPN Desktop" / "generated"

    return app_root / "client" / "desktop" / "python" / "generated"

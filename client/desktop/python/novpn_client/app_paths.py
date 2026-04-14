from __future__ import annotations

import os
import sys
from dataclasses import dataclass
from pathlib import Path


@dataclass(slots=True)
class AppPaths:
    app_root: Path
    bundle_root: Path
    default_profile: Path
    bootstrap_profile: Path
    generated_root: Path
    runtime_root: Path
    runtime_generated_root: Path


def resolve_app_paths() -> AppPaths:
    app_root = _resolve_app_root()
    bundle_root = _resolve_bundle_root(app_root)
    generated_root = _resolve_generated_root(app_root)
    runtime_root = _resolve_runtime_root(bundle_root, app_root)
    return AppPaths(
        app_root=app_root,
        bundle_root=bundle_root,
        default_profile=_first_existing_path(
            [
                bundle_root / "client" / "common" / "profiles" / "reality" / "default.profile.json",
                app_root / "client" / "common" / "profiles" / "reality" / "default.profile.json",
            ]
        ),
        bootstrap_profile=_first_existing_path(
            [
                bundle_root / "client" / "android" / "app" / "src" / "main" / "secure" / "bootstrap.json",
                app_root / "client" / "android" / "app" / "src" / "main" / "secure" / "bootstrap.json",
            ]
        ),
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


def _resolve_bundle_root(app_root: Path) -> Path:
    if getattr(sys, "frozen", False):
        meipass = getattr(sys, "_MEIPASS", None)
        if meipass:
            return Path(meipass).resolve()
    return app_root


def _resolve_runtime_root(bundle_root: Path, app_root: Path) -> Path:
    candidates = [
        bundle_root / "client" / "desktop" / "runtime",
        app_root / "client" / "desktop" / "runtime",
        bundle_root / "runtime",
        app_root / "runtime",
    ]
    for candidate in candidates:
        if (candidate / "bin").is_dir():
            return candidate
    return candidates[0]


def _first_existing_path(candidates: list[Path]) -> Path:
    for candidate in candidates:
        if candidate.exists():
            return candidate
    return candidates[0]


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

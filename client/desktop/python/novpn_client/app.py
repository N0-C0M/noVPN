from __future__ import annotations

import argparse
from pathlib import Path

from .app_catalog_service import AppCatalogService
from .config_builder import XrayConfigBuilder
from .models import DesktopSettings
from .profile_store import ProfileStore
from .ui.main_window import MainWindow


def main() -> int:
    root = Path(__file__).resolve().parents[4]
    default_profile = root / "client" / "common" / "profiles" / "reality" / "default.profile.json"
    default_output = root / "client" / "desktop" / "python" / "generated" / "config.json"

    parser = argparse.ArgumentParser(description="NoVPN desktop scaffold")
    parser.add_argument("--profile", type=Path, default=default_profile)
    parser.add_argument("--output", type=Path, default=default_output)
    parser.add_argument("--bypass-ru", action="store_true")
    parser.add_argument("--exclude-app", action="append", default=[])
    parser.add_argument("--headless", action="store_true")
    args = parser.parse_args()

    store = ProfileStore(args.profile)
    builder = XrayConfigBuilder()
    catalog = AppCatalogService()

    if args.headless:
        profile = store.load()
        settings = DesktopSettings(
            bypass_ru=args.bypass_ru,
            excluded_apps=args.exclude_app,
            output_path=args.output,
        )
        output_path = builder.write(profile, settings)
        print(output_path)
        return 0

    window = MainWindow(
        profile_store=store,
        builder=builder,
        catalog=catalog,
        output_path=args.output,
    )
    return window.run()

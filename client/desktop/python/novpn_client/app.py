from __future__ import annotations

import argparse
import signal
import time
from pathlib import Path

from .app_catalog_service import AppCatalogService
from .config_builder import XrayConfigBuilder
from .models import DesktopSettings
from .profile_store import ProfileStore
from .runtime_manager import DesktopRuntimeManager
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
    parser.add_argument("--start-runtime", action="store_true")
    parser.add_argument("--xray-bin", type=Path)
    parser.add_argument("--obfuscator-bin", type=Path)
    args = parser.parse_args()

    store = ProfileStore(args.profile)
    builder = XrayConfigBuilder()
    catalog = AppCatalogService()
    runtime_manager = DesktopRuntimeManager(
        repo_root=root,
        xray_binary=args.xray_bin,
        obfuscator_binary=args.obfuscator_bin,
    )

    if args.headless:
        profile = store.load()
        settings = DesktopSettings(
            bypass_ru=args.bypass_ru,
            excluded_apps=args.exclude_app,
            output_path=args.output,
        )
        output_path = builder.write(profile, settings)
        print(output_path)

        if args.start_runtime:
            status = runtime_manager.start(profile, settings)
            print(status.detail)
            print(f"xray_log={status.xray_log}")
            print(f"obfuscator_log={status.obfuscator_log}")

            keep_running = True

            def handle_signal(_signum, _frame) -> None:
                nonlocal keep_running
                keep_running = False

            signal.signal(signal.SIGINT, handle_signal)
            signal.signal(signal.SIGTERM, handle_signal)

            try:
                while keep_running:
                    time.sleep(0.5)
            finally:
                runtime_manager.stop()

        return 0

    window = MainWindow(
        profile_store=store,
        builder=builder,
        catalog=catalog,
        output_path=args.output,
        runtime_manager=runtime_manager,
    )
    return window.run()

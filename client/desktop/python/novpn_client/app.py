from __future__ import annotations

import argparse
import signal
import sys
import time
from pathlib import Path

from .app_paths import resolve_app_paths
from .app_catalog_service import AppCatalogService
from .config_builder import XrayConfigBuilder
from .device_identity_store import DeviceIdentityStore
from .invite_redeemer import InviteRedeemer
from .logger import configure_logging, get_logger
from .models import AppRoutingMode, DesktopSettings, PatternMaskingStrategy, TrafficObfuscationStrategy
from .network_diagnostics import NetworkDiagnosticsRunner
from .profile_store import ProfileStore
from .runtime_manager import DesktopRuntimeManager
from .runtime_preflight import RuntimePreflightChecker
from .state_store import ClientStateStore
from .ui.main_window import MainWindow


def main() -> int:
    paths = resolve_app_paths()
    app_log_path = configure_logging(paths.generated_root / "logs")
    logger = get_logger("app")
    generated_root = paths.generated_root
    imported_profiles_dir = generated_root / "profiles"
    state_path = generated_root / "client_state.json"
    device_identity_path = generated_root / "device_identity.json"
    default_output = generated_root / "config.json"

    parser = argparse.ArgumentParser(description="NoVPN desktop client")
    parser.add_argument("--profile", type=Path, default=paths.default_profile)
    parser.add_argument("--output", type=Path, default=default_output)
    parser.add_argument("--bypass-ru", action="store_true")
    parser.add_argument("--exclude-app", action="append", default=[])
    parser.add_argument("--headless", action="store_true")
    parser.add_argument("--start-runtime", action="store_true")
    parser.add_argument("--xray-bin", type=Path)
    parser.add_argument("--obfuscator-bin", type=Path)
    args = parser.parse_args()

    logger.info(
        "starting desktop client headless=%s start_runtime=%s profile=%s generated_root=%s runtime_root=%s",
        args.headless,
        args.start_runtime,
        args.profile,
        generated_root,
        paths.runtime_root,
    )

    try:
        store = ProfileStore(args.profile, imported_profiles_dir, paths.bootstrap_profile)
        builder = XrayConfigBuilder()
        catalog = AppCatalogService()
        runtime_manager = DesktopRuntimeManager(
            runtime_root=paths.runtime_root,
            generated_root=paths.runtime_generated_root,
            xray_binary=args.xray_bin,
            obfuscator_binary=args.obfuscator_bin,
            logger=get_logger("runtime"),
        )
        state_store = ClientStateStore(state_path)
        device_identity_store = DeviceIdentityStore(device_identity_path)
        invite_redeemer = InviteRedeemer()
        diagnostics_runner = NetworkDiagnosticsRunner()
        preflight_checker = RuntimePreflightChecker(store, runtime_manager.layout)

        if args.headless:
            profile = store.load(args.profile)
            settings = DesktopSettings(
                bypass_ru=args.bypass_ru,
                app_routing_mode=AppRoutingMode.EXCLUDE_SELECTED,
                selected_apps=args.exclude_app,
                traffic_strategy=TrafficObfuscationStrategy.BALANCED,
                pattern_strategy=PatternMaskingStrategy.STEADY,
                device_id=device_identity_store.device_id(),
                output_path=args.output,
            )
            output_path = builder.write(profile, settings)
            logger.info("headless config generated at %s", output_path)
            print(output_path)

            if args.start_runtime:
                status = runtime_manager.start(profile, settings)
                logger.info(
                    "headless runtime started xray_log=%s obfuscator_log=%s",
                    status.xray_log,
                    status.obfuscator_log,
                )
                print(status.detail)
                print(f"app_log={app_log_path}")
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
            state_store=state_store,
            device_identity_store=device_identity_store,
            builder=builder,
            catalog=catalog,
            output_path=args.output,
            runtime_manager=runtime_manager,
            invite_redeemer=invite_redeemer,
            diagnostics_runner=diagnostics_runner,
            preflight_checker=preflight_checker,
            logger=get_logger("ui"),
            app_log_path=app_log_path,
        )
        return window.run()
    except Exception:
        logger.exception("desktop client startup failed")
        if args.headless:
            print("Desktop client startup failed. See log:", app_log_path, file=sys.stderr)
        return 1

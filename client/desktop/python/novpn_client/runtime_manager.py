from __future__ import annotations

import atexit
import os
import subprocess
import time
from io import TextIOWrapper
from pathlib import Path

from .config_builder import XrayConfigBuilder
from .models import ClientProfile, DesktopSettings, RuntimeStatus
from .obfuscator_config_builder import ObfuscatorConfigBuilder
from .runtime_layout import RuntimeLayout
from .session_obfuscation import SessionObfuscationPlanner


class DesktopRuntimeManager:
    def __init__(
        self,
        repo_root: Path,
        xray_binary: Path | None = None,
        obfuscator_binary: Path | None = None,
    ) -> None:
        self._layout = RuntimeLayout.detect(repo_root, xray_binary, obfuscator_binary)
        self._xray_builder = XrayConfigBuilder()
        self._obfuscator_builder = ObfuscatorConfigBuilder()
        self._xray_process: subprocess.Popen[str] | None = None
        self._obfuscator_process: subprocess.Popen[str] | None = None
        self._xray_log_handle: TextIOWrapper | None = None
        self._obfuscator_log_handle: TextIOWrapper | None = None
        atexit.register(self._shutdown_quietly)

    @property
    def layout(self) -> RuntimeLayout:
        return self._layout

    def start(self, profile: ClientProfile, settings: DesktopSettings) -> RuntimeStatus:
        if self._is_running():
            return self.status("Runtime already running")

        self._layout.ensure_directories()
        runtime_settings = DesktopSettings(
            bypass_ru=settings.bypass_ru,
            app_routing_mode=settings.app_routing_mode,
            selected_apps=list(settings.selected_apps),
            traffic_strategy=settings.traffic_strategy,
            pattern_strategy=settings.pattern_strategy,
            device_id=settings.device_id,
            output_path=self._layout.xray_config,
        )
        session_plan = SessionObfuscationPlanner.build(
            profile=profile,
            device_id=settings.device_id or "desktop-runtime",
        )
        self._xray_builder.write(profile, runtime_settings, session_plan)
        self._obfuscator_builder.write(
            profile,
            self._layout.obfuscator_config,
            self._layout.xray_config,
            settings.device_id,
            session_plan,
        )

        self._ensure_binary(self._layout.xray_binary)
        self._ensure_binary(self._layout.obfuscator_binary)

        self._obfuscator_process = self._spawn(
            self._layout.obfuscator_binary,
            ["--config", str(self._layout.obfuscator_config)],
            self._layout.obfuscator_log,
            extra_env={
                "NOVPN_OBFS_SEED": profile.obfuscation.seed,
                "NOVPN_ROLE": "obfuscator",
            },
        )
        self._xray_process = self._spawn(
            self._layout.xray_binary,
            ["run", "-config", str(self._layout.xray_config)],
            self._layout.xray_log,
            extra_env={
                "XRAY_LOCATION_ASSET": str(self._layout.xray_binary.parent),
                "XRAY_LOCATION_CONFIG": str(self._layout.xray_config.parent),
                "NOVPN_ROLE": "xray",
            },
        )

        time.sleep(0.35)
        failed_runtime = self._failed_runtime_process()
        if failed_runtime is not None:
            failed_runtime_name, log_path = failed_runtime
            self.stop()
            log_excerpt = self._read_log_excerpt(log_path)
            detail = f"{failed_runtime_name} exited immediately."
            if log_excerpt:
                detail += f" Last log lines: {log_excerpt}"
            raise RuntimeError(detail)
        return self.status("Runtime started")

    def stop(self) -> RuntimeStatus:
        self._stop_process(self._xray_process)
        self._stop_process(self._obfuscator_process)
        self._xray_process = None
        self._obfuscator_process = None
        self._close_log(self._xray_log_handle)
        self._close_log(self._obfuscator_log_handle)
        self._xray_log_handle = None
        self._obfuscator_log_handle = None
        return self.status("Runtime stopped")

    def status(self, detail: str | None = None) -> RuntimeStatus:
        detail_text = detail or ("Runtime running" if self._is_running() else "Runtime stopped")
        return RuntimeStatus(
            running=self._is_running(),
            xray_binary=self._layout.xray_binary,
            obfuscator_binary=self._layout.obfuscator_binary,
            xray_log=self._layout.xray_log,
            obfuscator_log=self._layout.obfuscator_log,
            detail=detail_text,
        )

    def _spawn(
        self,
        binary: Path,
        args: list[str],
        log_path: Path,
        extra_env: dict[str, str],
    ) -> subprocess.Popen[str]:
        creationflags = getattr(subprocess, "CREATE_NO_WINDOW", 0)
        log_path.parent.mkdir(parents=True, exist_ok=True)
        log_file = log_path.open("a", encoding="utf-8")
        process = subprocess.Popen(
            [str(binary), *args],
            stdout=log_file,
            stderr=subprocess.STDOUT,
            stdin=subprocess.DEVNULL,
            cwd=str(binary.parent),
            text=True,
            creationflags=creationflags,
            env={**os.environ, **extra_env},
        )
        if "xray" in binary.name.lower():
            self._xray_log_handle = log_file
        else:
            self._obfuscator_log_handle = log_file
        return process

    def _ensure_binary(self, binary: Path) -> None:
        if not binary.exists():
            raise FileNotFoundError(
                f"Embedded binary not found: {binary}. "
                f"Place xray.exe and obfuscator.exe under {self._layout.root / 'bin'}."
            )

    def _is_running(self) -> bool:
        return any(
            process is not None and process.poll() is None
            for process in (self._xray_process, self._obfuscator_process)
        )

    def _failed_runtime_process(self) -> tuple[str, Path] | None:
        candidates = [
            ("Xray", self._xray_process, self._layout.xray_log),
            ("Obfuscator", self._obfuscator_process, self._layout.obfuscator_log),
        ]
        for name, process, log_path in candidates:
            if process is not None and process.poll() is not None:
                return name, log_path
        return None

    def _stop_process(self, process: subprocess.Popen[str] | None) -> None:
        if process is None or process.poll() is not None:
            return

        process.terminate()
        try:
            process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait(timeout=2)

    def _shutdown_quietly(self) -> None:
        try:
            self.stop()
        except Exception:
            pass

    def _close_log(self, handle: TextIOWrapper | None) -> None:
        if handle is None or handle.closed:
            return
        handle.close()

    def _read_log_excerpt(self, log_path: Path, max_lines: int = 4) -> str:
        if not log_path.exists():
            return ""
        lines = log_path.read_text(encoding="utf-8", errors="replace").splitlines()
        excerpt = " | ".join(line.strip() for line in lines[-max_lines:] if line.strip())
        return excerpt[:500]

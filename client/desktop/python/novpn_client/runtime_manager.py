from __future__ import annotations

import atexit
import logging
import os
import subprocess
import time
from io import TextIOWrapper
from pathlib import Path

from .config_builder import XrayConfigBuilder
from .models import (
    ClientProfile,
    ConnectionMode,
    DesktopSettings,
    LocalPorts,
    RuntimeStatus,
    with_runtime_local_ports,
)
from .obfuscator_config_builder import ObfuscatorConfigBuilder
from .runtime_local_ports import RuntimeLocalPortResolver
from .runtime_layout import RuntimeLayout
from .session_obfuscation import SessionObfuscationPlanner
from .windows_tunnel import WindowsSystemTunnelManager


class DesktopRuntimeManager:
    def __init__(
        self,
        runtime_root: Path,
        generated_root: Path | None = None,
        xray_binary: Path | None = None,
        obfuscator_binary: Path | None = None,
        port_resolver: RuntimeLocalPortResolver | None = None,
        logger: logging.Logger | None = None,
    ) -> None:
        self._layout = RuntimeLayout.detect(runtime_root, generated_root, xray_binary, obfuscator_binary)
        self._xray_builder = XrayConfigBuilder()
        self._obfuscator_builder = ObfuscatorConfigBuilder()
        self._xray_process: subprocess.Popen[str] | None = None
        self._obfuscator_process: subprocess.Popen[str] | None = None
        self._xray_log_handle: TextIOWrapper | None = None
        self._obfuscator_log_handle: TextIOWrapper | None = None
        self._logger = logger or logging.getLogger("novpn.desktop.runtime")
        self._windows_tunnel = WindowsSystemTunnelManager(self._logger.getChild("system_tunnel"))
        self._port_resolver = port_resolver or RuntimeLocalPortResolver()
        self._active_connection_mode = ConnectionMode.LOCAL_PROXY
        self._active_local_ports = LocalPorts(socks_port=0, http_port=0)
        self._status_warnings: list[str] = []
        atexit.register(self._shutdown_quietly)

    @property
    def layout(self) -> RuntimeLayout:
        return self._layout

    def start(self, profile: ClientProfile, settings: DesktopSettings) -> RuntimeStatus:
        if self._is_running():
            self._logger.info("runtime start skipped because it is already running")
            return self.status("Runtime already running.")

        self._layout.ensure_directories()
        resolved_ports = self._port_resolver.resolve(profile.local)
        effective_profile = with_runtime_local_ports(profile, resolved_ports.local_ports)
        upstream_interface_name = ""
        if settings.connection_mode == ConnectionMode.SYSTEM_TUNNEL:
            self._windows_tunnel.ensure_supported(self._layout)
            tunnel_plan = self._windows_tunnel.build_plan(effective_profile)
            upstream_interface_name = tunnel_plan.upstream.interface_alias
        else:
            tunnel_plan = None
        self._logger.info(
            (
                "starting runtime profile=%s address=%s:%s generated_root=%s "
                "requested_mode=%s effective_socks=%s:%s effective_http=%s:%s"
            ),
            effective_profile.name,
            effective_profile.server.address,
            effective_profile.server.port,
            self._layout.generated_root,
            settings.connection_mode.value,
            effective_profile.local.socks_listen,
            effective_profile.local.socks_port,
            effective_profile.local.http_listen,
            effective_profile.local.http_port,
        )
        for warning in resolved_ports.warnings:
            self._logger.warning("runtime port resolver: %s", warning)
        runtime_settings = DesktopSettings(
            bypass_ru=settings.bypass_ru,
            app_routing_mode=settings.app_routing_mode,
            selected_apps=list(settings.selected_apps),
            traffic_strategy=settings.traffic_strategy,
            pattern_strategy=settings.pattern_strategy,
            connection_mode=settings.connection_mode,
            device_id=settings.device_id,
            output_path=self._layout.xray_config,
            network_interface_name=upstream_interface_name,
            network_interface_ipv4=settings.network_interface_ipv4,
        )
        session_plan = SessionObfuscationPlanner.build(
            profile=effective_profile,
            device_id=settings.device_id or "desktop-runtime",
        )
        self._xray_builder.write(effective_profile, runtime_settings, session_plan)
        self._obfuscator_builder.write(
            effective_profile,
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
            log_excerpt = self._read_log_excerpt(log_path)
            self.stop()
            if log_excerpt:
                self._logger.error("runtime start failed %s: %s", failed_runtime_name, log_excerpt)
            detail = f"{failed_runtime_name} exited during startup. Check {log_path}."
            raise RuntimeError(detail)

        self._active_connection_mode = settings.connection_mode
        self._active_local_ports = effective_profile.local
        self._status_warnings = list(resolved_ports.warnings)
        if tunnel_plan is not None:
            try:
                self._windows_tunnel.activate(tunnel_plan)
            except Exception:
                self._logger.exception("system tunnel activation failed")
                self.stop()
                raise
        self._logger.info(
            "runtime started xray_log=%s obfuscator_log=%s",
            self._layout.xray_log,
            self._layout.obfuscator_log,
        )
        return self.status(self._build_running_detail())

    def stop(self) -> RuntimeStatus:
        self._logger.info("stopping runtime")
        if self._active_connection_mode == ConnectionMode.SYSTEM_TUNNEL:
            try:
                self._windows_tunnel.deactivate()
            except Exception:
                self._logger.exception("system tunnel teardown failed")
        self._stop_process(self._xray_process)
        self._stop_process(self._obfuscator_process)
        self._xray_process = None
        self._obfuscator_process = None
        self._active_connection_mode = ConnectionMode.LOCAL_PROXY
        self._active_local_ports = LocalPorts(socks_port=0, http_port=0)
        self._status_warnings = []
        self._close_log(self._xray_log_handle)
        self._close_log(self._obfuscator_log_handle)
        self._xray_log_handle = None
        self._obfuscator_log_handle = None
        return self.status("Runtime stopped.")

    def status(self, detail: str | None = None) -> RuntimeStatus:
        if detail is not None:
            detail_text = detail
        elif self._is_running() and self._active_connection_mode == ConnectionMode.SYSTEM_TUNNEL:
            detail_text = self._build_running_detail()
        elif self._is_running():
            detail_text = self._build_running_detail()
        else:
            detail_text = "Runtime stopped."
        return RuntimeStatus(
            running=self._is_running(),
            xray_binary=self._layout.xray_binary,
            obfuscator_binary=self._layout.obfuscator_binary,
            xray_log=self._layout.xray_log,
            obfuscator_log=self._layout.obfuscator_log,
            detail=detail_text,
            effective_connection_mode=self._active_connection_mode,
            socks_listen=self._active_local_ports.socks_listen,
            socks_port=self._active_local_ports.socks_port,
            http_listen=self._active_local_ports.http_listen,
            http_port=self._active_local_ports.http_port,
            warnings=list(self._status_warnings),
        )

    def _build_running_detail(self) -> str:
        mode = (
            "system_tunnel"
            if self._active_connection_mode == ConnectionMode.SYSTEM_TUNNEL
            else "local_proxy"
        )
        return (
            f"Runtime running in {mode} mode. "
            f"SOCKS {self._active_local_ports.socks_listen}:{self._active_local_ports.socks_port}. "
            f"HTTP {self._active_local_ports.http_listen}:{self._active_local_ports.http_port}."
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
        log_file = log_path.open("a", encoding="utf-8", buffering=1)
        log_file.write(f"\n=== {time.strftime('%Y-%m-%d %H:%M:%S')} starting {binary.name} ===\n")
        self._logger.info("spawning runtime process binary=%s args=%s log=%s", binary, args, log_path)
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

        self._logger.info("terminating runtime process pid=%s", process.pid)
        process.terminate()
        try:
            process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            self._logger.warning("forcing runtime process shutdown pid=%s", process.pid)
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
        handle.flush()
        handle.close()

    def _read_log_excerpt(self, log_path: Path, max_lines: int = 4) -> str:
        if not log_path.exists():
            return ""
        lines = log_path.read_text(encoding="utf-8", errors="replace").splitlines()
        excerpt = " | ".join(line.strip() for line in lines[-max_lines:] if line.strip())
        return excerpt[:500]

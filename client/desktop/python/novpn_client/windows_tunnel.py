from __future__ import annotations

import json
import logging
import os
import platform
import socket
import subprocess
import time
from dataclasses import dataclass

from .models import ClientProfile
from .runtime_layout import RuntimeLayout


@dataclass(slots=True)
class DefaultRoute:
    interface_alias: str
    interface_index: int
    next_hop: str
    route_metric: int
    interface_metric: int


@dataclass(slots=True)
class TunnelAdapter:
    name: str
    interface_index: int
    ip_address: str


@dataclass(slots=True)
class TunnelPlan:
    server_ip: str
    upstream: DefaultRoute
    tunnel_name: str
    tunnel_interface_index: int = 0
    tunnel_ip: str = ""


class WindowsSystemTunnelManager:
    TUNNEL_NAME = "NoVPN Tunnel"

    def __init__(self, logger: logging.Logger | None = None) -> None:
        self._logger = logger or logging.getLogger("novpn.desktop.windows_tunnel")
        self._active_plan: TunnelPlan | None = None

    @staticmethod
    def is_windows() -> bool:
        return platform.system() == "Windows"

    @staticmethod
    def is_admin() -> bool:
        if not WindowsSystemTunnelManager.is_windows():
            return False
        try:
            import ctypes

            return bool(ctypes.windll.shell32.IsUserAnAdmin())
        except Exception:
            return False

    def ensure_supported(self, layout: RuntimeLayout) -> None:
        if not self.is_windows():
            raise RuntimeError("System tunnel mode is currently supported only on Windows.")
        if not self.is_admin():
            raise RuntimeError("System tunnel mode requires starting NoVPN Desktop as Administrator.")
        if not layout.wintun_dll.is_file():
            raise RuntimeError(
                f"wintun.dll was not found next to xray.exe: {layout.wintun_dll}"
            )

    def build_plan(self, profile: ClientProfile) -> TunnelPlan:
        upstream = self._get_default_route()
        server_ip = self._resolve_server_ip(profile.server.address)
        plan = TunnelPlan(
            server_ip=server_ip,
            upstream=upstream,
            tunnel_name=self.TUNNEL_NAME,
        )
        self._logger.info(
            "prepared system tunnel plan server_ip=%s upstream_interface=%s upstream_gateway=%s",
            plan.server_ip,
            plan.upstream.interface_alias,
            plan.upstream.next_hop,
        )
        return plan

    def activate(self, plan: TunnelPlan) -> TunnelPlan:
        adapter = self._wait_for_adapter(plan.tunnel_name)
        plan.tunnel_interface_index = adapter.interface_index
        plan.tunnel_ip = adapter.ip_address
        self._active_plan = plan
        try:
            self._delete_route(
                "0.0.0.0",
                "0.0.0.0",
                gateway=plan.tunnel_ip,
                interface_index=plan.tunnel_interface_index,
            )
            self._delete_route(
                plan.server_ip,
                "255.255.255.255",
                gateway=plan.upstream.next_hop,
                interface_index=plan.upstream.interface_index,
            )
            self._add_route(
                plan.server_ip,
                "255.255.255.255",
                gateway=plan.upstream.next_hop,
                interface_index=plan.upstream.interface_index,
                metric=1,
            )
            self._add_route(
                "0.0.0.0",
                "0.0.0.0",
                gateway=plan.tunnel_ip,
                interface_index=plan.tunnel_interface_index,
                metric=1,
            )
        except Exception:
            self.deactivate()
            raise
        self._logger.info(
            "system tunnel activated tunnel_name=%s tunnel_ip=%s tunnel_if=%s",
            plan.tunnel_name,
            plan.tunnel_ip,
            plan.tunnel_interface_index,
        )
        return plan

    def deactivate(self) -> None:
        if self._active_plan is None:
            return
        plan = self._active_plan
        self._logger.info(
            "deactivating system tunnel tunnel_name=%s tunnel_ip=%s",
            plan.tunnel_name,
            plan.tunnel_ip,
        )
        self._delete_route(
            "0.0.0.0",
            "0.0.0.0",
            gateway=plan.tunnel_ip,
            interface_index=plan.tunnel_interface_index,
        )
        self._delete_route(
            plan.server_ip,
            "255.255.255.255",
            gateway=plan.upstream.next_hop,
            interface_index=plan.upstream.interface_index,
        )
        self._active_plan = None

    def _get_default_route(self) -> DefaultRoute:
        script = """
$route = Get-NetRoute -AddressFamily IPv4 -DestinationPrefix '0.0.0.0/0' `
  | Where-Object { $_.State -eq 'Alive' -and $_.NextHop -and $_.NextHop -ne '0.0.0.0' } `
  | Sort-Object RouteMetric, InterfaceMetric `
  | Select-Object -First 1 InterfaceAlias, InterfaceIndex, NextHop, RouteMetric, InterfaceMetric
if ($null -eq $route) { exit 3 }
$route | ConvertTo-Json -Compress
"""
        payload = self._run_powershell_json(script)
        return DefaultRoute(
            interface_alias=str(payload["InterfaceAlias"]).strip(),
            interface_index=int(payload["InterfaceIndex"]),
            next_hop=str(payload["NextHop"]).strip(),
            route_metric=int(payload.get("RouteMetric", 0) or 0),
            interface_metric=int(payload.get("InterfaceMetric", 0) or 0),
        )

    def _wait_for_adapter(self, tunnel_name: str, timeout_seconds: float = 12.0) -> TunnelAdapter:
        deadline = time.time() + timeout_seconds
        while time.time() < deadline:
            adapter = self._read_adapter(tunnel_name)
            if adapter is not None:
                return adapter
            time.sleep(0.5)
        raise RuntimeError(
            "The Windows tunnel adapter did not appear in time. "
            "Run the desktop client as Administrator and verify that xray.exe supports TUN mode."
        )

    def _read_adapter(self, tunnel_name: str) -> TunnelAdapter | None:
        script = f"""
$adapter = Get-NetAdapter -Name '{tunnel_name}' -ErrorAction SilentlyContinue | Select-Object -First 1 Name, InterfaceIndex
if ($null -eq $adapter) {{ exit 3 }}
$ip = Get-NetIPAddress -InterfaceIndex $adapter.InterfaceIndex -AddressFamily IPv4 -ErrorAction SilentlyContinue `
  | Where-Object {{ $_.IPAddress -and $_.IPAddress -ne '0.0.0.0' }} `
  | Select-Object -First 1 IPAddress
if ($null -eq $ip) {{ exit 4 }}
[pscustomobject]@{{
  Name = $adapter.Name
  InterfaceIndex = $adapter.InterfaceIndex
  IPAddress = $ip.IPAddress
}} | ConvertTo-Json -Compress
"""
        try:
            payload = self._run_powershell_json(script, allow_exit_codes={3, 4})
        except RuntimeError:
            return None
        if payload is None:
            return None
        return TunnelAdapter(
            name=str(payload["Name"]).strip(),
            interface_index=int(payload["InterfaceIndex"]),
            ip_address=str(payload["IPAddress"]).strip(),
        )

    def _resolve_server_ip(self, address: str) -> str:
        candidate = address.strip()
        if not candidate:
            raise RuntimeError("Server address is empty, system tunnel cannot resolve upstream route.")
        try:
            socket.inet_aton(candidate)
            return candidate
        except OSError:
            pass

        infos = socket.getaddrinfo(candidate, None, socket.AF_INET, socket.SOCK_STREAM)
        for info in infos:
            ip_address = str(info[4][0]).strip()
            if ip_address:
                return ip_address
        raise RuntimeError(f"Unable to resolve an IPv4 address for the upstream server: {candidate}")

    def _add_route(
        self,
        destination: str,
        mask: str,
        gateway: str,
        interface_index: int,
        metric: int,
    ) -> None:
        self._run_command(
            [
                "route",
                "add",
                destination,
                "mask",
                mask,
                gateway,
                "if",
                str(interface_index),
                "metric",
                str(metric),
            ]
        )

    def _delete_route(
        self,
        destination: str,
        mask: str,
        gateway: str,
        interface_index: int,
    ) -> None:
        self._run_command(
            [
                "route",
                "delete",
                destination,
                "mask",
                mask,
                gateway,
                "if",
                str(interface_index),
            ],
            check=False,
        )

    def _run_command(self, args: list[str], check: bool = True) -> subprocess.CompletedProcess[str]:
        creationflags = getattr(subprocess, "CREATE_NO_WINDOW", 0)
        process = subprocess.run(
            args,
            capture_output=True,
            text=True,
            encoding="utf-8",
            errors="replace",
            creationflags=creationflags,
            env=os.environ.copy(),
        )
        if check and process.returncode != 0:
            message = (process.stderr or process.stdout).strip() or f"exit code {process.returncode}"
            raise RuntimeError(f"Command failed: {' '.join(args)} :: {message}")
        return process

    def _run_powershell_json(
        self,
        script: str,
        allow_exit_codes: set[int] | None = None,
    ) -> dict[str, object] | None:
        creationflags = getattr(subprocess, "CREATE_NO_WINDOW", 0)
        process = subprocess.run(
            [
                "powershell",
                "-NoProfile",
                "-ExecutionPolicy",
                "Bypass",
                "-Command",
                script,
            ],
            capture_output=True,
            text=True,
            encoding="utf-8",
            errors="replace",
            creationflags=creationflags,
            env=os.environ.copy(),
        )
        allowed = allow_exit_codes or set()
        if process.returncode in allowed:
            return None
        if process.returncode != 0:
            message = (process.stderr or process.stdout).strip() or f"exit code {process.returncode}"
            raise RuntimeError(f"PowerShell command failed: {message}")
        raw = process.stdout.strip()
        if not raw:
            return None
        payload = json.loads(raw)
        if not isinstance(payload, dict):
            raise RuntimeError("PowerShell JSON output was not an object.")
        return payload
